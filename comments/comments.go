package comments

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"naevis/db"
	"naevis/middleware"
	"naevis/models"
	"naevis/utils"
)

// CreateComment adds a new comment
func CreateComment(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if strings.TrimSpace(body.Content) == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Comment cannot be empty")
		return
	}

	tokenString := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(tokenString)
	if err != nil {
		utils.RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	comment := models.Comment{
		EntityType: entityType,
		EntityID:   entityID,
		CreatedBy:  claims.UserID,
		Content:    body.Content,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	res, err := db.CommentsCollection.InsertOne(ctx, comment)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "DB insert failed")
		return
	}
	comment.ID = res.InsertedID.(primitive.ObjectID).Hex()

	utils.RespondWithJSON(w, http.StatusCreated, comment)
}

// GetComment fetches a single comment by ID
func GetComment(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	commentID := ps.ByName("commentid") // Correctly get the comment ID
	objID, err := primitive.ObjectIDFromHex(commentID)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	var comment models.Comment
	err = db.CommentsCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&comment)
	if err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "Comment not found")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, comment)
}

// UpdateComment edits a comment
func UpdateComment(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	commentID := ps.ByName("commentid")
	objID, err := primitive.ObjectIDFromHex(commentID)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	tokenString := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(tokenString)
	if err != nil {
		utils.RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Ensure user owns the comment
	var existing models.Comment
	err = db.CommentsCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&existing)
	if err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "Comment not found")
		return
	}
	if existing.CreatedBy != claims.UserID {
		utils.RespondWithError(w, http.StatusForbidden, "Forbidden")
		return
	}

	update := bson.M{
		"$set": bson.M{
			"content":    body.Content,
			"updated_at": time.Now(),
		},
	}

	_, err = db.CommentsCollection.UpdateByID(ctx, objID, update)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "DB update failed")
		return
	}

	// Return updated comment
	err = db.CommentsCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&existing)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Fetch failed")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, existing)
}

// DeleteComment removes a comment
func DeleteComment(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	commentID := ps.ByName("commentid")
	objID, err := primitive.ObjectIDFromHex(commentID)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	tokenString := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(tokenString)
	if err != nil {
		utils.RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var existing models.Comment
	err = db.CommentsCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&existing)
	if err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "Comment not found")
		return
	}

	if existing.CreatedBy != claims.UserID {
		utils.RespondWithError(w, http.StatusForbidden, "Forbidden")
		return
	}

	_, err = db.CommentsCollection.DeleteOne(ctx, bson.M{"_id": objID})
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Delete failed")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
