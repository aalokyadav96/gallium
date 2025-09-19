package beats

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"naevis/db"
	"naevis/models"
	"naevis/mq"
	"naevis/userdata"
	"naevis/utils"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Generic subscribe/follow/unfollow handler
func HandleEntitySubscription(w http.ResponseWriter, r *http.Request, ps httprouter.Params, entityType, action string) {
	ctx := r.Context()
	currentUserID := utils.GetUserIDFromRequest(r)
	if currentUserID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	entityID := ps.ByName("id")
	if entityID == "" {
		http.Error(w, "Entity ID required", http.StatusBadRequest)
		return
	}

	if err := UpdateEntitySubscription(ctx, currentUserID, entityType, entityID, action); err != nil {
		log.Printf("Failed to update %s subscription: %v", entityType, err)
		http.Error(w, "Failed to update subscription", http.StatusInternalServerError)
		return
	}

	// Optionally update user data for UI notifications
	userdata.SetUserData(action, entityID, currentUserID, entityType, entityID)

	resp := map[string]any{
		"hasSubscribed": action == "subscribe",
		"ok":            true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// PUT /api/v1/subscribes/:id
func SubscribeEntity(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	HandleEntitySubscription(w, r, ps, ps.ByName("type"), "subscribe")
}

// DELETE /api/v1/subscribes/:id
func UnsubscribeEntity(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	HandleEntitySubscription(w, r, ps, ps.ByName("type"), "unsubscribe")
}

// GET /api/v1/subscribes/:type/:id
func DoesSubscribeEntity(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	currentUserID := utils.GetUserIDFromRequest(r)
	if currentUserID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	entityID := ps.ByName("id")
	if entityID == "" {
		http.Error(w, "Entity ID required", http.StatusBadRequest)
		return
	}

	entityType := ps.ByName("type")

	// Map entityType to the correct collection / field
	var filter bson.M
	switch entityType {
	case "user", "artist", "feedpost":
		filter = bson.M{
			"userid": currentUserID,
			"subscribed": bson.M{
				"$in": []string{entityID},
			},
		}
	default:
		http.Error(w, "Invalid entity type", http.StatusBadRequest)
		return
	}

	count, err := db.SubscribersCollection.CountDocuments(r.Context(), filter)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	resp := map[string]bool{"hasSubscribed": count > 0}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Core generic logic for updating subscription/follow/unfollow
func UpdateEntitySubscription(ctx context.Context, userID, entityType, entityID, action string) error {
	if action != "subscribe" && action != "unsubscribe" {
		return fmt.Errorf("invalid action: %s", action)
	}

	// Determine updates based on action
	opCurrent := bson.M{"$addToSet": bson.M{"subscribed": entityID}}
	opTarget := bson.M{"$addToSet": bson.M{"subscribers": userID}}
	method := "PUT"
	event := "subscribed"

	if action == "unsubscribe" {
		opCurrent = bson.M{"$pull": bson.M{"subscribed": entityID}}
		opTarget = bson.M{"$pull": bson.M{"subscribers": userID}}
		method = "DELETE"
		event = "unsubscribed"
	}

	// Update user subscription list
	_, err := db.SubscribersCollection.UpdateOne(ctx, bson.M{"userid": userID}, opCurrent, options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("failed to update user's subscriptions: %w", err)
	}

	// Update target entity's subscribers list
	_, err = db.SubscribersCollection.UpdateOne(ctx, bson.M{"userid": entityID}, opTarget, options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("failed to update entity's subscribers: %w", err)
	}

	// Emit MQ event
	m := models.Index{EntityType: entityType, EntityId: userID, Method: method, ItemId: entityID}
	go mq.Emit(ctx, event, m)

	return nil
}

// Helper: ensure entry exists in collection
func EnsureSubscriptionEntry(userID string) {
	update := bson.M{"$setOnInsert": bson.M{"userid": userID, "subscribed": []string{}, "subscribers": []string{}}}
	_, err := db.SubscribersCollection.UpdateOne(context.TODO(), bson.M{"userid": userID}, update, options.Update().SetUpsert(true))
	if err != nil {
		log.Printf("Failed to create subscription entry for user %s: %v", userID, err)
	}
}

// GET /api/v1/subscribers/:id
func GetSubscribers(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	targetUserID := ps.ByName("id")
	if targetUserID == "" {
		http.Error(w, "Target user ID required", http.StatusBadRequest)
		return
	}

	var sub models.UserSubscribe
	err := db.SubscribersCollection.FindOne(r.Context(), bson.M{"userid": targetUserID}).Decode(&sub)
	if err != nil && err != mongo.ErrNoDocuments {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if len(sub.Subscribers) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]models.User{})
		return
	}

	cursor, err := db.SubscribersCollection.Find(r.Context(), bson.M{"userid": bson.M{"$in": sub.Subscribers}})
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(r.Context())

	var subscribers []models.User
	if err := cursor.All(r.Context(), &subscribers); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(subscribers)
}
