package media

import (
	"encoding/json"
	"fmt"
	"naevis/db"
	"naevis/feed"
	"naevis/filemgr"
	"naevis/globals"
	"naevis/mq"
	"naevis/rdx"
	"naevis/structs"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

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

	files := r.MultipartForm.File["media"]
	if len(files) == 0 {
		http.Error(w, "No media file provided", http.StatusBadRequest)
		return
	}

	header := files[0]
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = utils.GuessMimeType(header.Filename)
	}
	media.MimeType = mimeType

	var picType filemgr.PictureType
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		media.Type = "image"
		picType = filemgr.PicPhoto
	case strings.HasPrefix(mimeType, "video/"):
		media.Type = "video"
		picType = filemgr.PicVideo
	default:
		http.Error(w, "Unsupported media type: "+mimeType, http.StatusUnsupportedMediaType)
		return
	}

	filename, err := filemgr.SaveFormFile(r.MultipartForm, "media", filemgr.EntityMedia, picType, true)
	if err != nil {
		http.Error(w, "Media upload failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	media.URL = filename

	fullPath := filepath.Join(filemgr.ResolvePath(filemgr.EntityMedia, picType), filename)

	if media.Type == "video" {
		media.FileSize = header.Size
		media.Duration = feed.ExtractVideoDuration(fullPath)
		posterPath := filepath.Join(filemgr.ResolvePath(filemgr.EntityMedia, filemgr.PicThumb), media.ID+".jpg")
		if err := feed.CreatePoster(fullPath, posterPath); err != nil {
			fmt.Printf("Poster creation failed: %v\n", err)
		} else {
			fmt.Printf("Poster created at %s\n", posterPath)
		}
	}

	_, err = db.MediaCollection.InsertOne(r.Context(), media)
	if err != nil {
		http.Error(w, "Error saving media to database: "+err.Error(), http.StatusInternalServerError)
		return
	}

	userdata.SetUserData("media", media.ID, requestingUserID, entityType, entityID)

	go mq.Emit("media-created", mq.Index{
		EntityType: "media",
		EntityId:   media.ID,
		Method:     "POST",
		ItemType:   entityType,
		ItemId:     entityID,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(media)
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
