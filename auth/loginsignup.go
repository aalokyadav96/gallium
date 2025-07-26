// package auth

// import (
// 	"context"
// 	"crypto/rand"
// 	"crypto/sha256"
// 	"encoding/hex"
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"log"
// 	"net/http"
// 	"os"
// 	"strings"
// 	"time"

// 	"naevis/db"
// 	"naevis/middleware"
// 	"naevis/mq"
// 	"naevis/rdx"
// 	"naevis/structs"
// 	"naevis/utils"

// 	"github.com/golang-jwt/jwt/v5"
// 	"go.mongodb.org/mongo-driver/bson"
// 	"go.mongodb.org/mongo-driver/mongo"
// 	"golang.org/x/crypto/bcrypt"
// )

// const (
// 	refreshTokenTTL = 7 * 24 * time.Hour
// 	accessTokenTTL  = 15 * time.Minute
// )

// var (
// 	jwtSecret = []byte(os.Getenv("JWT_SECRET")) // Set securely via env
// )

// func loginHandler(w http.ResponseWriter, r *http.Request) {
// 	var input struct {
// 		Username string `json:"username"`
// 		Password string `json:"password"`
// 	}

// 	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
// 		sendError(w, http.StatusBadRequest, "Invalid input")
// 		return
// 	}

// 	if input.Username == "" || input.Password == "" {
// 		sendError(w, http.StatusBadRequest, "Username and password are required")
// 		return
// 	}
// 	log.Println(input.Username)
// 	var storedUser structs.User
// 	err := db.UserCollection.FindOne(context.TODO(), bson.M{"username": input.Username}).Decode(&storedUser)
// 	if err != nil {
// 		sendError(w, http.StatusUnauthorized, "Invalid username or password")
// 		return
// 	}

// 	// Compare with PasswordHash instead of Password field
// 	if err := bcrypt.CompareHashAndPassword([]byte(storedUser.Password), []byte(input.Password)); err != nil {
// 		sendError(w, http.StatusUnauthorized, "Invalid username or password")
// 		return
// 	}

// 	tokenString, err := generateAccessToken(storedUser)
// 	if err != nil {
// 		sendError(w, http.StatusInternalServerError, "Failed to generate token")
// 		return
// 	}

// 	refreshToken, err := generateRefreshToken()
// 	if err != nil {
// 		sendError(w, http.StatusInternalServerError, "Failed to generate refresh token")
// 		return
// 	}

// 	hashedRefresh := hashToken(refreshToken)

// 	_, err = db.UserCollection.UpdateOne(
// 		context.TODO(),
// 		bson.M{"userid": storedUser.UserID},
// 		bson.M{
// 			"$set": bson.M{
// 				"refresh_token":  hashedRefresh,
// 				"refresh_expiry": time.Now().Add(refreshTokenTTL),
// 				"last_login":     time.Now(),
// 			},
// 		},
// 	)
// 	if err != nil {
// 		sendError(w, http.StatusInternalServerError, "Failed to store refresh token")
// 		return
// 	}

// 	if err := rdx.RdxHset("tokki", storedUser.UserID, tokenString); err != nil {
// 		log.Printf("Redis token storage failed: %v", err)
// 	}

// 	utils.SendResponse(w, http.StatusOK, map[string]string{
// 		"token":        tokenString,
// 		"refreshToken": refreshToken,
// 		"userid":       storedUser.UserID,
// 	}, "Login successful", nil)
// }

// // func loginHandler(w http.ResponseWriter, r *http.Request) {
// // 	var input struct {
// // 		Username string `json:"username"`
// // 		Password string `json:"password"`
// // 	}

// // 	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
// // 		sendError(w, http.StatusBadRequest, "Invalid input")
// // 		return
// // 	}

// // 	if input.Username == "" || input.Password == "" {
// // 		sendError(w, http.StatusBadRequest, "Username and password are required")
// // 		return
// // 	}

// // 	var storedUser structs.User
// // 	err := db.UserCollection.FindOne(context.TODO(), bson.M{"username": input.Username}).Decode(&storedUser)
// // 	if err != nil {
// // 		sendError(w, http.StatusUnauthorized, "Invalid username or password")
// // 		return
// // 	}

