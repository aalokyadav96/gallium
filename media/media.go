package media

import (
	"encoding/json"
	"fmt"
	"io"
	"naevis/db"
	"naevis/feed"
	"naevis/globals"
	"naevis/mq"
	"naevis/rdx"
	"naevis/structs"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var mediaUploadPath = "./static/uploads"

func AddMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")
	if entityID == "" {
		http.Error(w, "Entity ID is required", http.StatusBadRequest)
		return
	}

	err := r.ParseMultipartForm(50 << 20)
	if err != nil {
		http.Error(w, "Unable to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok || requestingUserID == "" {
		http.Error(w, "Invalid or missing user ID", http.StatusUnauthorized)
		return
	}

	media := structs.Media{
		EntityID:   entityID,
		EntityType: entityType,
		Caption:    r.FormValue("caption"),
		CreatorID:  requestingUserID,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if entityType == "event" {
		media.ID = "e" + utils.GenerateID(16)
	} else {
		media.ID = "p" + utils.GenerateID(16)
	}

	file, fileHeader, err := r.FormFile("media")
	if err != nil {
		if err == http.ErrMissingFile {
			http.Error(w, "Media file is required", http.StatusBadRequest)
		} else {
			http.Error(w, "Error retrieving media file: "+err.Error(), http.StatusBadRequest)
		}
		return
	}
	defer file.Close()

	mimeType := fileHeader.Header.Get("Content-Type")
	media.MimeType = mimeType

	// Determine extension and type
	var extension string
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		extension = filepath.Ext(fileHeader.Filename)
		media.Type = "image"
	case strings.HasPrefix(mimeType, "video/"):
		extension = ".mp4"
		media.Type = "video"
	default:
		http.Error(w, "Unsupported media type", http.StatusUnsupportedMediaType)
		return
	}

	// Save file using filemgr
	filename := media.ID + extension
	fullPath := filepath.Join(mediaUploadPath, filename)
	outFile, err := os.Create(fullPath)
	if err != nil {
		http.Error(w, "Error saving media file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, file); err != nil {
		http.Error(w, "Error writing media file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	media.URL = filename

	switch media.Type {
	case "video":
		media.FileSize = fileHeader.Size
		media.Duration = ExtractVideoDuration(fullPath)

		posterPath := filepath.Join(mediaUploadPath, media.ID+".jpg")
		feed.CreatePoster(fullPath, posterPath, "00:00:01")
		fmt.Printf("Default poster %s created successfully!\n", posterPath)

		// Optional: generate a thumbnail for the poster image
		// utils.CreateThumb(media.ID, mediaUploadPath, ".jpg", 150, 200)
	case "image":
		// Generate image thumbnail
		// Using your utility or imaging logic
		utils.CreateThumb(media.ID, mediaUploadPath, extension, 150, 200)
	}

	_, err = db.MediaCollection.InsertOne(r.Context(), media)
	if err != nil {
		http.Error(w, "Error saving media to database: "+err.Error(), http.StatusInternalServerError)
		return
	}

	userdata.SetUserData("media", media.ID, requestingUserID, entityType, entityID)

	m := mq.Index{EntityType: "media", EntityId: media.ID, Method: "POST", ItemType: entityType, ItemId: entityID}
	go mq.Emit("media-created", m)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(media)
}

// var mediaUploadPath = "./static/uploads"

// func AddMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	entityType := ps.ByName("entitytype")
// 	entityID := ps.ByName("entityid")
// 	if entityID == "" {
// 		http.Error(w, "Entity ID is required", http.StatusBadRequest)
// 		return
// 	}

// 	err := r.ParseMultipartForm(50 << 20) // Limit to 50 MB
// 	if err != nil {
// 		http.Error(w, "Unable to parse form: "+err.Error(), http.StatusBadRequest)
// 		return
// 	}

// 	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
// 	if !ok || requestingUserID == "" {
// 		http.Error(w, "Invalid or missing user ID", http.StatusUnauthorized)
// 		return
// 	}

// 	media := structs.Media{
// 		EntityID:   entityID,
// 		EntityType: entityType,
// 		Caption:    r.FormValue("caption"),
// 		CreatorID:  requestingUserID,
// 		CreatedAt:  time.Now(),
// 		UpdatedAt:  time.Now(),
// 	}

// 	if entityType == "event" {
// 		media.ID = "e" + utils.GenerateID(16)
// 	} else {
// 		media.ID = "p" + utils.GenerateID(16)
// 	}

// 	file, fileHeader, err := r.FormFile("media")
// 	if err != nil {
// 		if err == http.ErrMissingFile {
// 			http.Error(w, "Media file is required", http.StatusBadRequest)
// 		} else {
// 			http.Error(w, "Error retrieving media file: "+err.Error(), http.StatusBadRequest)
// 		}
// 		return
// 	}
// 	if file != nil {
// 		defer file.Close()
// 	}

// 	var fileExtension, mimeType string
// 	if file != nil {
// 		mimeType = fileHeader.Header.Get("Content-Type")
// 		switch {
// 		case strings.HasPrefix(mimeType, "image/"):
// 			fileExtension = ".jpg"
// 			media.Type = "image"
// 		case strings.HasPrefix(mimeType, "video/"):
// 			fileExtension = ".mp4"
// 			media.Type = "video"
// 		default:
// 			http.Error(w, "Unsupported media type", http.StatusUnsupportedMediaType)
// 			return
// 		}

// 		savePath := mediaUploadPath + "/" + media.ID + fileExtension
// 		out, err := os.Create(savePath)
// 		if err != nil {
// 			http.Error(w, "Error saving media file: "+err.Error(), http.StatusInternalServerError)
// 			return
// 		}
// 		defer out.Close()

// 		if _, err := io.Copy(out, file); err != nil {
// 			http.Error(w, "Error saving media file: "+err.Error(), http.StatusInternalServerError)
// 			return
// 		}

// 		media.URL = media.ID + fileExtension
// 		media.MimeType = mimeType
// 		if media.Type == "video" {
// 			media.FileSize = fileHeader.Size
// 			media.Duration = ExtractVideoDuration(savePath)
// 		}
// 		// Generate default poster from the original video
// 		defaultPosterPath := filepath.Join(mediaUploadPath, "/", media.ID+".jpg")
// 		feed.CreatePoster(savePath, defaultPosterPath, "00:00:01")

// 		utils.CreateThumb(media.ID, mediaUploadPath, ".jpg", 150, 200)

// 		fmt.Printf("Default poster %s created successfully!\n", defaultPosterPath)
// 	}

// 	_, err = db.MediaCollection.InsertOne(r.Context(), media)
// 	if err != nil {
// 		http.Error(w, "Error saving media to database: "+err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	userdata.SetUserData("media", media.ID, requestingUserID, entityType, entityID)

// 	m := mq.Index{EntityType: "media", EntityId: media.ID, Method: "POST", ItemType: entityType, ItemId: entityID}
// 	go mq.Emit("media-created", m)

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(media)
// }

func GetMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")
	mediaID := ps.ByName("id")

	var media structs.Media
	err := db.MediaCollection.FindOne(r.Context(), bson.M{
		"entityid":   entityID,
		"entitytype": entityType,
		"id":         mediaID,
	}).Decode(&media)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Media not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(media)
}

func GetMedias(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")

	cursor, err := db.MediaCollection.Find(r.Context(), bson.M{
		"entityid":   entityID,
		"entitytype": entityType,
	})
	if err != nil {
		http.Error(w, "Failed to retrieve media", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(r.Context())

	var medias []structs.Media
	if err = cursor.All(r.Context(), &medias); err != nil {
		http.Error(w, "Failed to parse media", http.StatusInternalServerError)
		return
	}

	if len(medias) == 0 {
		medias = []structs.Media{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(medias)
}

func DeleteMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")
	mediaID := ps.ByName("id")

	// Retrieve the ID of the requesting user from the context
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	_, err := db.MediaCollection.DeleteOne(r.Context(), bson.M{
		"entityid":   entityID,
		"entitytype": entityType,
		"id":         mediaID,
	})
	if err != nil {
		http.Error(w, "Failed to delete media", http.StatusInternalServerError)
		return
	}

	userdata.DelUserData("media", mediaID, requestingUserID)

	m := mq.Index{EntityType: "media", EntityId: mediaID, Method: "DELETE", ItemType: entityType, ItemId: entityID}
	go mq.Emit("media-deleted", m)

	// Respond with success
	w.WriteHeader(http.StatusOK)
	response := map[string]any{
		"status":  http.StatusNoContent,
		"message": "Media deleted successfully",
	}
	json.NewEncoder(w).Encode(response)
}

func ExtractVideoDuration(savePath string) int {
	_ = savePath
	return 5
}

func EditMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")
	mediaID := ps.ByName("id")
	cacheKey := fmt.Sprintf("media:%s:%s", entityID, mediaID)

	// Check the cache first
	cachedMedia, err := rdx.RdxGet(cacheKey)
	if err == nil && cachedMedia != "" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cachedMedia))
		return
	}

	// Fetch the media from MongoDB
	var media structs.Media
	err = db.MediaCollection.FindOne(r.Context(), bson.M{
		"entityid":   entityID,
		"entitytype": entityType,
		"id":         mediaID,
	}).Decode(&media)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Media not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Cache the result
	mediaJSON, _ := json.Marshal(media)
	rdx.RdxSet(cacheKey, string(mediaJSON))

	m := mq.Index{EntityType: "media", EntityId: mediaID, Method: "{PUT}", ItemType: entityType, ItemId: entityID}
	go mq.Emit("media-edited", m)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(media)
}
