package itinerary

import (
	"context"
	"naevis/db"
	"naevis/models"
	"naevis/utils"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// GET /api/itineraries
func GetItineraries(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	filter := bson.M{"deleted": bson.M{"$ne": true}}
	itineraries, err := utils.FindAndDecode[models.Itinerary](ctx, db.ItineraryCollection, filter)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Error fetching itineraries")
		return
	}

	for i := range itineraries {
		if itineraries[i].Days == nil {
			itineraries[i].Days = []models.Day{}
		} else {
			for j := range itineraries[i].Days {
				if itineraries[i].Days[j].Visits == nil {
					itineraries[i].Days[j].Visits = []models.Visit{}
				}
			}
		}
	}

	if itineraries == nil {
		itineraries = []models.Itinerary{}
	}

	utils.RespondWithJSON(w, http.StatusOK, itineraries)
}

// GET /api/itineraries/search
func SearchItineraries(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	query := r.URL.Query()

	filter := bson.M{"deleted": bson.M{"$ne": true}}
	if start := query.Get("start_date"); start != "" {
		filter["start_date"] = start
	}
	if location := query.Get("location"); location != "" {
		filter["days.visits.location"] = bson.M{"$in": []string{location}}
	}
	if status := query.Get("status"); status != "" {
		filter["status"] = status
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := db.ItineraryCollection.Find(ctx, filter)
	if err != nil {
		http.Error(w, "Error fetching itineraries", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var itineraries []models.Itinerary
	for cursor.Next(ctx) {
		var itinerary models.Itinerary
		if err := cursor.Decode(&itinerary); err == nil {
			if itinerary.Days == nil {
				itinerary.Days = []models.Day{}
			} else {
				for i := range itinerary.Days {
					if itinerary.Days[i].Visits == nil {
						itinerary.Days[i].Visits = []models.Visit{}
					}
				}
			}
			itineraries = append(itineraries, itinerary)
		}
	}

	if itineraries == nil {
		itineraries = []models.Itinerary{}
	}

	utils.RespondWithJSON(w, http.StatusOK, itineraries)
}

// // Itineraries
// func GetItineraries(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
// 	defer cancel()

// 	filter := bson.M{"deleted": bson.M{"$ne": true}}
// 	itineraries, err := utils.FindAndDecode[models.Itinerary](ctx, db.ItineraryCollection, filter)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Error fetching itineraries")
// 		return
// 	}

// 	utils.RespondWithJSON(w, http.StatusOK, itineraries)
// }

// // GET /api/itineraries/search
// func SearchItineraries(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	query := r.URL.Query()

// 	filter := bson.M{"deleted": bson.M{"$ne": true}}
// 	if start := query.Get("start_date"); start != "" {
// 		filter["start_date"] = start
// 	}
// 	if location := query.Get("location"); location != "" {
// 		// filter["locations"] = bson.M{"$in": []string{location}}
// 		filter["days.visits.location"] = bson.M{"$in": []string{location}}
// 	}
// 	if status := query.Get("status"); status != "" {
// 		filter["status"] = status
// 	}

// 	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
// 	defer cancel()

// 	cursor, err := db.ItineraryCollection.Find(ctx, filter)
// 	if err != nil {
// 		http.Error(w, "Error fetching itineraries", http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(ctx)

// 	var itineraries []models.Itinerary
// 	for cursor.Next(ctx) {
// 		var itinerary models.Itinerary
// 		if err := cursor.Decode(&itinerary); err == nil {
// 			itineraries = append(itineraries, itinerary)
// 		}
// 	}

// 	if itineraries == nil {
// 		itineraries = []models.Itinerary{}
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(itineraries)
// }
