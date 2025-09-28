package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"naevis/db"
	"naevis/globals"
	"naevis/middleware"
	"naevis/models"
	"naevis/mq"
	"naevis/rdx"
	"naevis/utils"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

const (
	RefreshTokenTTL = 7 * 24 * time.Hour // 7 days
	AccessTokenTTL  = 24 * time.Hour     // 15 minutes
)

// ===== LOGIN =====
func loginHandler(w http.ResponseWriter, r *http.Request) {
	var user models.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	var storedUser models.User
	err := db.UserCollection.FindOne(context.TODO(), bson.M{"username": user.Username}).Decode(&storedUser)
	if err != nil {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(storedUser.Password), []byte(user.Password)); err != nil {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	// Generate access token
	claims := &middleware.Claims{
		Username: storedUser.Username,
		UserID:   storedUser.UserID,
		Role:     storedUser.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	access := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := access.SignedString(globals.JwtSecret)
	if err != nil {
		log.Printf("login: failed to sign access token: %v", err)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Generate refresh token (raw), store only hash in DB
	refreshToken, err := generateRefreshToken()
	if err != nil {
		log.Printf("login: failed to generate refresh token: %v", err)
		http.Error(w, "Error generating refresh token", http.StatusInternalServerError)
		return
	}
	hashedRefresh := hashToken(refreshToken)

	_, err = db.UserCollection.UpdateOne(
		context.TODO(),
		bson.M{"userid": storedUser.UserID},
		bson.M{"$set": bson.M{
			"refresh_token":  hashedRefresh,
			"refresh_expiry": time.Now().Add(RefreshTokenTTL),
			"last_login":     time.Now(),
		}},
	)
	if err != nil {
		log.Printf("login: failed to store refresh token in DB for user %s: %v", storedUser.UserID, err)
		http.Error(w, "Failed to store refresh token", http.StatusInternalServerError)
		return
	}

	// Set refresh token as a secure, HttpOnly cookie. Path is "/" so refresh endpoint receives it.
	// Secure: true for production (HTTPS). If testing locally over HTTP, set Secure false temporarily.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/", // cookie available across your API; restrict if you need more granularity
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(RefreshTokenTTL),
	})

	// Return access token and user id; refresh token is kept in cookie only
	utils.SendResponse(w, http.StatusOK, map[string]string{
		"token":  accessToken,
		"userid": storedUser.UserID,
	}, "Login successful", nil)
}

// ===== REGISTER =====
func registerHandler(w http.ResponseWriter, r *http.Request) {
	var user models.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	log.Printf("Registering user: %s", user.Username)

	// Check if user already exists
	var existingUser models.User
	err := db.UserCollection.FindOne(context.TODO(), bson.M{"username": user.Username}).Decode(&existingUser)
	if err == nil {
		http.Error(w, "User already exists", http.StatusConflict)
		return
	} else if err != mongo.ErrNoDocuments {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash password: %v", err)
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}
	user.Password = string(hashedPassword)
	user.UserID = "u" + utils.GenerateRandomString(10)
	user.EmailVerified = true
	user.Role = []string{"user"}

	err = rdx.RdxSet(fmt.Sprintf("users:%s", user.UserID), user.Username)
	if err != nil {
		log.Printf("Failed to cache username: %v", err)
	}

	_, err = db.UserCollection.InsertOne(context.TODO(), user)
	if err != nil {
		log.Printf("register: failed to insert user %s: %v", user.Username, err)
		http.Error(w, "Failed to register user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"status":  http.StatusCreated,
		"message": "User registered successfully.",
	})
}

// ===== LOGOUT =====
func logoutUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		http.Error(w, "Missing token", http.StatusUnauthorized)
		return
	}

	if len(tokenString) < 7 || tokenString[:7] != "Bearer " {
		http.Error(w, "Invalid token format", http.StatusUnauthorized)
		return
	}

	claims := &middleware.Claims{}
	_, err := jwt.ParseWithClaims(tokenString[7:], claims, func(token *jwt.Token) (any, error) {
		return globals.JwtSecret, nil
	})
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Clear refresh token in DB
	_, err = db.UserCollection.UpdateOne(
		context.TODO(),
		bson.M{"userid": claims.UserID},
		bson.M{"$unset": bson.M{
			"refresh_token":  "",
			"refresh_expiry": "",
		}},
	)
	if err != nil {
		log.Printf("logout: failed to clear refresh token for user %s: %v", claims.UserID, err)
		http.Error(w, "Failed to log out", http.StatusInternalServerError)
		return
	}

	// Remove any cached access token from Redis (optional)
	_, _ = rdx.RdxHdel("tokki", claims.UserID)

	// Clear refresh token cookie client-side by setting an expired cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})

	m := models.Index{}
	mq.Emit(ctx, "user-loggedout", m)

	utils.SendResponse(w, http.StatusOK, nil, "User logged out successfully", nil)
}

