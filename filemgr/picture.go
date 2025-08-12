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

// type EntityConfig struct {
// 	Collection     *mongo.Collection
// 	IDField        string
// 	CachePrefix    string
// 	OwnerFieldName string
// }

// // Centralized config for all supported entities
// var entityConfigs = map[string]EntityConfig{
// 	"place": {
// 		Collection:     db.PlacesCollection,
// 		IDField:        "placeid",
// 		CachePrefix:    "place:",
// 		OwnerFieldName: "CreatedBy",
// 	},
// 	"event": {
// 		Collection:     db.EventsCollection,
// 		IDField:        "eventid",
// 		CachePrefix:    "event:",
// 		OwnerFieldName: "CreatorID",
// 	},
// 	"baito": {
// 		Collection:     db.BaitoCollection,
// 		IDField:        "baitoid",
// 		CachePrefix:    "baito:",
// 		OwnerFieldName: "OwnerID",
// 	},
// 	"artist": {
// 		Collection:     db.ArtistsCollection,
// 		IDField:        "artistid",
// 		CachePrefix:    "artist:",
// 		OwnerFieldName: "CreatorID",
// 	},
// }

// func updateEntityBannerInDB(w http.ResponseWriter, entityType, entityID string, updateFields bson.M) error {
// 	cfg, ok := entityConfigs[strings.ToLower(entityType)]
// 	if !ok {
// 		http.Error(w, "Unsupported entity type", http.StatusBadRequest)
// 		return fmt.Errorf("unsupported entity type: %s", entityType)
// 	}

// 	// Update DB
// 	filter := bson.M{cfg.IDField: entityID}
// 	if _, err := cfg.Collection.UpdateOne(context.TODO(), filter, bson.M{"$set": updateFields}); err != nil {
// 		http.Error(w, "Error updating "+entityType, http.StatusInternalServerError)
// 		return err
// 	}

// 	// Invalidate cache
// 	if _, err := rdx.RdxDel(cfg.CachePrefix + entityID); err != nil {
// 		log.Printf("Cache deletion failed for %s ID: %s. Error: %v", entityType, entityID, err)
// 	} else {
// 		log.Printf("Cache invalidated for %s ID: %s", entityType, entityID)
// 	}

// 	return nil
// }

// func authorizeUserForEntity(ctx context.Context, entityType, entityID, userID string) error {
// 	cfg, ok := entityConfigs[strings.ToLower(entityType)]
// 	if !ok {
// 		return fmt.Errorf("unsupported entity type")
// 	}

// 	// Fetch entity into a generic map
// 	var entity map[string]interface{}
// 	err := cfg.Collection.FindOne(ctx, bson.M{cfg.IDField: entityID}).Decode(&entity)
// 	if err != nil {
// 		if err == mongo.ErrNoDocuments {
// 			return fmt.Errorf("not found")
// 		}
// 		return fmt.Errorf("database error")
// 	}

// 	// Check owner
// 	if owner, ok := entity[cfg.OwnerFieldName]; !ok || owner != userID {
// 		log.Println("owner", owner, "userid", userID)
// 		return fmt.Errorf("unauthorized")
// 	}

// 	return nil
// }

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

// func updateEntityBannerInDB(w http.ResponseWriter, entityType, entityID string, updateFields bson.M) error {
// 	// Dispatch DB update by entity type
// 	switch strings.ToLower(entityType) {
// 	case "place":
// 		_, err := db.PlacesCollection.UpdateOne(context.TODO(), bson.M{"placeid": entityID}, bson.M{"$set": updateFields})
// 		if err != nil {
// 			http.Error(w, "Error updating place", http.StatusInternalServerError)
// 			return err
// 		}
// 		if _, err := rdx.RdxDel("place:" + entityID); err != nil {
// 			log.Printf("Cache deletion failed for place ID: %s. Error: %v", entityID, err)
// 		} else {
// 			log.Printf("Cache invalidated for place ID: %s", entityID)
// 		}

// 	case "event":
// 		_, err := db.EventsCollection.UpdateOne(context.TODO(), bson.M{"eventid": entityID}, bson.M{"$set": updateFields})
// 		if err != nil {
// 			http.Error(w, "Error updating event", http.StatusInternalServerError)
// 			return err
// 		}
// 		if _, err := rdx.RdxDel("event:" + entityID); err != nil {
// 			log.Printf("Cache deletion failed for event ID: %s. Error: %v", entityID, err)
// 		} else {
// 			log.Printf("Cache invalidated for event ID: %s", entityID)
// 		}

// 	case "baito":
// 		_, err := db.BaitoCollection.UpdateOne(context.TODO(), bson.M{"baitoid": entityID}, bson.M{"$set": updateFields})
// 		if err != nil {
// 			http.Error(w, "Error updating baito", http.StatusInternalServerError)
// 			return err
// 		}
// 		if _, err := rdx.RdxDel("baito:" + entityID); err != nil {
// 			log.Printf("Cache deletion failed for baito ID: %s. Error: %v", entityID, err)
// 		} else {
// 			log.Printf("Cache invalidated for baito ID: %s", entityID)
// 		}

