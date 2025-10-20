package profile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"naevis/db"
	"naevis/middleware"
	"naevis/models"
	"naevis/rdx"
	"naevis/utils"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Handlers for user profile
func GetUserProfile(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// 1. Extract and validate the JWT from the Authorization header (strip "Bearer " if present).
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	tokenString := authHeader
	// if strings.HasPrefix(authHeader, "Bearer ") {
	// 	tokenString = strings.TrimPrefix(authHeader, "Bearer ")
	// }

	claims, err := middleware.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Get the target username from URL parameters.
	username := ps.ByName("username")

	// 3. Look up the user in MongoDB using the request context.
	var user models.User
	err = db.UserCollection.FindOne(r.Context(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// 4. Fetch follow data for that user.
	userFollow, err := GetUserFollowData(user.UserID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// 5. Check online status in Redis.
	user.Online = rdx.Exists("online:" + user.UserID)

	// 6. Build a trimmed-down response DTO so we don’t accidentally expose fields like PasswordHash.
	userProfile := models.UserProfileResponse{
		UserID:         user.UserID,
		Username:       user.Username,
		Email:          user.Email,
		Name:           user.Name,
		Bio:            user.Bio,
		Avatar:         user.Avatar,
		Banner:         user.Banner,
		FollowersCount: len(userFollow.Followers),
		FollowingCount: len(userFollow.Follows),
		IsFollowing:    utils.Contains(userFollow.Followers, claims.UserID),
		Online:         user.Online,
		LastLogin:      user.LastLogin,
	}

	// 7. Write JSON response.
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(userProfile); err != nil {
		// At this point, headers are already sent. Nothing more to do.
	}
}

func GetProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// 1. Extract and validate the JWT from the Authorization header (strip "Bearer " if present).
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	tokenString := authHeader
	// if strings.HasPrefix(authHeader, "Bearer ") {
	// 	tokenString = strings.TrimPrefix(authHeader, "Bearer ")
	// }

	claims, err := middleware.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// // 2. Attempt to read a cached JSON profile from Redis. If it exists (non-empty), return it immediately.
	// if cachedJSON, err := GetCachedProfile(claims.Username); err == nil && cachedJSON != "" {
	// 	w.Header().Set("Content-Type", "application/json")
	// 	w.Write([]byte(cachedJSON))
	// 	return
	// }

	// 3. Fetch follow data for this user.
	userFollow, err := GetUserFollowData(claims.UserID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// 4. Look up the user document in MongoDB using the request context.
	var user models.User
	err = db.UserCollection.FindOne(r.Context(), bson.M{"userid": claims.UserID}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// 5. Clear any sensitive fields before serializing.
	// user.Password = ""

	// 6. Populate follower/follow counts and online status.
	user.FollowersCount = len(userFollow.Followers)
	user.FollowingCount = len(userFollow.Follows)
	user.Online = rdx.Exists("online:" + user.UserID)

	// 7. Marshal to JSON and cache the result.
	profileJSON, err := json.Marshal(user)
	if err != nil {
		http.Error(w, "Failed to encode profile", http.StatusInternalServerError)
		return
	}

	// Best-effort cache write; ignore errors here.
	// _ = CacheProfile(claims.Username, string(profileJSON))

	// 8. Return the JSON.
	w.Header().Set("Content-Type", "application/json")
	w.Write(profileJSON)
}

func RespondWithUserProfile(w http.ResponseWriter, userid string) error {
	var userProfile models.User
	err := db.UserCollection.FindOne(context.TODO(), bson.M{"userid": userid}).Decode(&userProfile)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			// 404 is already written; caller should not write anything else.
			http.Error(w, "User not found", http.StatusNotFound)
			return nil
		}
		return err
	}

	// Clear sensitive fields before encoding
	// userProfile.Password = ""

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(userProfile)
}

func GetUserByUsername(username string) (*models.User, error) {
	var user models.User
	err := db.UserCollection.FindOne(context.TODO(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil // Caller must check for nil => not found
		}
		return nil, err
	}
	return &user, nil
}

func GetUserFollowData(userID string) (models.UserFollow, error) {
	var userFollow models.UserFollow
	err := db.FollowingsCollection.FindOne(context.TODO(), bson.M{"userid": userID}).Decode(&userFollow)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.UserFollow{
				Followers: []string{},
				Follows:   []string{},
			}, nil
		}
		return userFollow, err
	}
	return userFollow, nil
}

func CacheProfile(username string, profileJSON string) error {
	return rdx.RdxSet("profile:"+username, profileJSON)
}

func GetCachedProfile(username string) (string, error) {
	return rdx.RdxGet("profile:" + username)
}

func InvalidateCachedProfile(username string) error {
	_, err := rdx.RdxDel("profile:" + username)
	return err
}

func UpdateCachedUsername(userid string) error {
	_, err := rdx.RdxDel(fmt.Sprintf("users:%s", userid))
	return err
}
