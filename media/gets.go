package media

import (
	"context"
	"encoding/json"
	"naevis/db"
	"naevis/filemgr"
	"naevis/globals"
	"naevis/models"
	"naevis/mq"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func GetMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")
	mediaID := ps.ByName("id")

	var media models.Media
	err := db.MediaCollection.FindOne(r.Context(), bson.M{
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(media)
}

// ---------------------- Delete Media ----------------------
func DeleteMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")
	mediaID := ps.ByName("id")

	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok || requestingUserID == "" {
		http.Error(w, "Invalid user", http.StatusUnauthorized)
		return
	}

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

	if media.CreatorID != requestingUserID {
		http.Error(w, "Not authorized to delete this media", http.StatusForbidden)
		return
	}

	_, err = db.MediaCollection.DeleteOne(ctx, bson.M{"mediaid": mediaID})
	if err != nil {
		http.Error(w, "Failed to delete media", http.StatusInternalServerError)
		return
	}

	// Remove media file and thumbnail
	if media.URL != "" {
		mediaPath := filepath.Join(filemgr.ResolvePath(filemgr.EntityMedia, filemgr.PicPhoto), media.URL)
		_ = os.Remove(mediaPath)
	}
	thumbPath := filepath.Join(filemgr.ResolvePath(filemgr.EntityMedia, filemgr.PicThumb), media.MediaID+".jpg")
	_ = os.Remove(thumbPath)

	userdata.DelUserData("media", mediaID, requestingUserID)

	go mq.Emit(ctx, "media-deleted", models.Index{
		EntityType: "media",
		EntityId:   mediaID,
		Method:     "DELETE",
		ItemType:   entityType,
		ItemId:     entityID,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "Media deleted successfully"})
}

// Media
func GetMedias(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	filter := bson.M{"entityid": ps.ByName("entityid"), "entitytype": ps.ByName("entitytype")}
	medias, err := utils.FindAndDecode[models.Media](ctx, db.MediaCollection, filter)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve media")
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, medias)
}

// ---------------------- Get Media Groups ----------------------
func GetMediaGroups(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	filter := bson.M{
		"entityid":   ps.ByName("entityid"),
		"entitytype": ps.ByName("entitytype"),
	}

	cur, err := db.MediaCollection.Find(ctx, filter)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve media")
		return
	}
	defer cur.Close(ctx)

	mediaMap := make(map[string][]models.Media)
	for cur.Next(ctx) {
		var m models.Media
		if err := cur.Decode(&m); err == nil {
			mediaMap[m.MediaGroupID] = append(mediaMap[m.MediaGroupID], m)
		}
	}

	// convert map to slice of groups
	groups := make([]map[string]any, 0, len(mediaMap))
	for groupID, medias := range mediaMap {
		groups = append(groups, map[string]any{
			"groupId": groupID,
			"files":   medias,
		})
	}

	utils.RespondWithJSON(w, http.StatusOK, groups)
}
