package reports

import (
	"context"
	"encoding/json"
	"naevis/db"
	"naevis/models"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// Submit a Report
func ReportContent(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var payload models.Report
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid data", http.StatusBadRequest)
		return
	}

	payload.Status = "pending"
	payload.CreatedAt = time.Now()
	payload.UpdatedAt = time.Now()

	_, err := db.ReportsCollection.InsertOne(context.TODO(), payload)
	if err != nil {
		http.Error(w, "Failed to report", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Report submitted"})
}

// Get All Reports
func GetReports(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	cursor, err := db.ReportsCollection.Find(context.TODO(), bson.M{})
	if err != nil {
		http.Error(w, "Failed to fetch reports", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var reports []models.Report
	if err = cursor.All(context.TODO(), &reports); err != nil {
		http.Error(w, "Error processing reports", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(reports)
}

// Update Report Status
func UpdateReport(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")

	var updateData struct {
		Status string `json:"status"`
		Notes  string `json:"notes,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		http.Error(w, "Invalid data", http.StatusBadRequest)
		return
	}

	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{"status": updateData.Status, "notes": updateData.Notes, "updatedAt": time.Now()}}

	_, err := db.ReportsCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		http.Error(w, "Failed to update report", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"message": "Report updated"})
}

// func ReportContent(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	var payload models.Report
// 	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
// 		http.Error(w, "Invalid data", http.StatusBadRequest)
// 		return
// 	}

// 	payload.Status = "pending"
// 	payload.CreatedAt = time.Now()
// 	payload.UpdatedAt = time.Now()

// 	// Insert to MongoDB
// 	_, err := db.ReportsCollection.InsertOne(context.TODO(), payload)
// 	if err != nil {
// 		http.Error(w, "Failed to report", http.StatusInternalServerError)
// 		return
// 	}

// 	json.NewEncoder(w).Encode(map[string]string{"message": "Report submitted"})
// }
