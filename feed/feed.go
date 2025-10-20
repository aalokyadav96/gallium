package feed

import (
	"context"
	"encoding/json"
	"log"
	"naevis/db"
	"naevis/filedrop"
	"naevis/globals"
	"naevis/models"
	"naevis/mq"
	"naevis/rdx"
	"naevis/userdata"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func GetPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.ByName("postid")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Step 1: Fetch post from MongoDB
	var post models.FeedPost
	err := db.PostsCollection.FindOne(ctx, bson.M{"postid": id}).Decode(&post)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Post not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to fetch post", http.StatusInternalServerError)
		}
		return
	}

	// Step 2: Fetch like count from Redis
	redisKey := "like:count:post:" + id
	countStr, err := rdx.Conn.Get(ctx, redisKey).Result()
	var likeCount int64

	switch err {
	case nil:
		likeCount, err = strconv.ParseInt(countStr, 10, 64)
		if err != nil {
			log.Println("Failed to parse Redis like count:", countStr)
		}
	case redis.Nil:
		// Redis miss → fallback to MongoDB count
		likeCount, _ = db.LikesCollection.CountDocuments(ctx, bson.M{
			"entity_type": "post",
			"entity_id":   id,
		})
		// Optionally cache it in Redis
		_ = rdx.Conn.Set(ctx, redisKey, likeCount, 10*time.Minute).Err()
	default:
		log.Println("Redis Get error:", err)
	}

	// Step 3: Update post document with latest like count
	post.Likes = likeCount // assuming FeedPost has a Likes field

	_, err = db.PostsCollection.UpdateOne(ctx, bson.M{"postid": id}, bson.M{
		"$set": bson.M{"likes": likeCount},
	})
	if err != nil {
		log.Println("MongoDB update error:", err)
	}

	// Step 4: Return enriched post
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(post); err != nil {
		http.Error(w, "Failed to encode post data", http.StatusInternalServerError)
	}
}

// deletePost handles deleting a post by ID
func DeletePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	postID := ps.ByName("postid")

	if postID == "" {
		http.Error(w, "Post ID is required", http.StatusBadRequest)
		return
	}

	// Retrieve the ID of the requesting user from the context
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	// // Convert postID to ObjectID
	// id, err := objectIDFromString(postID)
	// if err != nil {
	// 	http.Error(w, "Invalid post ID", http.StatusBadRequest)
	// 	return
	// }

	// Delete the post from MongoDB
	// db.FeedPostsCollection := client.Database("eventdb").Collection("posts")
	result, err := db.PostsCollection.DeleteOne(context.TODO(), bson.M{"postid": postID})
	if err != nil {
		http.Error(w, "Failed to delete post", http.StatusInternalServerError)
		return
	}

	// Check if the file exists
	var existingFile models.FileMetadata
	db.FilesCollection.FindOne(context.TODO(), bson.M{"postid": postID}).Decode(&existingFile)

	filedrop.RemoveUserFile(requestingUserID, postID, existingFile.Hash)

	if result.DeletedCount == 0 {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	userdata.DelUserData("feedpost", postID, requestingUserID)

	m := models.Index{EntityType: "feedpost", EntityId: postID, Method: "DELETE"}
	go mq.Emit(ctx, "post-deleted", m)

	// Respond with a success message
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"message": "Post deleted successfully",
	})
}
