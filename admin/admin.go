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

// func GetReports(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	// Filter out any report whose status is exactly "resolved"
// 	filter := bson.M{"status": bson.M{"$ne": "resolved"}}

// 	cursor, err := db.ReportsCollection.Find(context.TODO(), filter)
// 	if err != nil {
// 		http.Error(w, `{"error":"Failed to fetch reports"}`, http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(context.TODO())

// 	var reports []models.Report
// 	if err := cursor.All(context.TODO(), &reports); err != nil {
// 		http.Error(w, `{"error":"Error processing reports"}`, http.StatusInternalServerError)
// 		return
// 	}

// 	// Write JSON; thanks to models.Report.MarshalJSON, each object has "id":"<hex>" plus all new fields
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(reports)
// }

// package admin

// import (
// 	"context"
// 	"encoding/json"
// 	"naevis/db"
// 	"naevis/models"
// 	"net/http"

// 	"github.com/julienschmidt/httprouter"
// 	"go.mongodb.org/mongo-driver/bson"
// )

// // //	func GetReports(next httprouter.Handle) httprouter.Handle {
// // //		return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// // //		}
// // //	}

// // func GetReports(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// // 	cursor, err := db.ReportsCollection.Find(context.TODO(), bson.M{})
// // 	if err != nil {
// // 		http.Error(w, `{"error":"Failed to fetch reports"}`, http.StatusInternalServerError)
// // 		return
// // 	}
// // 	defer cursor.Close(context.TODO())

// // 	var reports []models.Report
// // 	if err := cursor.All(context.TODO(), &reports); err != nil {
// // 		http.Error(w, `{"error":"Error processing reports"}`, http.StatusInternalServerError)
// // 		return
// // 	}

// //		w.Header().Set("Content-Type", "application/json")
// //		json.NewEncoder(w).Encode(reports)
// //	}

// func GetReports(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	// Only fetch reports where status is not "resolved"
// 	filter := bson.M{"status": bson.M{"$ne": "resolved"}}

// 	cursor, err := db.ReportsCollection.Find(context.TODO(), filter)
// 	if err != nil {
// 		http.Error(w, `{"error":"Failed to fetch reports"}`, http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(context.TODO())

// 	var reports []models.Report
// 	if err := cursor.All(context.TODO(), &reports); err != nil {
// 		http.Error(w, `{"error":"Error processing reports"}`, http.StatusInternalServerError)
// 		return
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(reports)
// }
