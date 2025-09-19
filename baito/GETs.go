package baito

import (
	"context"
	"log"
	"naevis/db"
	"naevis/models"
	"naevis/utils"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetWorkers(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	// defer cancel()

	ctx := r.Context()

	search := strings.TrimSpace(r.URL.Query().Get("search"))
	skill := strings.TrimSpace(r.URL.Query().Get("skill"))
	log.Println("hi")
	filter := bson.M{}
	if search != "" {
		filter["$or"] = bson.A{
			utils.RegexFilter("name", search),
			utils.RegexFilter("address", search),
			utils.RegexFilter("bio", search),
		}
	}
	if skill != "" {
		filter["preferred_roles"] = bson.M{"$in": []string{skill}}
	}

	skip, limit := utils.ParsePagination(r, 10, 100)
	opts := options.Find().SetSkip(skip).SetLimit(limit).SetSort(bson.M{"created_at": -1})

	workers, err := utils.FindAndDecode[models.BaitoWorkersResponse](ctx, db.BaitoWorkerCollection, filter, opts)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch workers")
		return
	}

	if len(workers) == 0 {
		workers = []models.BaitoWorkersResponse{}
	}

	total, _ := db.BaitoWorkerCollection.CountDocuments(ctx, filter)
	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"data":  workers,
		"total": total,
	})
}

func GetLatestBaitos(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	opts := db.OptionsFindLatest(20).SetSort(bson.M{"createdAt": -1})

	cursor, err := db.BaitoCollection.Find(ctx, bson.M{}, opts)
	if err != nil {
		log.Printf("DB error: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	findAndRespondBaitos(ctx, w, cursor)
}

func GetRelatedBaitos(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	GetLatestBaitos(w, r, nil)
}

// Explicitly decode baitos (no generics)
func findAndRespondBaitos(ctx context.Context, w http.ResponseWriter, cursor *mongo.Cursor) {
	defer cursor.Close(ctx)
	var results []models.BaitosResponse
	if err := cursor.All(ctx, &results); err != nil {
		log.Printf("Cursor decode error: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to parse results")
		return
	}

	if len(results) == 0 {
		results = []models.BaitosResponse{}
	}

	utils.RespondWithJSON(w, http.StatusOK, results)
}
