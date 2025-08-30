package places

import (
	"context"
	"encoding/json"
	"naevis/db"
	"naevis/models"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 🏟️ Events
func GetEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("GetEvent not implemented yet"))
}

func PostEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("PostEvent not implemented yet"))
}

func PutEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("PutEvent not implemented yet"))
}

func DeleteEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("DeleteEvent not implemented yet"))
}

func PostViewEventDetails(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("PostViewEventDetails not implemented yet"))
}

func GetEvents(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")

	// Query parameters
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	placeID := ps.ByName("placeid")

	if placeID == "" {
		http.Error(w, "missing required path parameter: placeid", http.StatusBadRequest)
		return
	}

	// Default pagination values
	page := 1
	limit := 10

	if parsedPage, err := strconv.Atoi(pageStr); err == nil && parsedPage > 0 {
		page = parsedPage
	}
	if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
		limit = parsedLimit
	}

	skip := (page - 1) * limit
	int64Limit := int64(limit)
	int64Skip := int64(skip)
	now := time.Now()

	// Filter: only upcoming events for this place
	filter := bson.M{
		"placeid": placeID,
		"date":    bson.M{"$gte": now},
	}

	// Project only required fields
	projection := bson.M{
		"eventid":         1,
		"title":           1,
		"description":     1,
		"start_date_time": 1,
		"end_date_time":   1,
		"placename":       1,
		"banner_image":    1,
		"category":        1,
	}

	findOptions := options.Find().
		SetSkip(int64Skip).
		SetLimit(int64Limit).
		SetSort(bson.D{{Key: "date", Value: 1}}).
		SetProjection(projection)

	// Count total events for pagination
	totalCount, err := db.EventsCollection.CountDocuments(context.TODO(), filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch paginated events
	cursor, err := db.EventsCollection.Find(context.TODO(), filter, findOptions)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	// var events []bson.M
	var events []models.Event
	if err := cursor.All(context.TODO(), &events); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Wrap in structured response
	response := map[string]any{
		"events": events,
		"total":  totalCount,
		"page":   page,
		"limit":  limit,
	}

	// Send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
