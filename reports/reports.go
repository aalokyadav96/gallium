package reports

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"naevis/db"
	"naevis/models"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// -------------------------
// 1) Submit a Report
// -------------------------
// Endpoint: POST /report
//
// Expects JSON payload:
//
//	{
//	  "reportedBy":  "user123",
//	  "targetId":    "post567",
//	  "targetType":  "post",
//	  "reason":      "Spam",
//	  "notes":       "Contains repeated ads"    (optional)
//	}
//
// Returns:
//
//	201 Created { "message": "Report submitted" }
//	400 Bad Request { "error": "Missing required field: reason" }
//	409 Conflict { "error": "You have already reported this item" }
//	500 Internal Server Error { "error": "Failed to save report" }
func ReportContent(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Decode incoming JSON into models.Report
	var payload models.Report
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, `{"error":"Invalid JSON payload"}`, http.StatusBadRequest)
		return
	}

	// Trim whitespace (just in case)
	payload.ReportedBy = stringTrim(payload.ReportedBy)
	payload.TargetID = stringTrim(payload.TargetID)
	payload.TargetType = stringTrim(payload.TargetType)
	payload.Reason = stringTrim(payload.Reason)
	payload.Notes = stringTrim(payload.Notes)

	// 1.1) Validate required fields
	if payload.ReportedBy == "" {
		http.Error(w, `{"error":"Missing required field: reportedBy"}`, http.StatusBadRequest)
		return
	}
	if payload.TargetID == "" {
		http.Error(w, `{"error":"Missing required field: targetId"}`, http.StatusBadRequest)
		return
	}
	if payload.TargetType == "" {
		http.Error(w, `{"error":"Missing required field: targetType"}`, http.StatusBadRequest)
		return
	}
	if payload.Reason == "" {
		http.Error(w, `{"error":"Missing required field: reason"}`, http.StatusBadRequest)
		return
	}

	// 1.2) Check for duplicate: same user, same targetType, same targetId
	filter := bson.M{
		"reportedBy": payload.ReportedBy,
		"targetType": payload.TargetType,
		"targetId":   payload.TargetID,
	}
	var existing models.Report
	err := db.ReportsCollection.FindOne(context.TODO(), filter).Decode(&existing)
	if err == nil {
		// Document found → duplicate
		http.Error(w, `{"error":"You have already reported this item"}`, http.StatusConflict)
		return
	}
	// If err != nil and not a “NoDocuments” error, it’s a DB error
	if err.Error() != "mongo: no documents in result" {
		http.Error(w, `{"error":"Error checking existing reports"}`, http.StatusInternalServerError)
		return
	}

	// 1.3) Initialize status & timestamps
	payload.Status = "pending"
	payload.CreatedAt = time.Now().UTC()
	payload.UpdatedAt = time.Now().UTC()

	// 1.4) Insert into MongoDB
	res, err := db.ReportsCollection.InsertOne(context.TODO(), payload)
	if err != nil {
		http.Error(w, `{"error":"Failed to save report"}`, http.StatusInternalServerError)
		return
	}

	// Return 201 Created + new report ID
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "Report submitted",
		"reportId": res.InsertedID,
	})
}

