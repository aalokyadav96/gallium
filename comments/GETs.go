package comments

import (
	"context"
	"naevis/db"
	"naevis/models"
	"naevis/utils"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetComments returns paginated and sorted comments for an entity
func GetComments(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	entityType := ps.ByName("entitytype")
	entityId := ps.ByName("entityid")

	// Pagination
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	sortBy := r.URL.Query().Get("sort") // "new", "old", "likes"

	page := 1
	limit := 10
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	filter := bson.M{
		"entity_type": entityType,
		"entity_id":   entityId,
	}

	// Sorting
	findOptions := options.Find()
	findOptions.SetSkip(int64((page - 1) * limit))
	findOptions.SetLimit(int64(limit))

	switch sortBy {
	case "old":
		findOptions.SetSort(bson.D{{Key: "createdAt", Value: 1}})
	case "likes":
		findOptions.SetSort(bson.D{{Key: "likes", Value: -1}})
	default: // "new"
		findOptions.SetSort(bson.D{{Key: "createdAt", Value: -1}})
	}

	comments, err := utils.FindAndDecode[models.Comment](ctx, db.CommentsCollection, filter, findOptions)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch comments")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, comments)
}
