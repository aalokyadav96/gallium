package places

import (
	"context"
	"fmt"
	"log"
	"naevis/db"
	"naevis/filemgr"
	"naevis/globals"
	"naevis/models"
	"naevis/mq"
	"naevis/rdx"
	"naevis/utils"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Inserts or updates a place in the database
func updatePlaceBannerInDB(w http.ResponseWriter, placeID string, updateFields bson.M) error {
	_, err := db.PlacesCollection.UpdateOne(context.TODO(), bson.M{"placeid": placeID}, bson.M{"$set": updateFields})
	if err != nil {
		http.Error(w, "Error updating place", http.StatusInternalServerError)
		return err
	}

	// Invalidate cache
	if _, err := rdx.RdxDel("place:" + placeID); err != nil {
		log.Printf("Cache deletion failed for place ID: %s. Error: %v", placeID, err)
	} else {
		log.Printf("Cache successfully invalidated for place ID: %s", placeID)
	}

	return nil
}

func EditPlaceBanner(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	placeID := ps.ByName("placeid")

	// Validate user
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusUnauthorized)
		return
	}

	// Fetch existing place
	var place models.Place
	err := db.PlacesCollection.FindOne(context.TODO(), bson.M{"placeid": placeID}).Decode(&place)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Place not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	// Authorization
	if place.CreatedBy != requestingUserID {
		http.Error(w, "You are not authorized to edit this place", http.StatusForbidden)
		return
	}

	// Parse multipart form for banner file
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// Save banner file from form
	banner, err := filemgr.SaveFormFile(r.MultipartForm, "banner", filemgr.EntityPlace, filemgr.PicBanner, false)
	if err != nil {
		http.Error(w, fmt.Sprintf("Banner upload failed: %v", err), http.StatusBadRequest)
		return
	}
	if banner == "" {
		http.Error(w, "No banner file uploaded", http.StatusBadRequest)
		return
	}

	updateFields := bson.M{
		"banner":     banner,
		"updated_at": time.Now(),
	}

	if err := updatePlaceBannerInDB(w, placeID, updateFields); err != nil {
		return
	}

	go mq.Emit(r.Context(), "place-edited", models.Index{EntityType: "place", EntityId: placeID, Method: "PUT"})

	utils.RespondWithJSON(w, http.StatusOK, updateFields)
}
