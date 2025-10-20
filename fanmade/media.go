package fanmade

import (
	"encoding/json"
	"log"
	"naevis/db"
	"naevis/globals"
	"naevis/models"
	"naevis/mq"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
)

func AddMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")

	if entityID == "" {
		http.Error(w, "Entity ID is required", http.StatusBadRequest)
		return
	}

	requestingUserID, ok := ctx.Value(globals.UserIDKey).(string)
	if !ok || requestingUserID == "" {
		http.Error(w, "Invalid or missing user ID", http.StatusUnauthorized)
		return
	}

	var payload struct {
		Caption string                   `json:"caption"`
		Files   []map[string]interface{} `json:"files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(payload.Files) == 0 {
		http.Error(w, "No files provided", http.StatusBadRequest)
		return
	}

	mediaGroupID := "g" + utils.GenerateRandomString(16)
	var insertedMedia []models.Media

	for _, fileData := range payload.Files {
		filename, _ := fileData["filename"].(string)
		if filename == "" {
			continue
		}

		extn, _ := fileData["extn"].(string)

		var mediaType, mimeType string
		switch strings.ToLower(extn) {
		case ".jpg", ".jpeg", ".png":
			mediaType = models.MediaTypeImage
			mimeType = "image/" + strings.TrimPrefix(strings.ToLower(extn), ".")
		case ".mp4", ".webm":
			mediaType = models.MediaTypeVideo
			mimeType = "video/" + strings.TrimPrefix(strings.ToLower(extn), ".")
		default:
			mediaType = "unknown"
			mimeType = "application/octet-stream"
		}

		media := models.Media{
			MediaID:      "m" + utils.GenerateRandomString(16),
			MediaGroupID: mediaGroupID,
			EntityID:     entityID,
			EntityType:   entityType,
			Type:         mediaType,
			MimeType:     mimeType,
			Caption:      payload.Caption,
			CreatorID:    requestingUserID,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
			URL:          filename,
			Extn:         extn,
		}

		if _, err := db.MediaCollection.InsertOne(ctx, media); err != nil {
			log.Printf("Failed to insert media %s: %v", filename, err)
			continue
		}

		userdata.SetUserData("media", media.MediaID, requestingUserID, entityType, entityID)
		insertedMedia = append(insertedMedia, media)
	}

	go mq.Emit(ctx, "media-created", models.Index{
		EntityType: "media",
		EntityId:   mediaGroupID,
		Method:     "POST",
		ItemType:   entityType,
		ItemId:     entityID,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(insertedMedia)
}
