package reports

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"naevis/db"
	"naevis/models"
	"naevis/utils"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func stringTrim(s string) string { return strings.TrimSpace(s) }

func getActorID(r *http.Request) string {
	return utils.GetUserIDFromRequest(r)
}

// writeJSON helper
func writeJSON(w http.ResponseWriter, v interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	if status != 0 {
		w.WriteHeader(status)
	}
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, msg string, status int) {
	writeJSON(w, map[string]string{"error": msg}, status)
}

// -------------------------
// 1) Submit a Report
// -------------------------
func ReportContent(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()

	var payload models.Report
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	payload.ReportedBy = stringTrim(payload.ReportedBy)
	payload.TargetID = stringTrim(payload.TargetID)
	payload.TargetType = stringTrim(payload.TargetType)
	payload.Reason = stringTrim(payload.Reason)
	payload.Notes = stringTrim(payload.Notes)
	payload.ParentType = stringTrim(payload.ParentType)
	payload.ParentID = stringTrim(payload.ParentID)

	if payload.ReportedBy == "" || payload.TargetID == "" || payload.TargetType == "" || payload.Reason == "" {
		writeError(w, "Missing required field", http.StatusBadRequest)
		return
	}

	// Duplicate check
	filter := bson.M{
		"reportedBy": payload.ReportedBy,
		"targetType": payload.TargetType,
		"targetId":   payload.TargetID,
	}
	var existing models.Report
	err := db.ReportsCollection.FindOne(ctx, filter).Decode(&existing)
	if err == nil {
		writeError(w, "You have already reported this item", http.StatusConflict)
		return
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		writeError(w, "Error checking existing reports", http.StatusInternalServerError)
		return
	}

	// Insert new report
	payload.Status = "pending"
	payload.CreatedAt = time.Now().UTC()
	payload.UpdatedAt = payload.CreatedAt
	payload.Notified = false

	res, err := db.ReportsCollection.InsertOne(ctx, payload)
	if err != nil {
		writeError(w, "Failed to save report", http.StatusInternalServerError)
		return
	}

	idHex := ""
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		idHex = oid.Hex()
	}

	writeJSON(w, map[string]interface{}{
		"message":  "Report submitted",
		"reportId": idHex,
	}, http.StatusCreated)
}