// // 	if err := bcrypt.CompareHashAndPassword([]byte(storedUser.Password), []byte(input.Password)); err != nil {
// // 		sendError(w, http.StatusUnauthorized, "Invalid username or password")
// // 		return
// // 	}

// // 	tokenString, err := generateAccessToken(storedUser)
// // 	if err != nil {
// // 		sendError(w, http.StatusInternalServerError, "Failed to generate token")
// // 		return
// // 	}

// // 	refreshToken, err := generateRefreshToken()
// // 	if err != nil {
// // 		sendError(w, http.StatusInternalServerError, "Failed to generate refresh token")
// // 		return
// // 	}

// // 	hashedRefresh := hashToken(refreshToken)
// // 	_, err = db.UserCollection.UpdateOne(
// // 		context.TODO(),
// // 		bson.M{"userid": storedUser.UserID},
// // 		bson.M{
// // 			"$set": bson.M{
// // 				"refresh_token":  hashedRefresh,
// // 				"refresh_expiry": time.Now().Add(refreshTokenTTL),
// // 				"last_login":     time.Now(),
// // 			},
// // 		},
// // 	)
// // 	if err != nil {
// // 		sendError(w, http.StatusInternalServerError, "Failed to store refresh token")
// // 		return
// // 	}

// // 	rdx.RdxHset("tokki", storedUser.UserID, tokenString)

// // 	utils.SendResponse(w, http.StatusOK, map[string]string{
// // 		"token":        tokenString,
// // 		"refreshToken": refreshToken,
// // 		"userid":       storedUser.UserID,
// // 	}, "Login successful", nil)
// // }

// func registerHandler(w http.ResponseWriter, r *http.Request) {
// 	type registrationInput struct {
// 		Username string `json:"username"`
// 		Email    string `json:"email"`
// 		Password string `json:"password"`
// 	}

// 	var input registrationInput
// 	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
// 		sendError(w, http.StatusBadRequest, "Invalid input")
// 		return
// 	}

// 	if input.Username == "" || input.Password == "" || input.Email == "" {
// 		sendError(w, http.StatusBadRequest, "Missing required fields")
// 		return
// 	}

// 	// Check if user already exists
// 	err := db.UserCollection.FindOne(context.TODO(), bson.M{"username": input.Username}).Err()
// 	if err == nil {
// 		sendError(w, http.StatusConflict, "User already exists")
// 		return
// 	} else if err != mongo.ErrNoDocuments {
// 		sendError(w, http.StatusInternalServerError, "Database error")
// 		return
// 	}

// 	// Hash password
// 	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
// 	if err != nil {
// 		log.Printf("Hash error: %v", err)
// 		sendError(w, http.StatusInternalServerError, "Could not process password")
// 		return
// 	}

// 	user := structs.User{
// 		UserID:        "u" + utils.GenerateName(10),
// 		Username:      input.Username,
// 		Email:         input.Email,
// 		PasswordHash:  string(hashedPassword),
// 		EmailVerified: true,
// 		IsVerified:    false,
// 		Role:          []string{"user"},
// 		CreatedAt:     time.Now(),
// 		UpdatedAt:     time.Now(),
// 		LastLogin:     time.Time{},
// 		Online:        false,
// 	}

// 	if err := rdx.RdxSet(fmt.Sprintf("users:%s", user.UserID), user.Username); err != nil {
// 		log.Printf("Redis cache failed: %v", err)
// 	}

// 	if _, err := db.UserCollection.InsertOne(context.TODO(), user); err != nil {
// 		sendError(w, http.StatusInternalServerError, "Failed to register user")
// 		return
// 	}

// 	utils.SendResponse(w, http.StatusCreated, nil, "Registration successful", nil)
// }

// // func registerHandler(w http.ResponseWriter, r *http.Request) {
// // 	var user structs.User
// // 	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
// // 		sendError(w, http.StatusBadRequest, "Invalid input")
// // 		return
// // 	}

// // 	if user.Username == "" || user.Password == "" || user.Email == "" {
// // 		sendError(w, http.StatusBadRequest, "Missing required fields")
// // 		return
// // 	}

