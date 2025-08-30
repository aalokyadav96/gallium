package events

import (
	"context"
	"log"
	"naevis/db"
	"naevis/models"
	"naevis/utils"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetEventsCount returns the total count of published events.
func GetEventsCount(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Example static count; replace with a real DB query if needed.
	count := 3
	utils.RespondWithJSON(w, http.StatusOK, count)
}

func GetEvents(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	skip, limit := utils.ParsePagination(r, 10, 100)
	filter := bson.M{} // optionally {"published": true}

	totalCount, err := db.EventsCollection.CountDocuments(ctx, filter)
	if err != nil {
		log.Println("CountDocuments error:", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch event count")
		return
	}

	opts := options.Find().SetSkip(skip).SetLimit(limit).SetSort(bson.D{{Key: "created_at", Value: -1}})
	rawEvents, err := utils.FindAndDecode[models.Event](ctx, db.EventsCollection, filter, opts)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch events")
		return
	}

	safeEvents := make([]models.Event, 0, len(rawEvents))
	for _, e := range rawEvents {
		safeEvents = append(safeEvents, toSafeEvent(e))
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"events":     safeEvents,
		"eventCount": totalCount,
		"page":       skip/int64(limit) + 1,
		"limit":      limit,
	})
}
