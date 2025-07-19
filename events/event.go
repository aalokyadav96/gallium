package events

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"naevis/db"
	"naevis/structs"
	"naevis/utils"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetEventsCount returns the total count of published events.
func GetEventsCount(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Example static count; replace with a real DB query if needed.
	count := 3
	utils.SendJSONResponse(w, http.StatusOK, count)
}

// GetEvents returns a paginated list of published events, sorted newest first.
func GetEvents(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// use request context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	page := 1
	limit := 10
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}
	skip := int64((page - 1) * limit)
	limit64 := int64(limit)

	// filter := bson.M{"published": "true"}
	filter := bson.M{}

	totalCount, err := db.EventsCollection.CountDocuments(ctx, filter)
	if err != nil {
		log.Println("CountDocuments error:", err)
		http.Error(w, "Failed to fetch event count", http.StatusInternalServerError)
		return
	}

	opts := options.Find().
		SetSkip(skip).
		SetLimit(limit64).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := db.EventsCollection.Find(ctx, filter, opts)
	if err != nil {
		log.Println("Find error:", err)
		http.Error(w, "Failed to fetch events", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var rawEvents []structs.Event
	if err := cursor.All(ctx, &rawEvents); err != nil {
		log.Println("Cursor.All error:", err)
		http.Error(w, "Failed to parse events", http.StatusInternalServerError)
		return
	}

	// sanitize and enrich
	safeEvents := make([]structs.Event, 0, len(rawEvents))
	for _, e := range rawEvents {
		safeEvents = append(safeEvents, toSafeEvent(e))
	}

	resp := struct {
		Events     []structs.Event `json:"events"`
		EventCount int64           `json:"eventCount"`
		Page       int             `json:"page"`
		Limit      int             `json:"limit"`
	}{
		Events:     safeEvents,
		EventCount: totalCount,
		Page:       page,
		Limit:      limit,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GetEvent returns one published event by its eventid, with tickets/media/merch lookups.
func GetEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	id := ps.ByName("eventid")
	// filter := bson.M{"eventid": id, "published": "true"}
	filter := bson.M{"eventid": id}

	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: filter}},
		bson.D{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "ticks"},
			{Key: "localField", Value: "eventid"},
			{Key: "foreignField", Value: "eventid"},
			{Key: "as", Value: "tickets"},
		}}},
		bson.D{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "media"},
			{Key: "let", Value: bson.D{{Key: "eid", Value: "$eventid"}}},
			{Key: "pipeline", Value: mongo.Pipeline{
				bson.D{{Key: "$match", Value: bson.D{
					{Key: "$expr", Value: bson.D{
						{Key: "$and", Value: bson.A{
							bson.D{{Key: "$eq", Value: bson.A{"$entityid", "$$eid"}}},
							bson.D{{Key: "$eq", Value: bson.A{"$entitytype", "event"}}},
						}},
					}},
				}}},
			}},
			{Key: "as", Value: "media"},
		}}},
		bson.D{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "merch"},
			{Key: "let", Value: bson.D{{Key: "eid", Value: "$eventid"}}},
			{Key: "pipeline", Value: mongo.Pipeline{
				bson.D{{Key: "$match", Value: bson.D{
					{Key: "$expr", Value: bson.D{
						{Key: "$and", Value: bson.A{
							bson.D{{Key: "$eq", Value: bson.A{"$entity_id", "$$eid"}}},
							bson.D{{Key: "$eq", Value: bson.A{"$entity_type", "event"}}},
						}},
					}},
				}}},
			}},
			{Key: "as", Value: "merch"},
		}}},
	}

	cur, err := db.EventsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		log.Println("Aggregate error:", err)
		http.Error(w, "Failed to fetch event", http.StatusInternalServerError)
		return
	}
	defer cur.Close(ctx)

	if !cur.Next(ctx) {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	var rawEvent structs.Event
	if err := cur.Decode(&rawEvent); err != nil {
		log.Println("Decode error:", err)
		http.Error(w, "Failed to decode event", http.StatusInternalServerError)
		return
	}

	safe := toSafeEvent(rawEvent)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(safe)
}

// toSafeEvent ensures no nil slices or zero-values, computes Prices & Currency.
func toSafeEvent(e structs.Event) structs.Event {
	// default empty slices
	if e.Tickets == nil {
		e.Tickets = []structs.Ticket{}
	}
	if e.Merch == nil {
		e.Merch = []structs.Merch{}
	}
	if e.FAQs == nil {
		e.FAQs = []structs.FAQ{}
	}
	if e.Artists == nil {
		e.Artists = []string{}
	}
	if e.Tags == nil {
		e.Tags = []string{}
	}

	// sanitize zero dates
	if !e.Date.IsZero() {
		e.Date = e.Date.UTC()
	}
	if !e.CreatedAt.IsZero() {
		e.CreatedAt = e.CreatedAt.UTC()
	}
	if !e.UpdatedAt.IsZero() {
		e.UpdatedAt = e.UpdatedAt.UTC()
	}

	// compute prices & currency
	var prices []float64
	var currency string
	if len(e.Tickets) > 0 {
		for _, t := range e.Tickets {
			prices = append(prices, t.Price)
			if currency == "" && t.Currency != "" {
				currency = t.Currency
			}
		}
	} else {
		prices = []float64{0}
	}
	e.Prices = prices
	e.Currency = currency

	return e
}

