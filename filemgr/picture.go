package filemgr

import (
	"context"
	"errors"
	"fmt"
	"log"
	"mime/multipart"
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

// --- sentinel errors ---
var (
	ErrUnsupportedEntity = errors.New("unsupported entity type")
	ErrNotFound          = errors.New("not found")
	ErrUnauthorized      = errors.New("unauthorized")
)

// --- Entity metadata ---
type entityMeta struct {
	collection  *mongo.Collection
	keyField    string
	cachePrefix string
	ownerField  string
}

var entityMetaMap = map[string]entityMeta{
	"place":    {db.PlacesCollection, "placeid", "place:", "createdBy"},
	"event":    {db.EventsCollection, "eventid", "event:", "creatorid"},
	"baito":    {db.BaitoCollection, "baitoid", "baito:", "ownerId"},
	"worker":   {db.BaitoWorkerCollection, "baito_user_id", "worker:", "userid"},
	"artist":   {db.ArtistsCollection, "artistid", "artist:", "creatorid"},
	"farm":     {db.FarmsCollection, "farmid", "farm:", "createdBy"},
	"crop":     {db.CropsCollection, "cropid", "crop:", "createdby"},
	"feedpost": {db.PostsCollection, "postid", "feedpost:", "userid"},
}

func getEntityMeta(entityType string) (entityMeta, bool) {
	em, ok := entityMetaMap[strings.ToLower(entityType)]
	return em, ok
}

// --- Picture fields map ---
var pictureFieldMap = map[string]PictureType{
	"banner": PicBanner,
	"photo":  PicPhoto,
}

// --- Authorization ---
func authorizeUserForEntity(ctx context.Context, entityType, entityID, userID string) error {
	meta, ok := getEntityMeta(entityType)
	if !ok {
		return ErrUnsupportedEntity
	}

	var result bson.M
	findOpts := options.FindOne().SetProjection(bson.M{meta.ownerField: 1})
	err := meta.collection.FindOne(ctx, bson.M{meta.keyField: entityID}, findOpts).Decode(&result)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrNotFound
		}
		return fmt.Errorf("database error: %w", err)
	}

	owner, _ := result[meta.ownerField].(string)
	if owner != userID {
		return ErrUnauthorized
	}
	return nil
}

// --- Update with cache invalidation ---
func updateEntityBannerInDB(ctx context.Context, w http.ResponseWriter, entityType, entityID string, updateFields bson.M) error {
	meta, ok := getEntityMeta(entityType)
	if !ok {
		http.Error(w, "Unsupported entity type", http.StatusBadRequest)
		return ErrUnsupportedEntity
	}

	if _, err := meta.collection.UpdateOne(ctx, bson.M{meta.keyField: entityID}, bson.M{"$set": updateFields}); err != nil {
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

// --- small helper to handle auth errors consistently in handler ---
func handleAuthError(w http.ResponseWriter, err error, entityType string) {
	switch {
	case errors.Is(err, ErrNotFound):
		http.Error(w, fmt.Sprintf("%s not found", entityType), http.StatusNotFound)
	case errors.Is(err, ErrUnauthorized):
		http.Error(w, "You are not authorized to edit this "+entityType, http.StatusForbidden)
	default:
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

// --- File upload wrapper ---
func handleFileUpload(form *multipart.Form, field string, entity EntityType, picType PictureType) (string, error) {
	return SaveFormFile(form, field, entity, picType, true)
}

// --- Handler ---
func EditBanner(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityTypeStr := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")

	meta, ok := getEntityMeta(entityTypeStr)
	if !ok || meta.collection == nil {
		http.Error(w, "Unsupported entity type", http.StatusBadRequest)
		return
	}

	// Validate user from context
	requestingUserID, _ := r.Context().Value(globals.UserIDKey).(string)
	if requestingUserID == "" {
		http.Error(w, "Invalid user", http.StatusUnauthorized)
		return
	}

	// Authorization check
	if err := authorizeUserForEntity(r.Context(), entityTypeStr, entityID, requestingUserID); err != nil {
		handleAuthError(w, err, entityTypeStr)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// detect which allowed field exists
	var field string
	var etype PictureType
	for k, v := range pictureFieldMap {
		if _, found := r.MultipartForm.File[k]; found {
			field = k
			etype = v
			break
		}
	}
	if field == "" {
		http.Error(w, "No banner or photo file uploaded", http.StatusBadRequest)
		return
	}

	// Save file
	fileName, err := handleFileUpload(r.MultipartForm, field, EntityType(entityTypeStr), etype)
	if err != nil {
		http.Error(w, fmt.Sprintf("%s upload failed: %v", field, err), http.StatusBadRequest)
		return
	}
	if fileName == "" {
		http.Error(w, fmt.Sprintf("No %s file uploaded", field), http.StatusBadRequest)
		return
	}

	updateFields := bson.M{
		field:        fileName,
		"updated_at": time.Now(),
	}

	if err := updateEntityBannerInDB(r.Context(), w, entityTypeStr, entityID, updateFields); err != nil {
		return
	}

	// Best-effort MQ emit
	go mq.Emit(r.Context(), fmt.Sprintf("%s-edited", entityTypeStr), models.Index{
		EntityType: entityTypeStr,
		EntityId:   entityID,
		Method:     "PUT",
	})

	utils.RespondWithJSON(w, http.StatusOK, updateFields)
}
