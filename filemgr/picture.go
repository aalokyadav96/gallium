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
)

/**/
// Generic DB update and cache invalidation
func updateEntityBannerInDB(w http.ResponseWriter, entityType, entityID string, updateFields bson.M) error {
	collection, keyField, cachePrefix := getEntityMeta(entityType)
	if collection == nil {
		http.Error(w, "Unsupported entity type", http.StatusBadRequest)
		return fmt.Errorf("unsupported entity type: %s", entityType)
	}

	_, err := collection.UpdateOne(context.TODO(), bson.M{keyField: entityID}, bson.M{"$set": updateFields})
	if err != nil {
		http.Error(w, fmt.Sprintf("Error updating %s", entityType), http.StatusInternalServerError)
		return err
	}

	if _, err := rdx.RdxDel(cachePrefix + entityID); err != nil {
		log.Printf("Cache deletion failed for %s ID: %s. Error: %v", entityType, entityID, err)
	} else {
		log.Printf("Cache invalidated for %s ID: %s", entityType, entityID)
	}

	return nil
}

// Generic authorization check with type safety preserved
func authorizeUserForEntity(ctx context.Context, entityType, entityID, userID string) error {
	meta := getEntityAuthMeta(entityType)
	if meta.collection == nil {
		return fmt.Errorf("unsupported entity type")
	}

	switch strings.ToLower(entityType) {
	case "place":
		var place models.Place
		err := meta.collection.FindOne(ctx, bson.M{meta.keyField: entityID}).Decode(&place)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return fmt.Errorf("not found")
			}
			return fmt.Errorf("database error")
		}
		if place.CreatedBy != userID {
			return fmt.Errorf("unauthorized")
		}

	case "event":
		var event models.Event
		err := meta.collection.FindOne(ctx, bson.M{meta.keyField: entityID}).Decode(&event)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return fmt.Errorf("not found")
			}
			return fmt.Errorf("database error")
		}
		if event.CreatorID != userID {
			return fmt.Errorf("unauthorized")
		}

	case "baito":
		var baito models.Baito
		err := meta.collection.FindOne(ctx, bson.M{meta.keyField: entityID}).Decode(&baito)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return fmt.Errorf("not found")
			}
			return fmt.Errorf("database error")
		}
		if baito.OwnerID != userID {
			return fmt.Errorf("unauthorized")
		}

	case "artist":
		var artist models.Artist
		err := meta.collection.FindOne(ctx, bson.M{meta.keyField: entityID}).Decode(&artist)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return fmt.Errorf("not found")
			}
			return fmt.Errorf("database error")
		}
		if artist.CreatorID != userID {
			return fmt.Errorf("unauthorized")
		}

	default:
		return fmt.Errorf("unsupported entity type")
	}

	return nil
}

// type entityMeta struct {
// 	collection  *mongo.Collection
// 	keyField    string
// 	cachePrefix string
// }

func getEntityMeta(entityType string) (*mongo.Collection, string, string) {
	switch strings.ToLower(entityType) {
	case "place":
		return db.PlacesCollection, "placeid", "place:"
	case "event":
		return db.EventsCollection, "eventid", "event:"
	case "baito":
		return db.BaitoCollection, "baitoid", "baito:"
	case "artist":
		return db.ArtistsCollection, "artistid", "artist:"
	default:
		return nil, "", ""
	}
}

type authMeta struct {
	collection *mongo.Collection
	keyField   string
	ownerField string
}

func getEntityAuthMeta(entityType string) authMeta {
	switch strings.ToLower(entityType) {
	case "place":
		return authMeta{db.PlacesCollection, "placeid", "createdby"}
	case "event":
		return authMeta{db.EventsCollection, "eventid", "creatorid"}
	case "baito":
		return authMeta{db.BaitoCollection, "baitoid", "ownerid"}
	case "artist":
		return authMeta{db.ArtistsCollection, "artistid", "creatorid"}
	default:
		return authMeta{}
	}
}

/**/

func EditBanner(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityTypeStr := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")

	// Convert to EntityType
	entityType := EntityType(strings.ToLower(entityTypeStr))

	// Validate entity type
	validEntity := false
	switch entityType {
	case EntityPlace, EntityEvent, EntityArtist, EntityUser, EntityBaito, EntitySong, EntityPost, EntityChat, EntityFarm, EntityCrop, EntityMedia, EntityFeed, EntityProduct:
		validEntity = true
	}
	if !validEntity {
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

	// Parse multipart form for banner file
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// Use your typed constants for PictureType
	bannerFileName, err := SaveFormFile(r.MultipartForm, "banner", entityType, PicBanner, true)
	if err != nil {
		http.Error(w, fmt.Sprintf("Banner upload failed: %v", err), http.StatusBadRequest)
		return
	}
	if bannerFileName == "" {
		http.Error(w, "No banner file uploaded", http.StatusBadRequest)
		return
	}

	updateFields := bson.M{
		"banner":     bannerFileName,
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
