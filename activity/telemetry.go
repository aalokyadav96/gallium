package activity

import (
	"context"
	"encoding/json"
	"log"
	"naevis/db"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func HandleTelemetry(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	defer r.Body.Close()
	var data map[string]interface{}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if _, ok := data["ts"]; !ok {
		data["ts"] = time.Now().UnixMilli()
	}
	if data["event"] == nil && data["type"] != nil {
		data["event"] = data["type"]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := db.AnalyticsCollection.InsertOne(ctx, bson.M(data)); err != nil {
		log.Println("Failed to insert telemetry:", err)
		http.Error(w, "DB insert error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
