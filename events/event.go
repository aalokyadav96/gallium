package events

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"naevis/db"
	"naevis/models"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

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

	var rawEvent models.Event
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
func toSafeEvent(e models.Event) models.Event {
	// default empty slices
	if e.Tickets == nil {
		e.Tickets = []models.Ticket{}
	}
	if e.Merch == nil {
		e.Merch = []models.Merch{}
	}
	if e.FAQs == nil {
		e.FAQs = []models.FAQ{}
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
	var newFAQ models.FAQ
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
