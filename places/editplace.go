package places

import (
	"context"
	"encoding/json"
	"log"
	"naevis/db"
	"naevis/globals"
	"naevis/models"
	"naevis/mq"
	"naevis/rdx"
	"naevis/userdata"
	"naevis/utils"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Inserts or updates a place in the database
func updatePlaceInDB(w http.ResponseWriter, placeID string, updateFields bson.M) error {
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

// EditPlace endpoint
func EditPlace(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	placeID := ps.ByName("placeid")

	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusUnauthorized)
		return
	}

	// Authorization check
	var existing models.Place
	err := db.PlacesCollection.FindOne(context.TODO(), bson.M{"placeid": placeID}).Decode(&existing)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Place not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}
	if existing.CreatedBy != requestingUserID {
		http.Error(w, "You are not authorized to edit this place", http.StatusForbidden)
		return
	}

	_, updateFields, err := parseAndBuildPlace(r, "update")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(updateFields) == 0 {
		http.Error(w, "No fields to update", http.StatusBadRequest)
		return
	}

	if err := updatePlaceInDB(w, placeID, updateFields); err != nil {
		return
	}

	go mq.Emit(ctx, "place-edited", models.Index{EntityType: "place", EntityId: placeID, Method: "PUT"})

	utils.RespondWithJSON(w, http.StatusOK, updateFields)
}

// func EditPlace(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	ctx := r.Context()
// 	placeID := ps.ByName("placeid")

func DeletePlace(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	placeID := ps.ByName("placeid")
	var place models.Place

	// Get the ID of the requesting user from the context
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}
	// log.Println("Requesting User ID:", requestingUserID)

	// Get the place from the database using placeID
	err := db.PlacesCollection.FindOne(context.TODO(), bson.M{"placeid": placeID}).Decode(&place)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Place not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Check if the place was created by the requesting user
	if place.CreatedBy != requestingUserID {
		http.Error(w, "You are not authorized to delete this place", http.StatusForbidden)
		return
	}

	// Delete the place from MongoDB
	_, err = db.PlacesCollection.DeleteOne(context.TODO(), bson.M{"placeid": placeID})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rdx.RdxDel("place:" + placeID) // Invalidate the cache for the deleted place

	userdata.DelUserData("place", placeID, requestingUserID)

	m := models.Index{EntityType: "place", EntityId: placeID, Method: "DELETE"}
	go mq.Emit(ctx, "place-deleted", m)

	// Respond with success
	w.WriteHeader(http.StatusOK)
	response := map[string]any{
		"status":  http.StatusNoContent,
		"message": "Place deleted successfully",
	}
	json.NewEncoder(w).Encode(response)
}