// -------------------------
// 2) Get All Reports
// -------------------------
// Endpoint: GET /reports
//
// Returns an array of all reports (only for moderators/admins).
//
//	200 OK [ { … }, { … } … ]
//	500 Internal Server Error { "error": "Failed to fetch reports" }
func GetReports(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	cursor, err := db.ReportsCollection.Find(context.TODO(), bson.M{})
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

// -------------------------
// 3) Update Report Status (Moderator Action)
// -------------------------
// Endpoint: PUT /report/:id
//
// Expects JSON payload (one or both of the following):
//
//	{
//	  "status":      "resolved",         // required
//	  "reviewedBy":  "modUser123",       // optional
//	  "reviewNotes": "User warned, post removed"  // optional
//	}
//
// Returns:
//
//	200 OK { "message": "Report updated" }
//	400 Bad Request { "error": "Invalid report ID" }
//	404 Not Found { "error": "Report not found" }
//	500 Internal Server Error { "error": "Failed to update report" }
func UpdateReport(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	idParam := ps.ByName("id")
	if idParam == "" {
		http.Error(w, `{"error":"Missing report ID in URL"}`, http.StatusBadRequest)
		return
	}

	// 3.1) Parse ID into ObjectID
	objID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		http.Error(w, `{"error":"Invalid report ID format"}`, http.StatusBadRequest)
		return
	}

	// 3.2) Decode update payload
	var payload struct {
		Status      string `json:"status"`
		ReviewedBy  string `json:"reviewedBy,omitempty"`
		ReviewNotes string `json:"reviewNotes,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, `{"error":"Invalid JSON payload"}`, http.StatusBadRequest)
		return
	}

	payload.Status = stringTrim(payload.Status)
	payload.ReviewedBy = stringTrim(payload.ReviewedBy)
	payload.ReviewNotes = stringTrim(payload.ReviewNotes)

	// 3.3) Validate status
	if payload.Status == "" {
		http.Error(w, `{"error":"Missing required field: status"}`, http.StatusBadRequest)
		return
	}
	// (You can also enforce that status ∈ {"pending","reviewed","resolved","rejected"} if you like.)

	// 3.4) Build the update document
	updateFields := bson.M{
		"status":    payload.Status,
		"updatedAt": time.Now().UTC(),
	}
	if payload.ReviewedBy != "" {
		updateFields["reviewedBy"] = payload.ReviewedBy
	}
	if payload.ReviewNotes != "" {
		updateFields["reviewNotes"] = payload.ReviewNotes
	}

	filter := bson.M{"_id": objID}
	update := bson.M{"$set": updateFields}

	// 3.5) Perform the update
	res, err := db.ReportsCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		http.Error(w, `{"error":"Failed to update report"}`, http.StatusInternalServerError)
		return
	}
	if res.MatchedCount == 0 {
		http.Error(w, `{"error":"Report not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Report updated"})
}

// -------------------------
// Helper: Trim whitespace from a string
// -------------------------
func stringTrim(s string) string {
	return strings.TrimSpace(s)
}

// package reports

// import (
// 	"context"
// 	"encoding/json"
// 	"naevis/db"
// 	"naevis/models"
// 	"net/http"
// 	"time"

// 	"github.com/julienschmidt/httprouter"
// 	"go.mongodb.org/mongo-driver/bson"
// )

// // Submit a Report
// func ReportContent(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	var payload models.Report
// 	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
// 		http.Error(w, "Invalid data", http.StatusBadRequest)
// 		return
// 	}

// 	payload.Status = "pending"
// 	payload.CreatedAt = time.Now()
// 	payload.UpdatedAt = time.Now()

// 	_, err := db.ReportsCollection.InsertOne(context.TODO(), payload)
// 	if err != nil {
// 		http.Error(w, "Failed to report", http.StatusInternalServerError)
// 		return
// 	}

// 	json.NewEncoder(w).Encode(map[string]string{"message": "Report submitted"})
// }

// // Get All Reports
// func GetReports(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	cursor, err := db.ReportsCollection.Find(context.TODO(), bson.M{})
// 	if err != nil {
// 		http.Error(w, "Failed to fetch reports", http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(context.TODO())

// 	var reports []models.Report
// 	if err = cursor.All(context.TODO(), &reports); err != nil {
// 		http.Error(w, "Error processing reports", http.StatusInternalServerError)
// 		return
// 	}

// 	json.NewEncoder(w).Encode(reports)
// }

// // Update Report Status
// func UpdateReport(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	id := ps.ByName("id")

// 	var updateData struct {
// 		Status string `json:"status"`
// 		Notes  string `json:"notes,omitempty"`
// 	}

// 	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
// 		http.Error(w, "Invalid data", http.StatusBadRequest)
// 		return
// 	}

// 	filter := bson.M{"_id": id}
// 	update := bson.M{"$set": bson.M{"status": updateData.Status, "notes": updateData.Notes, "updatedAt": time.Now()}}

// 	_, err := db.ReportsCollection.UpdateOne(context.TODO(), filter, update)
// 	if err != nil {
// 		http.Error(w, "Failed to update report", http.StatusInternalServerError)
// 		return
// 	}

// 	json.NewEncoder(w).Encode(map[string]string{"message": "Report updated"})
// }
