package beats

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"naevis/db"
	"naevis/globals"
	"naevis/middleware"
	"naevis/models"
	"naevis/mq"
	"naevis/userdata"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetFollowing(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	claims, ok := r.Context().Value(globals.UserIDKey).(*middleware.Claims)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := claims.UserID

	var userFollow models.UserFollow
	err := db.FollowingsCollection.FindOne(context.TODO(), bson.M{"userid": userID}).Decode(&userFollow)
	if err != nil && err != mongo.ErrNoDocuments {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	following := []models.User{}
	for _, followingID := range userFollow.Follows {
		var followUser models.User
		if err := db.FollowingsCollection.FindOne(context.TODO(), bson.M{"userid": followingID}).Decode(&followUser); err == nil {
			following = append(following, followUser)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(following)
}

func HandleFollowAction(w http.ResponseWriter, r *http.Request, ps httprouter.Params, action string) {
	ctx := r.Context()

	currentUserID := r.Context().Value(globals.UserIDKey).(string)
	targetUserID := ps.ByName("id")

	if err := UpdateFollowRelationship(ctx, currentUserID, targetUserID, action); err != nil {
		log.Printf("Error updating follow relationship: %v", err)
		http.Error(w, "Failed to update follow relationship", http.StatusInternalServerError)
		return
	}

	userdata.SetUserData(action, targetUserID, currentUserID, "profile", targetUserID)

	response := map[string]any{
		"isFollowing": action == "follow",
		"ok":          true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func ToggleFollow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	HandleFollowAction(w, r, ps, "follow")
}

func ToggleUnFollow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	HandleFollowAction(w, r, ps, "unfollow")
}
func GetFollowers(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := claims.UserID

	var userFollow models.UserFollow
	err = db.FollowingsCollection.FindOne(context.TODO(), bson.M{"userid": userID}).Decode(&userFollow)
	if err != nil && err != mongo.ErrNoDocuments {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if len(userFollow.Followers) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]models.User{})
		return
	}

	cursor, err := db.FollowingsCollection.Find(context.TODO(), bson.M{"userid": bson.M{"$in": userFollow.Followers}})
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	followers := []models.User{}
	if err = cursor.All(context.TODO(), &followers); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(followers)
}
func DoesFollow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID := claims.UserID
	followedUserID := ps.ByName("id")

	if followedUserID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Check directly in MongoDB instead of fetching full list
	count, err := db.FollowingsCollection.CountDocuments(context.TODO(), bson.M{
		"userid": userID,
		"follows": bson.M{
			"$in": []string{followedUserID},
		},
	})
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := map[string]bool{"isFollowing": count > 0}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func UpdateFollowRelationship(ctx context.Context, currentUserID, targetUserID, action string) error {
	if action != "follow" && action != "unfollow" {
		return fmt.Errorf("invalid action: %s", action)
	}

	// Update current user's follow list
	currentUserUpdate := bson.M{
		"$addToSet": bson.M{"follows": targetUserID},
	}
	if action == "unfollow" {
		currentUserUpdate = bson.M{
			"$pull": bson.M{"follows": targetUserID},
		}
	}
	_, err := db.FollowingsCollection.UpdateOne(
		context.TODO(),
		bson.M{"userid": currentUserID},
		currentUserUpdate,
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("failed to update current user's follows: %w", err)
	}

	// Update target user's followers list
	targetUserUpdate := bson.M{
		"$addToSet": bson.M{"followers": currentUserID},
	}
	if action == "unfollow" {
		targetUserUpdate = bson.M{
			"$pull": bson.M{"followers": currentUserID},
		}
	}
	_, err = db.FollowingsCollection.UpdateOne(
		context.TODO(),
		bson.M{"userid": targetUserID},
		targetUserUpdate,
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("failed to update target user's followers: %w", err)
	}

	if action == "unfollow" {
		m := models.Index{EntityType: "follow", EntityId: currentUserID, Method: "DELETE", ItemId: targetUserID}
		go mq.Emit(ctx, "unfllowed", m)

	} else {
		m := models.Index{EntityType: "follow", EntityId: currentUserID, Method: "PUT", ItemId: targetUserID}
		go mq.Emit(ctx, "followed", m)

	}

	return nil
}

func CreateFollowEntry(userid string) {
	var follow models.UserFollow
	fmt.Println("::::::::::::::::::::::::::::", userid)
	follow.UserID = userid
	// Insert the place into MongoDB
	_, err := db.FollowingsCollection.InsertOne(context.TODO(), follow)
	if err != nil {
		log.Printf("Error inserting place: %v", err)
		return
	}
}
