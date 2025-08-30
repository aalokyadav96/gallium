package filemgr

import (
	"context"
	"fmt"
	"log"
	"naevis/db"
	"naevis/globals"
	"naevis/models"
	"naevis/mq"
	"naevis/rdx"
	"naevis/utils"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// --- Entity metadata ---
type entityMeta struct {
	collection  *mongo.Collection
	keyField    string
	cachePrefix string
	ownerField  string
}

func getEntityMeta(entityType string) entityMeta {
	switch strings.ToLower(entityType) {
	case "place":
		return entityMeta{db.PlacesCollection, "placeid", "place:", "createdBy"}
	case "event":
		return entityMeta{db.EventsCollection, "eventid", "event:", "creatorid"}
	case "baito":
		return entityMeta{db.BaitoCollection, "baitoid", "baito:", "ownerId"}
	case "worker":
		return entityMeta{db.BaitoWorkerCollection, "workerid", "worker:", "userid"}
	case "artist":
		return entityMeta{db.ArtistsCollection, "artistid", "artist:", "creatorid"}
	case "farm":
		return entityMeta{db.FarmsCollection, "farmid", "farm:", "createdBy"}
	case "crop":
		return entityMeta{db.CropsCollection, "cropid", "crop:", "createdby"}
	case "feedpost":
		return entityMeta{db.PostsCollection, "postid", "feedpost:", "userid"}
	default:
		return entityMeta{}
	}
}

// --- Authorization ---
func authorizeUserForEntity(ctx context.Context, entityType, entityID, userID string) error {
	meta := getEntityMeta(entityType)
	if meta.collection == nil {
		return fmt.Errorf("unsupported entity type")
	}

	// Fetch only the owner field
	var result bson.M
	err := meta.collection.FindOne(
		ctx,
		bson.M{meta.keyField: entityID},
		options.FindOne().SetProjection(bson.M{meta.ownerField: 1}),
	).Decode(&result)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("not found")
		}
		return fmt.Errorf("database error")
	}

	owner, _ := result[meta.ownerField].(string)
	if owner != userID {
		return fmt.Errorf("unauthorized")
	}
	return nil
}

// --- Update with cache invalidation ---
func updateEntityBannerInDB(w http.ResponseWriter, entityType, entityID string, updateFields bson.M) error {
	meta := getEntityMeta(entityType)
	if meta.collection == nil {
		http.Error(w, "Unsupported entity type", http.StatusBadRequest)
		return fmt.Errorf("unsupported entity type: %s", entityType)
	}

	_, err := meta.collection.UpdateOne(context.TODO(), bson.M{meta.keyField: entityID}, bson.M{"$set": updateFields})
	if err != nil {
		http.Error(w, fmt.Sprintf("Error updating %s", entityType), http.StatusInternalServerError)
		return err
	}

	if _, err := rdx.RdxDel(meta.cachePrefix + entityID); err != nil {
		log.Printf("Cache deletion failed for %s ID: %s. Error: %v", entityType, entityID, err)
	} else {
		log.Printf("Cache invalidated for %s ID: %s", entityType, entityID)
	}

	return nil
}

// --- Handler ---
func EditBanner(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityTypeStr := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")

	meta := getEntityMeta(entityTypeStr)
	if meta.collection == nil {
		http.Error(w, "Unsupported entity type", http.StatusBadRequest)
		return
	}

	// Validate user from context
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok || requestingUserID == "" {
		http.Error(w, "Invalid user", http.StatusUnauthorized)
		return
	}

	// Authorization check
	if err := authorizeUserForEntity(r.Context(), entityTypeStr, entityID, requestingUserID); err != nil {
		switch err.Error() {
		case "not found":
			http.Error(w, fmt.Sprintf("%s not found", entityTypeStr), http.StatusNotFound)
		case "unauthorized":
			http.Error(w, "You are not authorized to edit this "+entityTypeStr, http.StatusForbidden)
		default:
			http.Error(w, "Internal error", http.StatusInternalServerError)
		}
		return
	}

	// Parse multipart form for banner/photo file
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// detect which field exists: "banner" or "photo"
	var field string
	var etype PictureType
	if _, ok := r.MultipartForm.File["banner"]; ok {
		field = "banner"
		etype = PicBanner
	} else if _, ok := r.MultipartForm.File["photo"]; ok {
		field = "photo"
		etype = PicPhoto
	} else {
		http.Error(w, "No banner or photo file uploaded", http.StatusBadRequest)
		return
	}

	// Save file
	fileName, err := SaveFormFile(r.MultipartForm, field, EntityType(entityTypeStr), etype, true)
	if err != nil {
		http.Error(w, fmt.Sprintf("%s upload failed: %v", field, err), http.StatusBadRequest)
		return
	}
	if fileName == "" {
		http.Error(w, fmt.Sprintf("No %s file uploaded", field), http.StatusBadRequest)
		return
	}

	updateFields := bson.M{
		field:        fileName, // always stored in DB as banner
		"updated_at": time.Now(),
	}

	if err := updateEntityBannerInDB(w, entityTypeStr, entityID, updateFields); err != nil {
		return
	}

	go mq.Emit(r.Context(), fmt.Sprintf("%s-edited", entityTypeStr), models.Index{
		EntityType: entityTypeStr,
		EntityId:   entityID,
		Method:     "PUT",
	})

	utils.RespondWithJSON(w, http.StatusOK, updateFields)
}

// // --- Handler ---
// func EditBanner(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	entityTypeStr := ps.ByName("entitytype")
// 	entityID := ps.ByName("entityid")

// 	meta := getEntityMeta(entityTypeStr)
// 	if meta.collection == nil {
// 		http.Error(w, "Unsupported entity type", http.StatusBadRequest)
// 		return
// 	}

// 	// Validate user from context
// 	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
// 	if !ok || requestingUserID == "" {
// 		http.Error(w, "Invalid user", http.StatusUnauthorized)
// 		return
// 	}

// 	// Authorization check
// 	if err := authorizeUserForEntity(r.Context(), entityTypeStr, entityID, requestingUserID); err != nil {
// 		switch err.Error() {
// 		case "not found":
// 			http.Error(w, fmt.Sprintf("%s not found", entityTypeStr), http.StatusNotFound)
// 		case "unauthorized":
// 			http.Error(w, "You are not authorized to edit this "+entityTypeStr, http.StatusForbidden)
// 		default:
// 			http.Error(w, "Internal error", http.StatusInternalServerError)
// 		}
// 		return
// 	}

// 	// Parse multipart form for banner file
// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		http.Error(w, "Unable to parse form", http.StatusBadRequest)
// 		return
// 	}

// 	// Save banner file
// 	bannerFileName, err := SaveFormFile(r.MultipartForm, "banner", EntityType(entityTypeStr), PicBanner, true)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("Banner upload failed: %v", err), http.StatusBadRequest)
// 		return
// 	}
// 	if bannerFileName == "" {
// 		http.Error(w, "No banner file uploaded", http.StatusBadRequest)
// 		return
// 	}

// 	updateFields := bson.M{
// 		"banner":     bannerFileName,
// 		"updated_at": time.Now(),
// 	}

// 	if err := updateEntityBannerInDB(w, entityTypeStr, entityID, updateFields); err != nil {
// 		return
// 	}

// 	go mq.Emit(r.Context(), fmt.Sprintf("%s-edited", entityTypeStr), models.Index{
// 		EntityType: entityTypeStr,
// 		EntityId:   entityID,
// 		Method:     "PUT",
// 	})

// 	utils.RespondWithJSON(w, http.StatusOK, updateFields)
// }