// // 	err := db.UserCollection.FindOne(context.TODO(), bson.M{"username": user.Username}).Decode(&structs.User{})
// // 	if err == nil {
// // 		sendError(w, http.StatusConflict, "User already exists")
// // 		return
// // 	} else if err != mongo.ErrNoDocuments {
// // 		sendError(w, http.StatusInternalServerError, "Database error")
// // 		return
// // 	}

// // 	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
// // 	if err != nil {
// // 		log.Printf("Hash error: %v", err)
// // 		sendError(w, http.StatusInternalServerError, "Could not process password")
// // 		return
// // 	}

// // 	user.PasswordHash = string(hashedPassword)
// // 	// user.Password = "" // optional: explicitly clear plaintext password

// // 	user.UserID = "u" + utils.GenerateName(10)
// // 	user.EmailVerified = true
// // 	user.Role = []string{"user"}
// // 	user.CreatedAt = time.Now()
// // 	user.UpdatedAt = time.Now()

// // 	if err := rdx.RdxSet(fmt.Sprintf("users:%s", user.UserID), user.Username); err != nil {
// // 		log.Printf("Redis cache failed: %v", err)
// // 	}

// // 	if _, err := db.UserCollection.InsertOne(context.TODO(), user); err != nil {
// // 		sendError(w, http.StatusInternalServerError, "Failed to register user")
// // 		return
// // 	}

// // 	utils.SendResponse(w, http.StatusCreated, nil, "Registration successful", nil)
// // }

// func logoutUserHandler(w http.ResponseWriter, r *http.Request) {
// 	tokenString := getBearerToken(r)
// 	if tokenString == "" {
// 		sendError(w, http.StatusUnauthorized, "Missing token")
// 		return
// 	}

// 	claims := &middleware.Claims{}
// 	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
// 		return jwtSecret, nil
// 	})
// 	if err != nil || !token.Valid {
// 		sendError(w, http.StatusUnauthorized, "Invalid token")
// 		return
// 	}

// 	if _, err := rdx.RdxHdel("tokki", claims.UserID); err != nil {
// 		log.Printf("Redis token remove failed: %v", err)
// 		sendError(w, http.StatusInternalServerError, "Logout failed")
// 		return
// 	}

// 	mq.Emit("user-loggedout", mq.Index{})
// 	utils.SendResponse(w, http.StatusOK, nil, "User logged out", nil)
// }

// func refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
// 	tokenString := getBearerToken(r)
// 	if tokenString == "" {
// 		sendError(w, http.StatusUnauthorized, "Missing token")
// 		return
// 	}

// 	claims := &middleware.Claims{}
// 	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
// 		return jwtSecret, nil
// 	})
// 	if err != nil || !token.Valid {
// 		sendError(w, http.StatusUnauthorized, "Invalid token")
// 		return
// 	}

// 	if time.Until(claims.ExpiresAt.Time) > 30*time.Minute {
// 		sendError(w, http.StatusForbidden, "Token refresh not allowed yet")
// 		return
// 	}

// 	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(72 * time.Hour))
// 	newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
// 	newTokenString, err := newToken.SignedString(jwtSecret)
// 	if err != nil {
// 		sendError(w, http.StatusInternalServerError, "Failed to refresh token")
// 		return
// 	}

// 	_ = rdx.RdxHset("tokki", claims.UserID, newTokenString)

// 	utils.SendResponse(w, http.StatusOK, map[string]string{"token": newTokenString}, "Token refreshed", nil)
// }

// func generateAccessToken(user structs.User) (string, error) {
// 	claims := &middleware.Claims{
// 		Username: user.Username,
// 		UserID:   user.UserID,
// 		Role:     user.Role,
// 		RegisteredClaims: jwt.RegisteredClaims{
// 			ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTokenTTL)),
// 			IssuedAt:  jwt.NewNumericDate(time.Now()),
// 		},
// 	}
// 	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
// 	return token.SignedString(jwtSecret)
// }

// func generateRefreshToken() (string, error) {
// 	buf := make([]byte, 32)
// 	_, err := rand.Read(buf)
// 	if err != nil {
// 		return "", err
// 	}
// 	return hex.EncodeToString(buf), nil
// }

// func hashToken(token string) string {
// 	sum := sha256.Sum256([]byte(token))
// 	return hex.EncodeToString(sum[:])
// }