// ===== REFRESH TOKEN =====
// This endpoint reads the refresh token from the HttpOnly cookie, verifies it (by comparing hashed value in DB),
// rotates the refresh token and sets the new cookie, and returns a new short-lived access token.
func refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		http.Error(w, "Missing refresh token", http.StatusUnauthorized)
		return
	}
	refreshToken := cookie.Value

	if refreshToken == "" {
		http.Error(w, "Missing refresh token", http.StatusUnauthorized)
		return
	}

	// Find user by hashed refresh token
	hashed := hashToken(refreshToken)
	var storedUser models.User
	err = db.UserCollection.FindOne(
		context.TODO(),
		bson.M{"refresh_token": hashed},
	).Decode(&storedUser)
	if err != nil {
		// Do not reveal whether token missing or DB error
		log.Printf("refresh: refresh token lookup failed: %v", err)
		http.Error(w, "Invalid or expired refresh token", http.StatusUnauthorized)
		return
	}

	// Check expiry
	if time.Now().After(storedUser.RefreshExpiry) {
		// Clear DB stale refresh token
		_, _ = db.UserCollection.UpdateOne(context.TODO(), bson.M{"userid": storedUser.UserID}, bson.M{"$unset": bson.M{
			"refresh_token":  "",
			"refresh_expiry": "",
		}})
		http.Error(w, "Invalid or expired refresh token", http.StatusUnauthorized)
		return
	}

	// New access token
	claims := &middleware.Claims{
		Username: storedUser.Username,
		UserID:   storedUser.UserID,
		Role:     storedUser.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	newAccess := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := newAccess.SignedString(globals.JwtSecret)
	if err != nil {
		log.Printf("refresh: failed to sign new access token for user %s: %v", storedUser.UserID, err)
		http.Error(w, "Failed to generate access token", http.StatusInternalServerError)
		return
	}

	// Rotate refresh token (single-use)
	newRefresh, err := generateRefreshToken()
	if err != nil {
		log.Printf("refresh: failed to generate new refresh token for user %s: %v", storedUser.UserID, err)
		http.Error(w, "Failed to generate refresh token", http.StatusInternalServerError)
		return
	}
	hashedNewRefresh := hashToken(newRefresh)

	_, err = db.UserCollection.UpdateOne(
		context.TODO(),
		bson.M{"userid": storedUser.UserID},
		bson.M{"$set": bson.M{
			"refresh_token":  hashedNewRefresh,
			"refresh_expiry": time.Now().Add(RefreshTokenTTL),
		}},
	)
	if err != nil {
		log.Printf("refresh: failed to update refresh token in DB for user %s: %v", storedUser.UserID, err)
		http.Error(w, "Failed to update refresh token", http.StatusInternalServerError)
		return
	}

	// Set rotated refresh token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    newRefresh,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(RefreshTokenTTL),
	})

	utils.SendResponse(w, http.StatusOK, map[string]string{
		"token":  accessToken,
		"userid": storedUser.UserID,
	}, "Token refreshed successfully", nil)
}

// ===== HELPERS =====
func generateRefreshToken() (string, error) {
	tokenBytes := make([]byte, 32)
	_, err := rand.Read(tokenBytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(tokenBytes), nil
}

func hashToken(token string) string {
	hash := sha256.New()
	hash.Write([]byte(token))
	return hex.EncodeToString(hash.Sum(nil))
}
