package reviews

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"naevis/db"
	"naevis/globals"
	"naevis/models"
	"naevis/mq"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// var db.ReviewsCollection *mongo.Collection

// Reviews
func GetReviews(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	entityType := ps.ByName("entityType")
	entityId := ps.ByName("entityId")

	skip, limit := utils.ParsePagination(r, 10, 100)
	sort := utils.ParseSort(r.URL.Query().Get("sort"), bson.D{{Key: "createdAt", Value: -1}}, nil)

	filter := bson.M{"entity_type": entityType, "entity_id": entityId}
	opts := options.Find().SetSkip(skip).SetLimit(limit).SetSort(sort)

	reviews, err := utils.FindAndDecode[models.Review](ctx, db.ReviewsCollection, filter, opts)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve reviews")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{"status": http.StatusOK, "ok": true, "reviews": reviews})
}

// GET /api/reviews/:entityType/:entityId/:reviewId
func GetReview(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	reviewId := ps.ByName("reviewId")

	var review models.Review
	err := db.ReviewsCollection.FindOne(context.TODO(), bson.M{"reviewid": reviewId}).Decode(&review)
	if err != nil {
		http.Error(w, fmt.Sprintf("Review not found: %v", err), http.StatusNotFound)
		return
	}
	log.Println("get review : ", review)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(review)
}

// POST /api/reviews/:entityType/:entityId
func AddReview(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	userId, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok || userId == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	log.Println("dfdj")
	var err error

	entityType := ps.ByName("entityType")
	entityId := ps.ByName("entityId")

	count, err := db.ReviewsCollection.CountDocuments(context.TODO(), bson.M{
		"userId":     userId,
		"entityType": entityType,
		"entityId":   entityId,
	})
	if err != nil {
		log.Printf("Error checking for existing review: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if count > 0 {
		http.Error(w, "You have already reviewed this entity", http.StatusConflict)
		return
	}

	var review models.Review
	if err := json.NewDecoder(r.Body).Decode(&review); err != nil || review.Rating < 1 || review.Rating > 5 || review.Comment == "" {
		http.Error(w, "Invalid review data", http.StatusBadRequest)
		return
	}

	review.ReviewID = utils.GenerateRandomString(16)
	review.UserID = userId
	review.EntityType = entityType
	review.EntityID = entityId
	// review.Date = time.Now().Format(time.RFC3339)
	review.Date = time.Now()

	inserted, err := db.ReviewsCollection.InsertOne(context.TODO(), review)
	if err != nil {
		http.Error(w, "Failed to insert review: "+err.Error(), http.StatusInternalServerError)
		return
	}

	userdata.SetUserData("review", review.ReviewID, userId, entityType, entityId)

	m := models.Index{EntityType: "review", EntityId: review.ReviewID, Method: "POST", ItemId: entityId, ItemType: entityType}
	go mq.Emit(ctx, "review-added", m)

	log.Println("review : ", review.ReviewID)
	log.Println("inserted review : ", inserted)

	w.WriteHeader(http.StatusCreated)
}

// PUT /api/reviews/:entityType/:entityId/:reviewId
func EditReview(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	userId, _ := r.Context().Value(globals.UserIDKey).(string)
	reviewId := ps.ByName("reviewId")

	var review models.Review
	err := db.ReviewsCollection.FindOne(context.TODO(), bson.M{"reviewid": reviewId}).Decode(&review)
	if err != nil {
		http.Error(w, fmt.Sprintf("Review not found: %v", err), http.StatusNotFound)
		return
	}

	if review.UserID != userId && !isAdmin(r.Context()) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var updatedFields map[string]any
	if err := json.NewDecoder(r.Body).Decode(&updatedFields); err != nil {
		http.Error(w, "Invalid update data", http.StatusBadRequest)
		return
	}

	_, err = db.ReviewsCollection.UpdateOne(
		context.TODO(),
		bson.M{"reviewid": reviewId},
		bson.M{"$set": updatedFields},
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to update Review: %v", err), http.StatusInternalServerError)
		return
	}

	m := models.Index{EntityType: "review", EntityId: reviewId, Method: "PUT", ItemId: review.EntityID, ItemType: review.EntityType}
	go mq.Emit(ctx, "review-edited", m)

	w.WriteHeader(http.StatusOK)
}

// DELETE /api/reviews/:entityType/:entityId/:reviewId
func DeleteReview(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	userId, _ := r.Context().Value(globals.UserIDKey).(string)
	reviewId := ps.ByName("reviewId")

	var review models.Review
	err := db.ReviewsCollection.FindOne(context.TODO(), bson.M{"reviewid": reviewId}).Decode(&review)
	if err != nil {
		http.Error(w, fmt.Sprintf("Review not found: %v", err), http.StatusNotFound)
		return
	}

	if review.UserID != userId && !isAdmin(r.Context()) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	_, err = db.ReviewsCollection.DeleteOne(context.TODO(), bson.M{"reviewid": reviewId})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete review: %v", err), http.StatusInternalServerError)
		return
	}

	userdata.DelUserData("review", reviewId, userId)

	m := models.Index{EntityType: "review", EntityId: reviewId, Method: "DELETE", ItemId: review.EntityID, ItemType: review.EntityType}
	go mq.Emit(ctx, "review-deleted", m)

	w.WriteHeader(http.StatusOK)
}

func isAdmin(ctx context.Context) bool {
	role, ok := ctx.Value(roleKey).(string)
	return ok && role == "admin"
}

const roleKey = "role"
