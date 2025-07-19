package activity

import (
	"encoding/json"
	"naevis/db"
	"naevis/globals"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func HandleAnalyticsEvent(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var payload struct {
		Event     string                 `json:"event"`
		Data      map[string]interface{} `json:"data"`
		Timestamp int64                  `json:"timestamp"`
		Path      string                 `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// Save to MongoDB (assuming you have analytics collection)
	_, err := db.AnalyticsCollection.InsertOne(globals.CTX, bson.M{
		"event":     payload.Event,
		"data":      payload.Data,
		"timestamp": time.UnixMilli(payload.Timestamp),
		"path":      payload.Path,
		"ip":        r.RemoteAddr,
		"userAgent": r.UserAgent(),
	})
	if err != nil {
		http.Error(w, "failed to save event", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
