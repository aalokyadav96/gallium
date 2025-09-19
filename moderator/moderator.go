package moderator

import (
	"context"
	"encoding/json"
	"net/http"

	"naevis/db"
	"naevis/models"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func GetReports(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	filter := bson.M{
		"status": bson.M{
			"$nin": []string{"resolved", "rejected"},
		},
	}

	cursor, err := db.ReportsCollection.Find(context.TODO(), filter)
	if err != nil {
		http.Error(w, `{"error":"Failed to fetch reports"}`, http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var reports []models.Report
	if err := cursor.All(context.TODO(), &reports); err != nil {
		http.Error(w, `{"error":"Error processing reports"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(reports)
}