func AddFAQs(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	if eventID == "" {
		log.Println("Missing event ID in request")
		http.Error(w, "Missing event ID", http.StatusBadRequest)
		return
	}

	// Parse the request body
	var newFAQ structs.FAQ
	err := json.NewDecoder(r.Body).Decode(&newFAQ)
	if err != nil {
		log.Printf("Invalid request payload: %v", err)
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Validate the input
	if newFAQ.Title == "" || newFAQ.Content == "" {
		http.Error(w, "Title and content are required", http.StatusBadRequest)
		return
	}

	// Append the new FAQ to the FAQs array in MongoDB
	result, err := db.EventsCollection.UpdateOne(
		context.TODO(),
		bson.M{"eventid": eventID},
		bson.M{"$push": bson.M{"faqs": newFAQ}}, // Use $push to append to the array
	)
	if err != nil {
		log.Printf("Error updating event %s: %v", eventID, err)
		http.Error(w, "Error updating event", http.StatusInternalServerError)
		return
	}

	if result.MatchedCount == 0 {
		log.Printf("Event with ID %s not found", eventID)
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	// Respond with success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "FAQ added successfully",
	})
}

// package events

// import (
// 	"context"
// 	"encoding/json"
// 	"log"
// 	"naevis/db"
// 	"naevis/structs"
// 	"naevis/utils"
// 	"net/http"
// 	"strconv"

// 	"github.com/julienschmidt/httprouter"
// 	"go.mongodb.org/mongo-driver/bson"
// 	"go.mongodb.org/mongo-driver/mongo"
// 	"go.mongodb.org/mongo-driver/mongo/options"
// )

// func GetEventsCount(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	var ciount int = 3
// 	utils.SendJSONResponse(w, http.StatusOK, ciount)
// }

// func GetEvents(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	w.Header().Set("Content-Type", "application/json")

// 	pageStr := r.URL.Query().Get("page")
// 	limitStr := r.URL.Query().Get("limit")

// 	page := 1
// 	limit := 10
// 	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
// 		page = p
// 	}
// 	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
// 		limit = l
// 	}
// 	skip := (page - 1) * limit
// 	int64Limit := int64(limit)
// 	int64Skip := int64(skip)

// 	filter := bson.M{"published": "true"}

// 	totalCount, err := db.EventsCollection.CountDocuments(context.TODO(), filter)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	sortOrder := bson.D{{Key: "created_at", Value: -1}}

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

// 	for i := range events {
// 		sanitizeEventDefaults(&events[i])

// 		var prices []float64
// 		var currency string

// 		if len(events[i].Tickets) > 0 {
// 			for _, ticket := range events[i].Tickets {
// 				prices = append(prices, ticket.Price)
// 				if ticket.Currency != "" && currency == "" {
// 					currency = ticket.Currency
// 				}
// 			}
// 		} else {
// 			prices = []float64{0}
// 			currency = ""
// 		}

// 		events[i].Prices = prices     // make sure these exist in your struct
// 		events[i].Currency = currency // make sure these exist in your struct
// 	}

// 	resp := struct {
// 		Events     []structs.Event `json:"events"`
// 		EventCount int64           `json:"eventCount"`
// 		Page       int             `json:"page"`
// 		Limit      int             `json:"limit"`
// 	}{
// 		Events:     events,
// 		EventCount: totalCount,
// 		Page:       page,
// 		Limit:      limit,
// 	}

// 	if err := json.NewEncoder(w).Encode(resp); err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 	}
// }

// func sanitizeEventDefaults(e *structs.Event) {
// 	if e.Tickets == nil {
// 		e.Tickets = []structs.Ticket{}
// 	}
// 	if e.Merch == nil {
// 		e.Merch = []structs.Merch{}
// 	}
// 	if e.FAQs == nil {
// 		e.FAQs = []structs.FAQ{}
// 	}
// 	if e.Artists == nil {
// 		e.Artists = []string{}
// 	}
// 	if e.Tags == nil {
// 		e.Tags = []string{}
// 	}
// 	if e.OrganizerName == "" {
// 		e.OrganizerName = ""
// 	}
// 	if e.OrganizerContact == "" {
// 		e.OrganizerContact = ""
// 	}
// 	if e.SeatingPlanImage == "" {
// 		e.SeatingPlanImage = ""
// 	}
// 	if e.WebsiteURL == "" {
// 		e.WebsiteURL = ""
// 	}
// 	// if e.AccessibilityInfo == "" {
// 	// 	e.AccessibilityInfo = ""
// 	// }
// 	// if e.CustomFields == nil {
// 	// 	e.CustomFields = nil
// 	// }
// 	// if e.SocialLinks == nil {
// 	// 	e.SocialLinks = nil
// 	// }

// 	// Ensure dates are in UTC
// 	e.Date = e.Date.UTC()
// 	e.CreatedAt = e.CreatedAt.UTC()
// 	e.UpdatedAt = e.UpdatedAt.UTC()
// }

