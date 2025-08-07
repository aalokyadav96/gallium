package feed

import (
	"context"
	"encoding/json"
	"log"
	"naevis/db"
	"naevis/globals"
	"naevis/middleware"
	"naevis/mq"
	"naevis/structs"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func CreateTweetPost(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(tokenString)
	if err != nil {
		log.Printf("JWT validation error: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	userID := claims.UserID
	username := claims.Username

	postType := r.FormValue("type")
	postText := r.FormValue("text")

	if postType == "" {
		postType = "text"
	}
	postText = utils.SanitizeText(postText) // optional

	validPostTypes := map[string]bool{
		"text": true, "image": true, "video": true, "audio": true, "blog": true, "merchandise": true,
	}
	if !validPostTypes[postType] {
		http.Error(w, "Invalid post type", http.StatusBadRequest)
		return
	}

	newPost := structs.Post{
		PostID:    utils.GenerateID(12),
		Username:  username,
		UserID:    userID,
		Text:      postText,
		Timestamp: time.Now().Format(time.RFC3339),
		Likes:     0,
		Type:      postType,
	}

	var (
		mediaPaths []string
		mediaNames []string
		mediaRes   []int
	)

	switch postType {
	case "image":
		// mediaNames, err = saveUploadedFiles(r, "images", "image")
		mediaNames, err = saveUploadedFiles(r, "images", "photo")
	case "video":
		mediaRes, mediaPaths, mediaNames, err = saveUploadedVideoFile(r, "video")
	case "audio":
		mediaRes, mediaPaths, mediaNames, err = saveUploadedAudioFile(r, "audio")
	}

	if err != nil {
		http.Error(w, "Failed to upload "+postType+": "+err.Error(), http.StatusInternalServerError)
		return
	}
	if (postType != "text") && (len(mediaPaths) == 0 && len(mediaNames) == 0) {
		http.Error(w, "No media uploaded", http.StatusBadRequest)
		return
	}

	newPost.Resolutions = mediaRes
	newPost.MediaURL = mediaNames
	newPost.Media = mediaPaths

	if _, err := db.PostsCollection.InsertOne(r.Context(), newPost); err != nil {
		http.Error(w, "Failed to insert post into DB: "+err.Error(), http.StatusInternalServerError)
		return
	}

	userdata.SetUserData("feedpost", newPost.PostID, userID, "", "")
	go mq.Emit("post-created", mq.Index{
		EntityType: "feedpost",
		EntityId:   newPost.PostID,
		Method:     "POST",
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"message": "Post created successfully",
		"data":    newPost,
	})
}

// deletePost handles deleting a post by ID
func DeletePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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
	// db.PostsCollection := client.Database("eventdb").Collection("posts")
	result, err := db.PostsCollection.DeleteOne(context.TODO(), bson.M{"postid": postID})
	if err != nil {
		http.Error(w, "Failed to delete post", http.StatusInternalServerError)
		return
	}

	// Check if the file exists
	var existingFile FileMetadata
	db.FilesCollection.FindOne(context.TODO(), bson.M{"postid": postID}).Decode(&existingFile)

	RemoveUserFile(requestingUserID, postID, existingFile.Hash)

	if result.DeletedCount == 0 {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	userdata.DelUserData("feedpost", postID, requestingUserID)

	m := mq.Index{EntityType: "feedpost", EntityId: postID, Method: "DELETE"}
	go mq.Emit("post-deleted", m)

	// Respond with a success message
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"message": "Post deleted successfully",
	})
}
