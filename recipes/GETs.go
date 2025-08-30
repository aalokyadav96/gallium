package recipes

import (
	"context"
	"encoding/json"
	"naevis/db"
	"naevis/models"
	"naevis/utils"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// --- List Recipes ---
func GetRecipes(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	filter := bson.M{}
	if search := r.URL.Query().Get("search"); search != "" {
		filter["$or"] = []bson.M{
			utils.RegexFilter("title", search),
			utils.RegexFilter("description", search),
		}
	}
	if ing := r.URL.Query().Get("ingredient"); ing != "" {
		filter["ingredients.name"] = bson.M{"$regex": regexp.QuoteMeta(ing), "$options": "i"}
	}
	if tags := r.URL.Query().Get("tags"); tags != "" {
		tagList := strings.Split(tags, ",")
		filter["tags"] = bson.M{"$all": tagList}
	}

	skip, limit := utils.ParsePagination(r, 10, 100)

	sort := utils.ParseSort(r.URL.Query().Get("sort"), bson.D{{Key: "createdAt", Value: -1}}, map[string]bson.D{
		"newest":   {{Key: "createdAt", Value: -1}},
		"oldest":   {{Key: "createdAt", Value: 1}},
		"views":    {{Key: "views", Value: -1}},
		"prepTime": {{Key: "prepTime", Value: 1}},
	})

	opts := options.Find().SetSort(sort).SetSkip(skip).SetLimit(limit)
	recipes, err := utils.FindAndDecode[models.Recipe](ctx, db.RecipeCollection, filter, opts)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch recipes")
		return
	}

	// Normalize slices for all recipes
	for i := range recipes {
		normalizeRecipeSlices(&recipes[i])
	}

	totalCount, err := db.RecipeCollection.CountDocuments(ctx, filter)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to count recipes")
		return
	}

	hasMore := (skip + limit) < int64(totalCount)

	utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"recipes": recipes,
		"hasMore": hasMore,
	})
}

func GetRecipeTags(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := context.TODO()
	pipeline := mongo.Pipeline{
		{{Key: "$unwind", Value: "$tags"}},
		{{Key: "$group", Value: bson.M{
			"_id":  nil,
			"tags": bson.M{"$addToSet": "$tags"},
		}}},
	}

	cursor, err := db.RecipeCollection.Aggregate(ctx, pipeline)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var result []struct {
		Tags []string `bson:"tags"`
	}
	if err := cursor.All(ctx, &result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tags := []string{}
	if len(result) > 0 && result[0].Tags != nil {
		tags = result[0].Tags
	}

	json.NewEncoder(w).Encode(tags)
}

// // Recipes
// func GetRecipes(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
// 	defer cancel()

// 	filter := bson.M{}
// 	if search := r.URL.Query().Get("search"); search != "" {
// 		filter["$or"] = []bson.M{
// 			utils.RegexFilter("title", search),
// 			utils.RegexFilter("description", search),
// 		}
// 	}
// 	if ing := r.URL.Query().Get("ingredient"); ing != "" {
// 		filter["ingredients.name"] = bson.M{"$regex": regexp.QuoteMeta(ing), "$options": "i"}
// 	}
// 	if tags := r.URL.Query().Get("tags"); tags != "" {
// 		tagList := strings.Split(tags, ",")
// 		filter["tags"] = bson.M{"$all": tagList}
// 	}

// 	skip, limit := utils.ParsePagination(r, 10, 100)

// 	sort := utils.ParseSort(r.URL.Query().Get("sort"), bson.D{{Key: "createdAt", Value: -1}}, map[string]bson.D{
// 		"newest":   {{Key: "createdAt", Value: -1}},
// 		"oldest":   {{Key: "createdAt", Value: 1}},
// 		"views":    {{Key: "views", Value: -1}},
// 		"prepTime": {{Key: "prepTime", Value: 1}},
// 	})

// 	opts := options.Find().SetSort(sort).SetSkip(skip).SetLimit(limit)
// 	recipes, err := utils.FindAndDecode[models.Recipe](ctx, db.RecipeCollection, filter, opts)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch recipes")
// 		return
// 	}

// 	// Get total count for pagination
// 	totalCount, err := db.RecipeCollection.CountDocuments(ctx, filter)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to count recipes")
// 		return
// 	}

// 	hasMore := (skip + limit) < int64(totalCount)

// 	// Return wrapped response
// 	resp := map[string]interface{}{
// 		"recipes": recipes,
// 		"hasMore": hasMore,
// 	}

// 	utils.RespondWithJSON(w, http.StatusOK, resp)
// }

// func GetRecipeTags(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx := context.TODO()
// 	pipeline := mongo.Pipeline{
// 		{{Key: "$unwind", Value: "$tags"}},
// 		{{Key: "$group", Value: bson.M{
// 			"_id":  nil,
// 			"tags": bson.M{"$addToSet": "$tags"},
// 		}}},
// 	}

// 	cursor, err := db.RecipeCollection.Aggregate(ctx, pipeline)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(ctx)

// 	var result []struct {
// 		Tags []string `bson:"tags"`
// 	}
// 	if err := cursor.All(ctx, &result); err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	if len(result) > 0 {
// 		json.NewEncoder(w).Encode(result[0].Tags)
// 	} else {
// 		json.NewEncoder(w).Encode([]string{})
// 	}
// }