// func GetEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	id := ps.ByName("eventid")

// 	// Aggregation pipeline to fetch event along with related tickets, media, and merch
// 	pipeline := mongo.Pipeline{
// 		bson.D{{Key: "$match", Value: bson.D{{Key: "eventid", Value: id}}}},
// 		bson.D{{Key: "$lookup", Value: bson.D{
// 			{Key: "from", Value: "ticks"},
// 			{Key: "localField", Value: "eventid"},
// 			{Key: "foreignField", Value: "eventid"},
// 			{Key: "as", Value: "tickets"},
// 		}}},

// 		bson.D{{Key: "$lookup", Value: bson.D{
// 			{Key: "from", Value: "media"},
// 			{Key: "let", Value: bson.D{
// 				{Key: "event_id", Value: "$eventid"},
// 			}},
// 			{Key: "pipeline", Value: mongo.Pipeline{
// 				bson.D{{Key: "$match", Value: bson.D{
// 					{Key: "$expr", Value: bson.D{
// 						{Key: "$and", Value: bson.A{
// 							bson.D{{Key: "$eq", Value: bson.A{"$entityid", "$$event_id"}}},
// 							bson.D{{Key: "$eq", Value: bson.A{"$entitytype", "event"}}},
// 						}},
// 					}},
// 				}}},
// 				bson.D{{Key: "$limit", Value: 10}},
// 				bson.D{{Key: "$skip", Value: 0}},
// 			}},
// 			{Key: "as", Value: "media"},
// 		}}},

// 		bson.D{{Key: "$lookup", Value: bson.D{
// 			{Key: "from", Value: "merch"},
// 			{Key: "let", Value: bson.D{
// 				{Key: "event_id", Value: "$eventid"},
// 			}},
// 			{Key: "pipeline", Value: mongo.Pipeline{
// 				bson.D{{Key: "$match", Value: bson.D{
// 					{Key: "$expr", Value: bson.D{
// 						{Key: "$and", Value: bson.A{
// 							bson.D{{Key: "$eq", Value: bson.A{"$entity_id", "$$event_id"}}},
// 							bson.D{{Key: "$eq", Value: bson.A{"$entity_type", "event"}}},
// 						}},
// 					}},
// 				}}},
// 			}},
// 			{Key: "as", Value: "merch"},
// 		}}},
// 	}

// 	// Execute the aggregation query
// 	// db.EventsCollection := client.Database("eventdb").Collection("events")
// 	cursor, err := db.EventsCollection.Aggregate(context.TODO(), pipeline)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(context.TODO())

// 	var event structs.Event
// 	if cursor.Next(context.TODO()) {
// 		if err := cursor.Decode(&event); err != nil {
// 			http.Error(w, "Failed to decode event data", http.StatusInternalServerError)
// 			return
// 		}
// 	} else {
// 		http.Error(w, "Event not found", http.StatusNotFound)
// 		return
// 	}

// 	// Encode the event as JSON and write to response
// 	w.Header().Set("Content-Type", "application/json")
// 	if err := json.NewEncoder(w).Encode(event); err != nil {
// 		http.Error(w, "Failed to encode event data", http.StatusInternalServerError)
// 	}
// }

// // // FAQ represents a single FAQ structure
// // type FAQ struct {
// // 	Title   string `json:"title"`
// // 	Content string `json:"content"`
// // }

// func AddFAQs(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	eventID := ps.ByName("eventid")
// 	if eventID == "" {
// 		log.Println("Missing event ID in request")
// 		http.Error(w, "Missing event ID", http.StatusBadRequest)
// 		return
// 	}

// 	// Parse the request body
// 	var newFAQ structs.FAQ
// 	err := json.NewDecoder(r.Body).Decode(&newFAQ)
// 	if err != nil {
// 		log.Printf("Invalid request payload: %v", err)
// 		http.Error(w, "Invalid request payload", http.StatusBadRequest)
// 		return
// 	}

// 	// Validate the input
// 	if newFAQ.Title == "" || newFAQ.Content == "" {
// 		http.Error(w, "Title and content are required", http.StatusBadRequest)
// 		return
// 	}

// 	// Append the new FAQ to the FAQs array in MongoDB
// 	result, err := db.EventsCollection.UpdateOne(
// 		context.TODO(),
// 		bson.M{"eventid": eventID},
// 		bson.M{"$push": bson.M{"faqs": newFAQ}}, // Use $push to append to the array
// 	)
// 	if err != nil {
// 		log.Printf("Error updating event %s: %v", eventID, err)
// 		http.Error(w, "Error updating event", http.StatusInternalServerError)
// 		return
// 	}

// 	if result.MatchedCount == 0 {
// 		log.Printf("Event with ID %s not found", eventID)
// 		http.Error(w, "Event not found", http.StatusNotFound)
// 		return
// 	}

// 	// Respond with success
// 	w.WriteHeader(http.StatusOK)
// 	json.NewEncoder(w).Encode(map[string]any{
// 		"success": true,
// 		"message": "FAQ added successfully",
// 	})
// }
