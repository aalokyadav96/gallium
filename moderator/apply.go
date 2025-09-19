package moderator

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"naevis/db"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ModeratorApplication struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    string             `bson:"userId" json:"userId"`
	Reason    string             `bson:"reason" json:"reason"`
	Status    string             `bson:"status" json:"status"` // pending, approved, rejected
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

func ApplyModerator(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var payload struct {
		UserID string `json:"userId"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, `{"error":"Invalid JSON payload"}`, http.StatusBadRequest)
		return
	}

	payload.UserID = strings.TrimSpace(payload.UserID)
	payload.Reason = strings.TrimSpace(payload.Reason)

	if payload.UserID == "" || payload.Reason == "" {
		http.Error(w, `{"error":"Missing required fields"}`, http.StatusBadRequest)
		return
	}

	// Prevent duplicate applications
	filter := bson.M{"userId": payload.UserID}
	var existing ModeratorApplication
	err := db.ModeratorApplications.FindOne(context.TODO(), filter).Decode(&existing)
	if err == nil {
		http.Error(w, `{"error":"You have already applied to be a moderator"}`, http.StatusConflict)
		return
	}

	app := ModeratorApplication{
		ID:        primitive.NewObjectID(),
		UserID:    payload.UserID,
		Reason:    payload.Reason,
		Status:    "pending",
		CreatedAt: time.Now().UTC(),
	}

	_, err = db.ModeratorApplications.InsertOne(context.TODO(), app)
	if err != nil {
		http.Error(w, `{"error":"Failed to save application"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Moderator application submitted",
		"id":      app.ID.Hex(),
	})
}
