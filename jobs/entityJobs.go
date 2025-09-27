package jobs

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"naevis/db"
	"naevis/models"
	"naevis/mq"
	"naevis/utils"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// ------------------ CREATE ------------------

func CreateBaitoForEntity(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	userID := utils.GetUserIDFromRequest(r)

	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")

	var baito models.Baito
	if err := json.NewDecoder(r.Body).Decode(&baito); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if baito.Title == "" || baito.Description == "" || baito.Category == "" ||
		baito.Location == "" || baito.Wage == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Missing required fields")
		return
	}

	// // Validate required fields
	// if baito.Title == "" || baito.Description == "" || baito.Category == "" || baito.SubCategory == "" ||
	// 	baito.Location == "" || baito.Wage == "" || baito.Phone == "" || baito.Requirements == "" || baito.WorkHours == "" {
	// 	utils.RespondWithError(w, http.StatusBadRequest, "Missing required fields")
	// 	return
	// }

	// Assign system values
	baito.BaitoId = utils.GenerateRandomString(15)
	baito.EntityType = entityType
	baito.EntityID = entityID
	baito.OwnerID = userID
	baito.CreatedAt = time.Now()
	baito.LastDateToApply = time.Now().AddDate(0, 1, 0) // default 1 month
	baito.ApplicationCount = 0

	_, err := db.BaitoCollection.InsertOne(ctx, baito)
	if err != nil {
		log.Printf("Insert error: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save baito")
		return
	}

	// Publish to MQ
	go mq.Emit(ctx, "baito-created", models.Index{
		EntityType: baito.EntityType,
		EntityId:   baito.BaitoId,
		Method:     "POST",
	})

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"baitoid": baito.BaitoId})
}

// ------------------ READ LIST ------------------
func GetJobsRelatedTOEntity(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")

	cursor, err := db.BaitoCollection.Find(ctx, bson.M{
		"entityType": entityType,
		"entityId":   entityID,
	})
	if err != nil {
		http.Error(w, "Failed to fetch jobs", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var jobs []models.BaitosResponse
	if err := cursor.All(ctx, &jobs); err != nil {
		http.Error(w, "Failed to decode jobs", http.StatusInternalServerError)
		return
	}

	if len(jobs) == 0 {
		jobs = []models.BaitosResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bson.M{"jobs": jobs})
}
