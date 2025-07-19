package places

import (
	"context"
	"encoding/json"
	"naevis/db"
	"naevis/structs"
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

// func GetEvents(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	w.Header().Set("Content-Type", "application/json")

// 	// Parse pagination query parameters
// 	pageStr := r.URL.Query().Get("page")
// 	limitStr := r.URL.Query().Get("limit")
// 	placeID := ps.ByName("placeid")

// 	// Validate PlaceID
// 	if placeID == "" {
// 		http.Error(w, "missing required query parameter: placeid", http.StatusBadRequest)
// 		return
// 	}

// 	page := 1
// 	limit := 10

// 	if pageStr != "" {
// 		if parsedPage, err := strconv.Atoi(pageStr); err == nil && parsedPage > 0 {
// 			page = parsedPage
// 		}
// 	}

// 	if limitStr != "" {
// 		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
// 			limit = parsedLimit
// 		}
// 	}

// 	skip := (page - 1) * limit
// 	int64Limit := int64(limit)
// 	int64Skip := int64(skip)

// 	// Time now to filter upcoming events
// 	now := time.Now()

// 	// Filter: events matching placeid and in the future
// 	filter := bson.M{
// 		"placeid": placeID,
// 		"date":    bson.M{"$gte": now},
// 	}

// 	sortOrder := bson.D{{Key: "date", Value: 1}} // soonest upcoming first

// 	cursor, err := db.EventsCollection.Find(context.TODO(), filter, &options.FindOptions{
// 		Skip:  &int64Skip,
// 		Limit: &int64Limit,
// 		Sort:  sortOrder,
// 	})
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(context.TODO())

// 	var events []structs.Event
// 	if err := cursor.All(context.TODO(), &events); err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 		if err := json.NewEncoder(w).Encode(events); err != nil {
// 			http.Error(w, err.Error(), http.StatusInternalServerError)
// 			return
// 		}
// 	}

// func GetEvents(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	w.Header().Set("Content-Type", "application/json")

// 	// Query parameters
// 	pageStr := r.URL.Query().Get("page")
// 	limitStr := r.URL.Query().Get("limit")
// 	placeID := ps.ByName("placeid")

// 	if placeID == "" {
// 		http.Error(w, "missing required path parameter: placeid", http.StatusBadRequest)
// 		return
// 	}

// 	// Pagination defaults
// 	page := 1
// 	limit := 10

// 	if parsedPage, err := strconv.Atoi(pageStr); err == nil && parsedPage > 0 {
// 		page = parsedPage
// 	}
// 	if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
// 		limit = parsedLimit
// 	}

// 	skip := (page - 1) * limit
// 	int64Limit := int64(limit)
// 	int64Skip := int64(skip)
// 	now := time.Now()

// 	// Filter for upcoming events by placeid
// 	filter := bson.M{
// 		"placeid":         placeID,
// 		"start_date_time": bson.M{"$gte": now},
// 	}

// 	// Only include necessary fields
// 	projection := bson.M{
// 		"eventid":         1,
// 		"title":           1,
// 		"description":     1,
// 		"start_date_time": 1,
// 		"end_date_time":   1,
// 		"placename":       1,
// 		"banner_image":    1,
// 		"category":        1,
// 	}

// 	// Pagination + sort options
// 	findOptions := options.Find().
// 		SetSkip(int64Skip).
// 		SetLimit(int64Limit).
// 		SetSort(bson.D{{Key: "start_date_time", Value: 1}}).
// 		SetProjection(projection)

// 	// Get total count of matching events
// 	totalCount, err := db.EventsCollection.CountDocuments(context.TODO(), filter)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	// Fetch paginated event data
// 	cursor, err := db.EventsCollection.Find(context.TODO(), filter, findOptions)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(context.TODO())

// 	var events []bson.M
// 	if err := cursor.All(context.TODO(), &events); err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	// Final response with pagination metadata
// 	response := map[string]any{
// 		"events": events,
// 		"total":  totalCount,
// 		"page":   page,
// 		"limit":  limit,
// 	}

// 	if err := json.NewEncoder(w).Encode(response); err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// }

// // func GetEvents(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// // 	w.Header().Set("Content-Type", "application/json")

// // 	// Query parameters
// // 	pageStr := r.URL.Query().Get("page")
// // 	limitStr := r.URL.Query().Get("limit")
// // 	placeID := ps.ByName("placeid")

// // 	if placeID == "" {
// // 		http.Error(w, "missing required query parameter: placeid", http.StatusBadRequest)
// // 		return
// // 	}

// // 	page := 1
// // 	limit := 10

// // 	if parsedPage, err := strconv.Atoi(pageStr); err == nil && parsedPage > 0 {
// // 		page = parsedPage
// // 	}
// // 	if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
// // 		limit = parsedLimit
// // 	}

// // 	skip := (page - 1) * limit
// // 	int64Limit := int64(limit)
// // 	int64Skip := int64(skip)
// // 	now := time.Now()

// // 	filter := bson.M{
// // 		"placeid":         placeID,
// // 		"start_date_time": bson.M{"$gte": now},
// // 	}

// // 	// Only project necessary fields
// // 	projection := bson.M{
// // 		"eventid":         1,
// // 		"title":           1,
// // 		"description":     1,
// // 		"start_date_time": 1,
// // 		"end_date_time":   1,
// // 		"placename":       1,
// // 		"banner_image":    1,
// // 		"category":        1,
// // 	}

// // 	findOptions := options.Find().
// // 		SetSkip(int64Skip).
// // 		SetLimit(int64Limit).
// // 		SetSort(bson.D{{Key: "start_date_time", Value: 1}}).
// // 		SetProjection(projection)
// // totalCount, err := db.EventsCollection.CountDocuments(context.TODO(), filter)
// // if err != nil {
// // 	http.Error(w, err.Error(), http.StatusInternalServerError)
// // 	return
// // }

// // // Wrap data in object with pagination info
// // response := map[string]any{
// // 	"events": events,
// // 	"total":  totalCount,
// // 	"page":   page,
// // 	"limit":  limit,
// // }

// // if err := json.NewEncoder(w).Encode(response); err != nil {
// // 	http.Error(w, err.Error(), http.StatusInternalServerError)
// // 	return
// // }

// // 	cursor, err := db.EventsCollection.Find(context.TODO(), filter, findOptions)
// // 	if err != nil {
// // 		http.Error(w, err.Error(), http.StatusInternalServerError)
// // 		return
// // 	}
// // 	defer cursor.Close(context.TODO())

// // 	var events []bson.M

// // 	if err := cursor.All(context.TODO(), &events); err != nil {
// // 		http.Error(w, err.Error(), http.StatusInternalServerError)
// // 		return
// // 	}

// // 	if err := json.NewEncoder(w).Encode(events); err != nil {
// // 		http.Error(w, err.Error(), http.StatusInternalServerError)
// // 		return
// // 	}
// // }
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
	var events []structs.Event
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