// func getBearerToken(r *http.Request) string {
// 	authHeader := r.Header.Get("Authorization")
// 	if !strings.HasPrefix(authHeader, "Bearer ") {
// 		return ""
// 	}
// 	return strings.TrimPrefix(authHeader, "Bearer ")
// }

// func sendError(w http.ResponseWriter, code int, msg string) {
// 	utils.SendResponse(w, code, nil, msg, errors.New(msg))
// }

// // package auth

// // import (
// // 	"context"
// // 	"crypto/rand"
// // 	"crypto/sha256"
// // 	"encoding/hex"
// // 	"encoding/json"
// // 	"errors"
// // 	"fmt"
// // 	"log"
// // 	"net/http"
// // 	"os"
// // 	"strconv"
// // 	"strings"
// // 	"time"

// // 	"naevis/db"
// // 	"naevis/middleware"
// // 	"naevis/mq"
// // 	"naevis/rdx"
// // 	"naevis/structs"
// // 	"naevis/utils"

// // 	"github.com/golang-jwt/jwt/v5"
// // 	"github.com/google/uuid"
// // 	"go.mongodb.org/mongo-driver/bson"
// // 	"go.mongodb.org/mongo-driver/mongo"
// // 	"golang.org/x/crypto/bcrypt"
// // )

// // var (
// // 	// Load secrets and config from environment
// // 	jwtSecret               = []byte(os.Getenv("JWT_SECRET"))                    // must be set
// // 	accessTokenTTL, _       = time.ParseDuration(os.Getenv("ACCESS_TOKEN_TTL"))  // e.g. "15m"
// // 	refreshTokenTTL, _      = time.ParseDuration(os.Getenv("REFRESH_TOKEN_TTL")) // e.g. "168h"
// // 	bcryptCost, _           = strconv.Atoi(os.Getenv("BCRYPT_COST"))             // e.g. "12"
// // 	enableEmailVerification = os.Getenv("ENABLE_EMAIL_VERIFICATION") == "true"   // "true" or "false"
// // )

// // func init() {
// // 	if len(jwtSecret) == 0 {
// // 		log.Fatal("JWT_SECRET must be set")
// // 	}
// // 	if bcryptCost < bcrypt.MinCost || bcryptCost > bcrypt.MaxCost {
// // 		bcryptCost = bcrypt.DefaultCost
// // 	}
// // }

// // // SERVICE LAYER (for testability)

// // // hashPassword hashes a plaintext password.
// // func hashPassword(password string) (string, error) {
// // 	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
// // 	return string(hashed), err
// // }

// // // verifyPassword compares a bcrypt-hashed password with its possible plaintext equivalent.
// // func verifyPassword(hashed, plain string) error {
// // 	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain))
// // }

// // // generateJTI returns a unique JWT ID.
// // func generateJTI() string {
// // 	return uuid.NewString()
// // }

// // // generateTokens produces an access and refresh token pair, stores the refresh in Redis.
// // func generateTokens(user structs.User) (accessToken, refreshToken string, err error) {
// // 	// Create claims with jti for replay protection
// // 	jti := generateJTI()
// // 	now := time.Now()

// // 	claims := middleware.Claims{
// // 		Username: user.Username,
// // 		UserID:   user.UserID,
// // 		Role:     user.Role,
// // 		RegisteredClaims: jwt.RegisteredClaims{
// // 			ID:        jti,
// // 			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenTTL)),
// // 			IssuedAt:  jwt.NewNumericDate(now),
// // 		},
// // 	}

// // 	at := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
// // 	accessToken, err = at.SignedString(jwtSecret)
// // 	if err != nil {
// // 		return "", "", err
// // 	}

// // 	// Generate secure refresh token
// // 	rtRaw := make([]byte, 32)
// // 	if _, err = rand.Read(rtRaw); err != nil {
// // 		return "", "", err
// // 	}
// // 	refreshToken = hex.EncodeToString(rtRaw)
// // 	hashedRT := sha256.Sum256([]byte(refreshToken))

// // 	// Store hashed refresh token in Redis with TTL, keyed by userID:jti
// // 	rtKey := fmt.Sprintf("refresh:%s:%s", user.UserID, jti)
// // 	if err = rdx.SetWithExpiry(rtKey, hex.EncodeToString(hashedRT[:]), refreshTokenTTL); err != nil {
// // 		return "", "", err
// // 	}