// 	// Add other entity types here (user, venue, etc.)...

// 	default:
// 		http.Error(w, "Unsupported entity type", http.StatusBadRequest)
// 		return fmt.Errorf("unsupported entity type: %s", entityType)
// 	}
// 	return nil
// }

// func authorizeUserForEntity(ctx context.Context, entityType, entityID, userID string) error {
// 	switch strings.ToLower(entityType) {
// 	case "place":
// 		var place models.Place
// 		err := db.PlacesCollection.FindOne(ctx, bson.M{"placeid": entityID}).Decode(&place)
// 		if err != nil {
// 			if err == mongo.ErrNoDocuments {
// 				return fmt.Errorf("not found")
// 			}
// 			return fmt.Errorf("database error")
// 		}
// 		if place.CreatedBy != userID {
// 			return fmt.Errorf("unauthorized")
// 		}

// 	case "event":
// 		var event models.Event
// 		err := db.EventsCollection.FindOne(ctx, bson.M{"eventid": entityID}).Decode(&event)
// 		if err != nil {
// 			if err == mongo.ErrNoDocuments {
// 				return fmt.Errorf("not found")
// 			}
// 			return fmt.Errorf("database error")
// 		}
// 		if event.CreatorID != userID {
// 			return fmt.Errorf("unauthorized")
// 		}

// 	case "baito":
// 		var baito models.Baito
// 		err := db.BaitoCollection.FindOne(ctx, bson.M{"baitoid": entityID}).Decode(&baito)
// 		if err != nil {
// 			if err == mongo.ErrNoDocuments {
// 				return fmt.Errorf("not found")
// 			}
// 			return fmt.Errorf("database error")
// 		}
// 		if baito.OwnerID != userID {
// 			return fmt.Errorf("unauthorized")
// 		}

// 	// Add authorization for other entities here...

// 	default:
// 		return fmt.Errorf("unsupported entity type")
// 	}

// 	return nil
// }

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

// package filemgr

// import (
// 	"context"
// 	"fmt"
// 	"log"
// 	"naevis/db"
// 	"naevis/globals"
// 	"naevis/models"
// 	"naevis/mq"
// 	"naevis/rdx"
// 	"naevis/utils"
// 	"net/http"
// 	"time"

// 	"github.com/julienschmidt/httprouter"
// 	"go.mongodb.org/mongo-driver/bson"
// 	"go.mongodb.org/mongo-driver/mongo"
// )

// // Inserts or updates a place in the database
// func updatePlaceBannerInDB(w http.ResponseWriter, placeID string, updateFields bson.M) error {
// 	_, err := db.PlacesCollection.UpdateOne(context.TODO(), bson.M{"placeid": placeID}, bson.M{"$set": updateFields})
// 	if err != nil {
// 		http.Error(w, "Error updating place", http.StatusInternalServerError)
// 		return err
// 	}

// 	// Invalidate cache
// 	if _, err := rdx.RdxDel("place:" + placeID); err != nil {
// 		log.Printf("Cache deletion failed for place ID: %s. Error: %v", placeID, err)
// 	} else {
// 		log.Printf("Cache successfully invalidated for place ID: %s", placeID)
// 	}

// 	return nil
// }

// func EditBanner(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	placeID := ps.ByName("placeid")

// 	// Validate user
// 	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
// 	if !ok {
// 		http.Error(w, "Invalid user", http.StatusUnauthorized)
// 		return
// 	}

// 	// Fetch existing place
// 	var place models.Place
// 	err := db.PlacesCollection.FindOne(context.TODO(), bson.M{"placeid": placeID}).Decode(&place)
// 	if err != nil {
// 		if err == mongo.ErrNoDocuments {
// 			http.Error(w, "Place not found", http.StatusNotFound)
// 		} else {
// 			http.Error(w, "Database error", http.StatusInternalServerError)
// 		}
// 		return
// 	}

// 	// Authorization
// 	if place.CreatedBy != requestingUserID {
// 		http.Error(w, "You are not authorized to edit this place", http.StatusForbidden)
// 		return
// 	}

// 	// Parse multipart form for banner file
// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		http.Error(w, "Unable to parse form", http.StatusBadRequest)
// 		return
// 	}

// 	// Save banner file from form
// 	banner, err := SaveFormFile(r.MultipartForm, "banner", EntityPlace, PicBanner, false)
// 	if err != nil {
// 		http.Error(w, fmt.Sprintf("Banner upload failed: %v", err), http.StatusBadRequest)
// 		return
// 	}
// 	if banner == "" {
// 		http.Error(w, "No banner file uploaded", http.StatusBadRequest)
// 		return
// 	}

// 	updateFields := bson.M{
// 		"banner":     banner,
// 		"updated_at": time.Now(),
// 	}

// 	if err := updatePlaceBannerInDB(w, placeID, updateFields); err != nil {
// 		return
// 	}

// 	go mq.Emit(r.Context(), "place-edited", models.Index{EntityType: "place", EntityId: placeID, Method: "PUT"})

// 	utils.RespondWithJSON(w, http.StatusOK, updateFields)
// }
