package baito

import (
	"context"
	"log"
	"naevis/db"
	"naevis/models"
	"naevis/utils"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Workers

func GetWorkerById(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var worker models.BaitoWorker
	err := db.BaitoWorkerCollection.FindOne(ctx, bson.M{"baito_user_id": ps.ByName("workerId")}).Decode(&worker)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.RespondWithError(w, http.StatusNotFound, "Worker not found")
		} else {
			log.Printf("DB error: %v", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch worker")
		}
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, worker)
}

func GetWorkerSkills(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	values, err := db.BaitoWorkerCollection.Distinct(ctx, "preferred_roles", bson.M{})
	if err != nil {
		log.Printf("DB error: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch skills")
		return
	}

	var skills []string
	for _, v := range values {
		if str, ok := v.(string); ok && str != "" {
			skills = append(skills, str)
		}
	}
	if len(skills) == 0 {
		skills = []string{}
	}

	utils.RespondWithJSON(w, http.StatusOK, skills)
}