// // 	return accessToken, refreshToken, nil
// // }

// // // validateRefreshToken checks the provided token against Redis and returns its JTI.
// // func validateRefreshToken(userID, token string) (jti string, err error) {
// // 	hash := sha256.Sum256([]byte(token))
// // 	iter := rdx.Conn.ScanKeys(fmt.Sprintf("refresh:%s:*", userID))
// // 	for iter.Next() {
// // 		key := iter.Val()
// // 		stored, _ := rdx.RdxGet(key)
// // 		if stored == hex.EncodeToString(hash[:]) {
// // 			// extract jti from key: refresh:<userID>:<jti>
// // 			parts := strings.Split(key, ":")
// // 			if len(parts) == 3 {
// // 				return parts[2], nil
// // 			}
// // 		}
// // 	}
// // 	return "", errors.New("refresh token not found or expired")
// // }

// // // invalidateTokens removes both access and refresh entries.
// // func invalidateTokens(userID, jti string) {
// // 	// Remove refresh token
// // 	rtKey := fmt.Sprintf("refresh:%s:%s", userID, jti)
// // 	rdx.RdxDel(rtKey)
// // 	// Optionally track blacklisted access tokens (not shown)
// // }

// // // HANDLERS

// // // registerHandler creates a new user (with optional email verification).
// // func registerHandler(w http.ResponseWriter, r *http.Request) {
// // 	var req struct {
// // 		Username string `json:"username"`
// // 		Email    string `json:"email"`
// // 		Password string `json:"password"`
// // 	}
// // 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// // 		log.Printf("register: invalid input: %v", err)
// // 		http.Error(w, "Invalid input", http.StatusBadRequest)
// // 		return
// // 	}

// // 	// Check for existing user
// // 	var existing structs.User
// // 	err := db.UserCollection.FindOne(context.TODO(), bson.M{"username": req.Username}).Decode(&existing)
// // 	if err == nil {
// // 		http.Error(w, "User already exists", http.StatusConflict)
// // 		return
// // 	} else if err != mongo.ErrNoDocuments {
// // 		log.Printf("register: db error: %v", err)
// // 		http.Error(w, "Internal error", http.StatusInternalServerError)
// // 		return
// // 	}

// // 	hashed, err := hashPassword(req.Password)
// // 	if err != nil {
// // 		log.Printf("register: bcrypt error: %v", err)
// // 		http.Error(w, "Could not hash password", http.StatusInternalServerError)
// // 		return
// // 	}

// // 	user := structs.User{
// // 		UserID:        uuid.NewString(),
// // 		Username:      req.Username,
// // 		Email:         req.Email,
// // 		PasswordHash:  hashed,
// // 		EmailVerified: !enableEmailVerification,
// // 		Role:          []string{"user"},
// // 		CreatedAt:     time.Now(),
// // 		UpdatedAt:     time.Now(),
// // 	}

// // 	// Optional: send verification email & OTP if enabled
// // 	if enableEmailVerification {
// // 		otp := utils.GenerateOTP(6)
// // 		if err := utils.SendEmailOTP(user.Email, otp); err != nil {
// // 			log.Printf("register: failed to send OTP: %v", err)
// // 		} else {
// // 			rdx.SetWithExpiry("otp:"+user.Email, otp, 10*time.Minute)
// // 		}
// // 	}

// // 	if _, err := db.UserCollection.InsertOne(context.TODO(), user); err != nil {
// // 		log.Printf("register: insert error: %v", err)
// // 		http.Error(w, "Failed to register", http.StatusInternalServerError)
// // 		return
// // 	}

// // 	resp := map[string]any{"userid": user.UserID}
// // 	if enableEmailVerification {
// // 		resp["message"] = "OTP sent; please verify email"
// // 	} else {
// // 		resp["message"] = "User registered"
// // 	}
// // 	utils.SendResponse(w, http.StatusCreated, resp, resp["message"].(string), nil)
// // }

