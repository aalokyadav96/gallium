package itinerary

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"naevis/db"
	"naevis/middleware"
	"naevis/models"
	"naevis/utils"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// Utility function to extract user ID from JWT
func GetRequestingUserID(w http.ResponseWriter, r *http.Request) (string, error) {
	tokenString := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(tokenString)
	if err != nil {
		log.Printf("JWT validation error: %v", err)
		return "", err
	}
	return claims.UserID, nil
}

// POST /api/itineraries
func CreateItinerary(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var itinerary models.Itinerary
	if err := json.NewDecoder(r.Body).Decode(&itinerary); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	userID, err := GetRequestingUserID(w, r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	itinerary.UserID = userID
	itinerary.ItineraryID = utils.GenerateRandomString(13)
	if itinerary.Status == "" {
		itinerary.Status = "Draft"
	}

	// Ensure Days and Visits arrays are never nil
	if itinerary.Days == nil {
		itinerary.Days = []models.Day{}
	} else {
		for i := range itinerary.Days {
			if itinerary.Days[i].Visits == nil {
				itinerary.Days[i].Visits = []models.Visit{}
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := db.ItineraryCollection.InsertOne(ctx, itinerary)
	if err != nil {
		http.Error(w, "Error inserting itinerary", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

// GET /api/itineraries/all/:id
func GetItinerary(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	itineraryID := ps.ByName("id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"itineraryid": itineraryID, "deleted": bson.M{"$ne": true}}

	var itinerary models.Itinerary
	err := db.ItineraryCollection.FindOne(ctx, filter).Decode(&itinerary)
	if err != nil {
		http.Error(w, "Itinerary not found", http.StatusNotFound)
		return
	}

	// Ensure arrays are not nil
	if itinerary.Days == nil {
		itinerary.Days = []models.Day{}
	} else {
		for i := range itinerary.Days {
			if itinerary.Days[i].Visits == nil {
				itinerary.Days[i].Visits = []models.Visit{}
			}
		}
	}

	utils.RespondWithJSON(w, http.StatusOK, itinerary)
}

// PUT /api/itineraries/:id
func UpdateItinerary(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userID, err := GetRequestingUserID(w, r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	itineraryID := ps.ByName("id")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var existing models.Itinerary
	if err := db.ItineraryCollection.FindOne(ctx, bson.M{"itineraryid": itineraryID}).Decode(&existing); err != nil {
		http.Error(w, "Itinerary not found", http.StatusNotFound)
		return
	}

	if existing.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var updated models.Itinerary
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Ensure arrays are not nil
	if updated.Days == nil {
		updated.Days = []models.Day{}
	} else {
		for i := range updated.Days {
			if updated.Days[i].Visits == nil {
				updated.Days[i].Visits = []models.Visit{}
			}
		}
	}

	update := bson.M{"$set": bson.M{
		"name":        updated.Name,
		"description": updated.Description,
		"start_date":  updated.StartDate,
		"end_date":    updated.EndDate,
		"status":      updated.Status,
		"published":   updated.Published,
		"days":        updated.Days,
	}}

	if _, err := db.ItineraryCollection.UpdateOne(ctx, bson.M{"itineraryid": itineraryID}, update); err != nil {
		http.Error(w, "Error updating itinerary", http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, bson.M{"message": "Itinerary updated successfully"})
}

// DELETE /api/itineraries/:id
func DeleteItinerary(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userID, err := GetRequestingUserID(w, r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	itineraryID := ps.ByName("id")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var itinerary models.Itinerary
	if err := db.ItineraryCollection.FindOne(ctx, bson.M{"itineraryid": itineraryID}).Decode(&itinerary); err != nil {
		http.Error(w, "Itinerary not found", http.StatusNotFound)
		return
	}

	if itinerary.UserID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	_, err = db.ItineraryCollection.UpdateOne(ctx, bson.M{"itineraryid": itineraryID}, bson.M{"$set": bson.M{"deleted": true}})
	if err != nil {
		http.Error(w, "Error deleting itinerary", http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, bson.M{"message": "Itinerary deleted successfully"})
}

// POST /api/itineraries/:id/fork
func ForkItinerary(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userID, err := GetRequestingUserID(w, r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	originalID := ps.ByName("id")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var original models.Itinerary
	if err := db.ItineraryCollection.FindOne(ctx, bson.M{"itineraryid": originalID}).Decode(&original); err != nil {
		http.Error(w, "Original itinerary not found", http.StatusNotFound)
		return
	}

	if original.Days == nil {
		original.Days = []models.Day{}
	} else {
		for i := range original.Days {
			if original.Days[i].Visits == nil {
				original.Days[i].Visits = []models.Visit{}
			}
		}
	}

	newItinerary := models.Itinerary{
		ItineraryID: utils.GenerateRandomString(13),
		UserID:      userID,
		Name:        "Forked - " + original.Name,
		Description: original.Description,
		StartDate:   original.StartDate,
		EndDate:     original.EndDate,
		Days:        original.Days,
		Status:      "Draft",
		Published:   false,
		ForkedFrom:  &originalID,
	}

	result, err := db.ItineraryCollection.InsertOne(ctx, newItinerary)
	if err != nil {
		http.Error(w, "Error forking itinerary", http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, http.StatusCreated, result)
}

// PUT /api/itineraries/:id/publish
func PublishItinerary(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userID, err := GetRequestingUserID(w, r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	id := ps.ByName("id")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"itineraryid": id, "user_id": userID}
	update := bson.M{"$set": bson.M{"published": true}}

	result, err := db.ItineraryCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		http.Error(w, "Error publishing itinerary", http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, result)
}
