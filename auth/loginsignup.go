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
	"naevis/middleware"
	"naevis/mq"
	"naevis/rdx"
	"naevis/structs"
	"naevis/utils"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

const (
	refreshTokenTTL = 7 * 24 * time.Hour // 7 days
	accessTokenTTL  = 15 * time.Minute   // 15 minutes
)

var (
	// tokenSigningAlgo = jwt.SigningMethodHS256
	jwtSecret = []byte("your_secret_key") // Replace with a secure secret key
)

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var user structs.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	var storedUser structs.User
	err := db.UserCollection.FindOne(context.TODO(), bson.M{"username": user.Username}).Decode(&storedUser)
	if err != nil {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	// // Check if verified
	// if !storedUser.EmailVerified {
	// 	http.Error(w, "User not verified. Please check your email for the OTP.", http.StatusUnauthorized)
	// 	return
	// }

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(storedUser.Password), []byte(user.Password)); err != nil {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	// Generate JWT
	claims := &middleware.Claims{
		Username: storedUser.Username,
		UserID:   storedUser.UserID,
		Role:     storedUser.Role, // assumes Role []string exists in your structs.User
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Generate refresh token
	refreshToken, err := generateRefreshToken()
	if err != nil {
		http.Error(w, "Error generating refresh token", http.StatusInternalServerError)
		return
	}
	hashedRefresh := hashToken(refreshToken)

	_, err = db.UserCollection.UpdateOne(
		context.TODO(),
		bson.M{"userid": storedUser.UserID},
		bson.M{"$set": bson.M{
			"refresh_token":  hashedRefresh,
			"refresh_expiry": time.Now().Add(refreshTokenTTL),
			"last_login":     time.Now(),
		}},
	)
	if err != nil {
		http.Error(w, "Failed to store refresh token", http.StatusInternalServerError)
		return
	}

	// Return tokens
	utils.SendResponse(w, http.StatusOK, map[string]string{
		"token":        tokenString,
		"refreshToken": refreshToken,
		"userid":       storedUser.UserID,
	}, "Login successful", nil)
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	var user structs.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	log.Printf("Registering user: %s", user.Username)

	// Check if user already exists
	var existingUser structs.User
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
		log.Printf("Failed to hash password for user %s: %v", user.Username, err)
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}
	user.Password = string(hashedPassword)
	user.UserID = "u" + utils.GenerateName(10)
	// user.EmailVerified = false
	user.EmailVerified = true
	user.Role = []string{"user"}

	// // Generate OTP and send email
	// otp := generateOTP(6)
	// err = sendEmailOTP(user.Email, otp)
	// if err != nil {
	// 	log.Printf("Failed to send OTP email: %v", err)
	// 	http.Error(w, "Failed to send OTP", http.StatusInternalServerError)
	// 	return
	// }

	// // Store OTP in Redis (expires in 10 minutes)
	// err = rdx.SetWithExpiry("otp:"+user.Email, otp, 10*time.Minute)
	// if err != nil {
	// 	log.Printf("Failed to cache OTP: %v", err)
	// }

	err = rdx.RdxSet(fmt.Sprintf("users:%s", user.UserID), user.Username)
	if err != nil {
		log.Printf("Failed to cache username: %v", err)
	}

	// Save user in DB (unverified)
	_, err = db.UserCollection.InsertOne(context.TODO(), user)
	if err != nil {
		http.Error(w, "Failed to register user", http.StatusInternalServerError)
		return
	}

	// Optional: Emit to MQ, cache user ID, etc.

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"status":  http.StatusCreated,
		"message": "OTP sent to email. Please verify to complete registration.",
	})
}

func logoutUserHandler(w http.ResponseWriter, r *http.Request) {
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		http.Error(w, "Missing token", http.StatusUnauthorized)
		return
	}

	if len(tokenString) < 7 || tokenString[:7] != "Bearer " {
		http.Error(w, "Invalid token format", http.StatusUnauthorized)
		return
	}

	// Extract the token and invalidate it in Redis
	tokenString = tokenString[7:]
	claims := &middleware.Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Remove token from Redis cache
	_, err = rdx.RdxHdel("tokki", claims.UserID)
	if err != nil {
		log.Printf("Error removing token from Redis: %v", err)
		http.Error(w, "Failed to log out", http.StatusInternalServerError)
		return
	}

	m := mq.Index{}
	mq.Emit("user-loggedout", m)

	utils.SendResponse(w, http.StatusOK, nil, "User logged out successfully", nil)
}

func refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		http.Error(w, "Missing token", http.StatusUnauthorized)
		return
	}

	if len(tokenString) < 7 || tokenString[:7] != "Bearer " {
		http.Error(w, "Invalid token format", http.StatusUnauthorized)
		return
	}

	tokenString = tokenString[7:]
	claims := &middleware.Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Ensure the token is not expired and refresh it
	if time.Until(claims.ExpiresAt.Time) < 30*time.Minute {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(72 * time.Hour)) // Extend the expiration
		newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		newTokenString, err := newToken.SignedString(jwtSecret)
		if err != nil {
			http.Error(w, "Failed to refresh token", http.StatusInternalServerError)
			return
		}

		// Update the token in Redis
		err = rdx.RdxHset("tokki", claims.UserID, newTokenString)
		if err != nil {
			log.Printf("Error updating token in Redis: %v", err)
		}

		utils.SendResponse(w, http.StatusOK, map[string]string{"token": newTokenString}, "Token refreshed successfully", nil)
	} else {
		http.Error(w, "Token refresh not allowed yet", http.StatusForbidden)
	}
}

// Generates a random refresh token
func generateRefreshToken() (string, error) {
	tokenBytes := make([]byte, 32)
	_, err := rand.Read(tokenBytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(tokenBytes), nil
}

// Hashes a given token
func hashToken(token string) string {
	hash := sha256.New()
	hash.Write([]byte(token))
	return hex.EncodeToString(hash.Sum(nil))
}
