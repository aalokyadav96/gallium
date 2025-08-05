package baito

import (
	"context"
	"encoding/json"
	"naevis/db"
	"naevis/models"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetWorkers(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	search := strings.TrimSpace(r.URL.Query().Get("search"))
	skill := strings.TrimSpace(r.URL.Query().Get("skill"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}

	query := bson.M{}

	if search != "" {
		query["$or"] = bson.A{
			bson.M{"name": bson.M{"$regex": search, "$options": "i"}},
			bson.M{"location": bson.M{"$regex": search, "$options": "i"}},
		}
	}

	if skill != "" {
		query["preferred_roles"] = skill
	}

	skip := (page - 1) * limit
	opts := options.Find().SetSkip(int64(skip)).SetLimit(int64(limit))

	cursor, err := db.BaitoWorkerCollection.Find(ctx, query, opts)
	if err != nil {
		http.Error(w, "Error querying workers", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var workers []models.BaitoUserProfile
	if err := cursor.All(ctx, &workers); err != nil {
		http.Error(w, "Failed to parse results", http.StatusInternalServerError)
		return
	}

	if len(workers) == 0 {
		workers = []models.BaitoUserProfile{}
	}

	total, _ := db.BaitoWorkerCollection.CountDocuments(ctx, query)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":  workers,
		"total": total,
	})
}

func GetWorkerSkills(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	values, err := db.BaitoWorkerCollection.Distinct(ctx, "preferred_roles", bson.M{})
	if err != nil {
		http.Error(w, "Failed to fetch skills", http.StatusInternalServerError)
		return
	}

	skills := make([]string, 0, len(values))
	for _, v := range values {
		if str, ok := v.(string); ok && str != "" {
			skills = append(skills, str)
		}
	}

	if len(skills) == 0 {
		skills = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(skills)
}

func GetWorkerById(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	workerId := ps.ByName("workerId")
	if workerId == "" {
		http.Error(w, "Missing worker ID", http.StatusBadRequest)
		return
	}

	var worker models.BaitoUserProfile
	err := db.BaitoWorkerCollection.FindOne(ctx, bson.M{"baito_user_id": workerId}).Decode(&worker)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Worker not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to fetch worker", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(worker)
}
