package media

import (
	"encoding/json"
	"naevis/db"
	"naevis/globals"
	"naevis/models"
	"naevis/mq"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func EditMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")
	mediaID := ps.ByName("id")

	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok || requestingUserID == "" {
		http.Error(w, "Invalid user", http.StatusUnauthorized)
		return
	}

	var payload struct {
		Caption     *string  `json:"caption,omitempty"`
		Description *string  `json:"description,omitempty"`
		Visibility  *string  `json:"visibility,omitempty"`
		Tags        []string `json:"tags,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Find the media first to get the MediaGroupID
	var media models.Media
	err := db.MediaCollection.FindOne(ctx, bson.M{
		"entityid":   entityID,
		"entitytype": entityType,
		"mediaid":    mediaID,
	}).Decode(&media)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Media not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Authorization: only creator can edit
	if media.CreatorID != requestingUserID {
		http.Error(w, "Not authorized to edit this media", http.StatusForbidden)
		return
	}

	// Build update document
	update := bson.M{"updatedAt": time.Now()}
	if payload.Caption != nil {
		update["caption"] = *payload.Caption
	}
	if payload.Description != nil {
		update["description"] = *payload.Description
	}
	if payload.Visibility != nil {
		update["visibility"] = *payload.Visibility
	}
	if payload.Tags != nil {
		update["tags"] = payload.Tags
	}

	// Apply update to all media in the same group
	filter := bson.M{"mediaGroupId": media.MediaGroupID}
	_, err = db.MediaCollection.UpdateMany(ctx, filter, bson.M{"$set": update})
	if err != nil {
		http.Error(w, "Failed to update media group", http.StatusInternalServerError)
		return
	}

	// // Clear cache for all affected media
	// for _, m := range getMediaIDsByGroup(ctx, media.MediaGroupID) {
	// 	cacheKey := fmt.Sprintf("media:%s:%s", entityID, m)
	// 	_ = rdx.RdxDel(cacheKey)
	// }

	// Emit MQ event for the group
	go mq.Emit(ctx, "media-edited", models.Index{
		EntityType: "media",
		EntityId:   media.MediaGroupID,
		Method:     "PUT",
		ItemType:   entityType,
		ItemId:     entityID,
	})

	// Return updated media group
	var updatedMedias []models.Media
	cur, err := db.MediaCollection.Find(ctx, bson.M{"mediaGroupId": media.MediaGroupID})
	if err == nil {
		defer cur.Close(ctx)
		for cur.Next(ctx) {
			var m models.Media
			if err := cur.Decode(&m); err == nil {
				updatedMedias = append(updatedMedias, m)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedMedias)
}