// // // loginHandler authenticates and issues tokens.
// // func loginHandler(w http.ResponseWriter, r *http.Request) {
// // 	var req struct {
// // 		Username string `json:"username"`
// // 		Password string `json:"password"`
// // 	}
// // 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// // 		log.Printf("login: invalid input: %v", err)
// // 		http.Error(w, "Invalid input", http.StatusBadRequest)
// // 		return
// // 	}

// // 	var user structs.User
// // 	if err := db.UserCollection.FindOne(context.TODO(), bson.M{"username": req.Username}).Decode(&user); err != nil {
// // 		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
// // 		return
// // 	}

// // 	if enableEmailVerification && !user.EmailVerified {
// // 		http.Error(w, "Email not verified", http.StatusUnauthorized)
// // 		return
// // 	}

// // 	if err := verifyPassword(user.PasswordHash, req.Password); err != nil {
// // 		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
// // 		return
// // 	}

// // 	// Issue tokens
// // 	accessToken, refreshToken, err := generateTokens(user)
// // 	if err != nil {
// // 		log.Printf("login: token gen error: %v", err)
// // 		http.Error(w, "Failed to generate tokens", http.StatusInternalServerError)
// // 		return
// // 	}

// // 	// Update last login
// // 	db.UserCollection.UpdateOne(context.TODO(),
// // 		bson.M{"userid": user.UserID},
// // 		bson.M{"$set": bson.M{"last_login": time.Now()}},
// // 	)

// // 	utils.SendResponse(w, http.StatusOK, map[string]string{
// // 		"accessToken":  accessToken,
// // 		"refreshToken": refreshToken,
// // 	}, "Login successful", nil)
// // }

// // // logoutUserHandler revokes the user's refresh token (and optionally blacklists access).
// // func logoutUserHandler(w http.ResponseWriter, r *http.Request) {
// // 	tokenStr := middleware.ExtractBearer(r.Header.Get("Authorization"))
// // 	claims := &middleware.Claims{}
// // 	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
// // 		return jwtSecret, nil
// // 	})
// // 	if err != nil {
// // 		http.Error(w, "Invalid token", http.StatusUnauthorized)
// // 		return
// // 	}

// // 	// Invalidate by JTI
// // 	invalidateTokens(claims.UserID, claims.ID)
// // 	mq.Emit("user-loggedout", struct{ UserID string }{claims.UserID})

// // 	utils.SendResponse(w, http.StatusOK, nil, "Logged out", nil)
// // }

// // // refreshTokenHandler checks the refresh token, rotates it, and returns a new access token.
// // func refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
// // 	var req struct {
// // 		RefreshToken string `json:"refreshToken"`
// // 	}
// // 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// // 		http.Error(w, "Invalid input", http.StatusBadRequest)
// // 		return
// // 	}

// // 	// Extract access token to get userID
// // 	accessTokenStr := middleware.ExtractBearer(r.Header.Get("Authorization"))
// // 	claims := &middleware.Claims{}
// // 	_, err := jwt.ParseWithClaims(accessTokenStr, claims, func(t *jwt.Token) (any, error) {
// // 		return jwtSecret, nil
// // 	}, jwt.WithoutClaimsValidation())
// // 	if err != nil {
// // 		http.Error(w, "Invalid access token", http.StatusUnauthorized)
// // 		return
// // 	}

// // 	// Validate provided refresh token
// // 	oldJTI, err := validateRefreshToken(claims.UserID, req.RefreshToken)
// // 	if err != nil {
// // 		http.Error(w, "Invalid or expired refresh token", http.StatusUnauthorized)
// // 		return
// // 	}

// // 	// Rotate: invalidate old, issue new
// // 	invalidateTokens(claims.UserID, oldJTI)

// // 	var user structs.User
// // 	if err := db.UserCollection.FindOne(context.TODO(), bson.M{"userid": claims.UserID}).Decode(&user); err != nil {
// // 		http.Error(w, "User not found", http.StatusUnauthorized)
// // 		return
// // 	}

// // 	newAT, newRT, err := generateTokens(user)
// // 	if err != nil {
// // 		http.Error(w, "Could not generate new tokens", http.StatusInternalServerError)
// // 		return
// // 	}

// // 	utils.SendResponse(w, http.StatusOK, map[string]string{
// // 		"accessToken":  newAT,
// // 		"refreshToken": newRT,
// // 	}, "Token refreshed", nil)
// // }

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
