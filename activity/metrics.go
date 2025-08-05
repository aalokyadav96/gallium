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
		Events []map[string]interface{} `json:"events"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if len(payload.Events) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	docs := make([]interface{}, 0, len(payload.Events))
	now := time.Now()

	for _, event := range payload.Events {
		// Normalize timestamp if present
		var ts time.Time
		if rawTS, ok := event["ts"].(float64); ok {
			ts = time.UnixMilli(int64(rawTS))
		} else {
			ts = now
		}

		doc := bson.M{
			"type":      event["type"],
			"data":      event["data"],
			"url":       event["url"],
			"userAgent": event["ua"],
			"timestamp": ts,
			"session":   event["session"],
			"user":      event["user"],
			"referrer":  event["referrer"],
			"width":     event["width"],
			"height":    event["height"],
			"lang":      event["lang"],
			"platform":  event["platform"],
			"ip":        r.RemoteAddr,
		}

		docs = append(docs, doc)
	}

	_, err := db.AnalyticsCollection.InsertMany(globals.CTX, docs)
	if err != nil {
		http.Error(w, "failed to save events", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
