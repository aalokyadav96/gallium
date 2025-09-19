package baito

import (
	"context"
	"log"
	"naevis/models"
	"naevis/utils"
	"net/http"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// For baito applications
func findAndRespondApplications(ctx context.Context, w http.ResponseWriter, cursor *mongo.Cursor) {
	defer cursor.Close(ctx)
	var results []models.BaitoApplication
	if err := cursor.All(ctx, &results); err != nil {
		log.Printf("Cursor decode error: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to parse results")
		return
	}
	if len(results) == 0 {
		results = []models.BaitoApplication{}
	}

	utils.RespondWithJSON(w, http.StatusOK, results)
}

// For bson.M list (aggregation pipelines)
func findAndRespondBson(ctx context.Context, w http.ResponseWriter, cursor *mongo.Cursor) {
	defer cursor.Close(ctx)
	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		log.Printf("Cursor decode error: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to parse results")
		return
	}
	if len(results) == 0 {
		results = []bson.M{}
	}

	utils.RespondWithJSON(w, http.StatusOK, results)
}