// -------------------------
// 2) Get Reports (moderator)
// Supports ?status, ?targetType, ?reason, ?reportedBy, ?limit, ?offset
// Examples:
//
//	/moderator/reports?status=reviewed&reason=Spam&limit=10&offset=0
//	/moderator/reports?status=pending,reviewed&targetType=post
//
// -------------------------
func GetReports(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	q := r.URL.Query()
	filter := bson.M{}

	// STATUS
	if status := stringTrim(q.Get("status")); status != "" && status != "all" {
		// support comma-separated list
		parts := splitAndTrim(status)
		if len(parts) == 1 {
			filter["status"] = parts[0]
		} else if len(parts) > 1 {
			filter["status"] = bson.M{"$in": parts}
		}
	} else if status == "" {
		// previous default: hide resolved & rejected unless status explicitly asked
		filter["status"] = bson.M{"$nin": []string{"resolved", "rejected"}}
	}

	// TARGET TYPE
	if tt := stringTrim(q.Get("targetType")); tt != "" && tt != "all" {
		filter["targetType"] = tt
	}

	// REASON
	if reason := stringTrim(q.Get("reason")); reason != "" && reason != "all" {
		parts := splitAndTrim(reason)
		if len(parts) == 1 {
			filter["reason"] = parts[0]
		} else {
			filter["reason"] = bson.M{"$in": parts}
		}
	}

	// REPORTED BY
	if rb := stringTrim(q.Get("reportedBy")); rb != "" && rb != "all" {
		filter["reportedBy"] = rb
	}

	// pagination
	limit := int64(10)
	offset := int64(0)

	if l := stringTrim(q.Get("limit")); l != "" {
		if v, err := strconv.ParseInt(l, 10, 64); err == nil && v > 0 {
			if v > 200 {
				limit = 200
			} else {
				limit = v
			}
		}
	}
	if o := stringTrim(q.Get("offset")); o != "" {
		if v, err := strconv.ParseInt(o, 10, 64); err == nil && v >= 0 {
			offset = v
		}
	}

	findOpts := options.Find()
	findOpts.SetLimit(limit)
	findOpts.SetSkip(offset)
	// newest first
	findOpts.SetSort(bson.D{{Key: "createdAt", Value: -1}})

	cursor, err := db.ReportsCollection.Find(ctx, filter, findOpts)
	if err != nil {
		writeError(w, "Failed to fetch reports", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var reports []models.Report
	if err := cursor.All(ctx, &reports); err != nil {
		writeError(w, "Error processing reports", http.StatusInternalServerError)
		return
	}
	if reports == nil {
		reports = []models.Report{}
	}

	writeJSON(w, reports, http.StatusOK)
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := stringTrim(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// -------------------------
// 3) Update Report Status
// -------------------------
func UpdateReport(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	idParam := ps.ByName("id")
	if idParam == "" {
		writeError(w, "Missing report ID in URL", http.StatusBadRequest)
		return
	}
	objID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		writeError(w, "Invalid report ID format", http.StatusBadRequest)
		return
	}

	var payload struct {
		Status      string `json:"status"`
		ReviewedBy  string `json:"reviewedBy,omitempty"`
		ReviewNotes string `json:"reviewNotes,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	payload.Status = stringTrim(payload.Status)
	payload.ReviewedBy = utils.GetUserIDFromRequest(r)
	payload.ReviewNotes = stringTrim(payload.ReviewNotes)
	if payload.Status == "" {
		writeError(w, "Missing required field: status", http.StatusBadRequest)
		return
	}

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
	if payload.Status == "resolved" {
		updateFields["notified"] = false
	}

	filter := bson.M{"_id": objID}
	update := bson.M{"$set": updateFields}

	resUpdate, err := db.ReportsCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		writeError(w, "Failed to update report", http.StatusInternalServerError)
		return
	}
	if resUpdate.MatchedCount == 0 {
		writeError(w, "Report not found", http.StatusNotFound)
		return
	}

	writeJSON(w, map[string]string{"message": "Report updated"}, http.StatusOK)
}

// -------------------------
// SoftDeleteEntity
// PUT /api/v1/moderator/delete/:type/:id
// -------------------------
var errEntityNotFound = errors.New("entity not found")

func SoftDeleteEntity(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	entityType := stringTrim(ps.ByName("type"))
	idParam := stringTrim(ps.ByName("id"))
	if entityType == "" || idParam == "" {
		writeError(w, "Missing type or id", http.StatusBadRequest)
		return
	}

	moderatorID := getActorID(r)
	if moderatorID == "" {
		writeError(w, "Missing moderator id in context", http.StatusUnauthorized)
		return
	}

	if err := setEntityDeletedFlag(ctx, entityType, idParam, true, moderatorID); err != nil {
		if errors.Is(err, errEntityNotFound) {
			writeError(w, "Entity not found", http.StatusNotFound)
			return
		}
		writeError(w, "Failed to soft-delete entity", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"message": "Entity soft-deleted"}, http.StatusOK)
}

// -------------------------
// Appeals: Create / Get / Update
// -------------------------
func CreateAppeal(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()

	var payload struct {
		UserID     string `json:"userId"`
		TargetType string `json:"targetType"`
		TargetID   string `json:"targetId"`
		Reason     string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	payload.UserID = stringTrim(payload.UserID)
	payload.TargetType = stringTrim(payload.TargetType)
	payload.TargetID = stringTrim(payload.TargetID)
	payload.Reason = stringTrim(payload.Reason)

	if payload.UserID == "" || payload.TargetType == "" || payload.TargetID == "" || payload.Reason == "" {
		writeError(w, "Missing required field", http.StatusBadRequest)
		return
	}

	// Duplicate-appeal check
	filter := bson.M{
		"userId":     payload.UserID,
		"targetType": payload.TargetType,
		"targetId":   payload.TargetID,
		"status":     bson.M{"$in": []string{"pending", "submitted"}},
	}
	var existing bson.M
	err := db.AppealsCollection.FindOne(ctx, filter).Decode(&existing)
	if err == nil {
		writeError(w, "You already have a pending appeal for this content", http.StatusConflict)
		return
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		writeError(w, "Failed checking existing appeals", http.StatusInternalServerError)
		return
	}

	// Insert appeal
	now := time.Now().UTC()
	appeal := bson.M{
		"userId":      payload.UserID,
		"targetType":  payload.TargetType,
		"targetId":    payload.TargetID,
		"reason":      payload.Reason,
		"status":      "pending",
		"reviewedBy":  "",
		"reviewNotes": "",
		"createdAt":   now,
		"updatedAt":   now,
	}

	res, err := db.AppealsCollection.InsertOne(ctx, appeal)
	if err != nil {
		writeError(w, "Failed to create appeal", http.StatusInternalServerError)
		return
	}

	idHex := ""
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		idHex = oid.Hex()
	}

	writeJSON(w, map[string]interface{}{
		"message":  "Appeal submitted",
		"appealId": idHex,
	}, http.StatusCreated)
}

func GetAppeals(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	q := r.URL.Query()
	status := stringTrim(q.Get("status"))
	if status == "" {
		status = "pending"
	}
	filter := bson.M{"status": status}

	limit := int64(20)
	offset := int64(0)
	if l := stringTrim(q.Get("limit")); l != "" {
		if v, err := strconv.ParseInt(l, 10, 64); err == nil && v > 0 {
			limit = v
		}
	}
	if o := stringTrim(q.Get("offset")); o != "" {
		if v, err := strconv.ParseInt(o, 10, 64); err == nil && v >= 0 {
			offset = v
		}
	}

	findOpts := options.Find().SetLimit(limit).SetSkip(offset).SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cursor, err := db.AppealsCollection.Find(ctx, filter, findOpts)
	if err != nil {
		writeError(w, "Failed to fetch appeals", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var appeals []bson.M
	if err := cursor.All(ctx, &appeals); err != nil {
		writeError(w, "Error processing appeals", http.StatusInternalServerError)
		return
	}
	if appeals == nil {
		appeals = []bson.M{}
	}

	writeJSON(w, appeals, http.StatusOK)
}

func UpdateAppeal(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	idParam := ps.ByName("id")
	if idParam == "" {
		writeError(w, "Missing appeal ID in URL", http.StatusBadRequest)
		return
	}
	objID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		writeError(w, "Invalid appeal ID format", http.StatusBadRequest)
		return
	}

	var payload struct {
		Status      string `json:"status"`
		ReviewedBy  string `json:"reviewedBy,omitempty"`
		ReviewNotes string `json:"reviewNotes,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	payload.Status = stringTrim(payload.Status)
	payload.ReviewedBy = utils.GetUserIDFromRequest(r)
	payload.ReviewNotes = stringTrim(payload.ReviewNotes)
	if payload.Status == "" {
		writeError(w, "Missing required field: status", http.StatusBadRequest)
		return
	}
	if payload.Status != "approved" && payload.Status != "denied" {
		writeError(w, "Invalid status; must be 'approved' or 'denied'", http.StatusBadRequest)
		return
	}

	var appeal bson.M
	if err := db.AppealsCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&appeal); err != nil {
		if err == mongo.ErrNoDocuments {
			writeError(w, "Appeal not found", http.StatusNotFound)
			return
		}
		writeError(w, "Failed fetching appeal", http.StatusInternalServerError)
		return
	}

	updateFields := bson.M{
		"status":      payload.Status,
		"reviewedBy":  payload.ReviewedBy,
		"reviewNotes": payload.ReviewNotes,
		"updatedAt":   time.Now().UTC(),
	}
	if _, err := db.AppealsCollection.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{"$set": updateFields}); err != nil {
		writeError(w, "Failed to update appeal", http.StatusInternalServerError)
		return
	}

	if payload.Status == "approved" {
		tt, _ := appeal["targetType"].(string)
		tid, _ := appeal["targetId"].(string)
		moderatorID := payload.ReviewedBy
		if moderatorID == "" {
			moderatorID = getActorID(r)
		}
		// ignore error from restore; still respond success for appeal update
		_ = setEntityDeletedFlag(ctx, tt, tid, false, moderatorID)
	}

	writeJSON(w, map[string]string{"message": "Appeal updated"}, http.StatusOK)
}

func setEntityDeletedFlag(ctx context.Context, entityType, id string, deleted bool, by string) error {
	now := time.Now().UTC()

	var coll *mongo.Collection
	var idField string
	var useObjectID bool

	switch entityType {
	case "post":
		coll = db.PostsCollection
		idField = "postid"
	case "place":
		coll = db.PlacesCollection
		idField = "placeid"
	case "event":
		coll = db.EventsCollection
		idField = "eventid"
	case "user":
		coll = db.UserCollection
		idField = "userid"
	case "merch":
		coll = db.MerchCollection
		idField = "merchid"
	case "message":
		coll = db.MessagesCollection
		idField = "_id"
		useObjectID = true
	case "chat":
		coll = db.ChatsCollection
		idField = "_id"
		useObjectID = true
	case "comment":
		coll = db.CommentsCollection
		idField = "_id"
		useObjectID = true
	default:
		return errors.New("unsupported entity type")
	}
	if coll == nil {
		return errors.New("collection not configured")
	}

	var filter bson.M
	if useObjectID {
		oid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return errors.New("invalid ObjectID")
		}
		filter = bson.M{idField: oid}
	} else {
		filter = bson.M{idField: id}
	}

	setMap := bson.M{
		"deleted": deleted,
		"deletedBy": func() interface{} {
			if deleted {
				return by
			}
			return ""
		}(),
		"deletedAt": func() interface{} {
			if deleted {
				return now
			}
			// setting to nil will unset on some drivers; using explicit null here
			return nil
		}(),
	}

	update := bson.M{"$set": setMap}

	res, err := coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return errEntityNotFound
	}
	return nil
}
