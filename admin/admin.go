package admin

import (
	"context"
	"encoding/json"
	"net/http"

	"naevis/db"
	"naevis/models"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// GetReports returns all non‐resolved reports for the admin UI.
//
// Endpoint: GET /admin/reports
//
// Response: 200 OK [ { …Report fields… }, { … } … ]
// Reports with status == "resolved" are excluded.

func GetReports(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Filter out reports whose status is either "resolved" or "rejected"
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
	json.NewEncoder(w).Encode(reports)
}
