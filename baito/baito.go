package baito

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"naevis/db"
	"naevis/models"
	"naevis/utils"
)

// ------------------- Handlers -------------------

func GetBaitoByID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	id := ps.ByName("baitoid")

	var b models.Baito
	if err := db.BaitoCollection.FindOne(ctx, bson.M{"baitoid": id}).Decode(&b); err != nil {
		if err == mongo.ErrNoDocuments {
			utils.RespondWithError(w, http.StatusNotFound, "Not found")
		} else {
			log.Printf("DB error: %v", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Database error")
		}
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, b)
}

func ApplyToBaito(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid form data")
		return
	}
	defer r.MultipartForm.RemoveAll()

	pitch := strings.TrimSpace(r.FormValue("pitch"))
	if pitch == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Pitch message required")
		return
	}

	app := models.BaitoApplication{
		BaitoID:     ps.ByName("baitoid"),
		UserID:      utils.GetUserIDFromRequest(r),
		Username:    utils.GetUsernameFromRequest(r),
		Pitch:       pitch,
		SubmittedAt: time.Now(),
	}

	if _, err := db.BaitoApplicationsCollection.InsertOne(ctx, app); err != nil {
		log.Printf("Insert error: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save application")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Application submitted",
	})
}

func GetMyBaitos(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	userID := utils.GetUserIDFromRequest(r)

	cursor, err := db.BaitoCollection.Find(ctx, bson.M{"ownerId": userID}, options.Find().SetSort(bson.M{"createdAt": -1}))
	if err != nil {
		log.Printf("DB error: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	findAndRespondBaitos(ctx, w, cursor)
}

func GetBaitoApplicants(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	cursor, err := db.BaitoApplicationsCollection.Find(ctx, bson.M{"baitoid": ps.ByName("baitoid")})
	if err != nil {
		log.Printf("DB error: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	findAndRespondApplications(ctx, w, cursor)
}

func GetMyApplications(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	userID := utils.GetUserIDFromRequest(r)

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"userid": userID}}},
		{{Key: "$lookup", Value: bson.M{
			"from":         "baitos",
			"localField":   "baitoid",
			"foreignField": "baitoid",
			"as":           "job",
		}}},
		{{Key: "$unwind", Value: "$job"}},
		{{Key: "$project", Value: bson.M{
			"id":          "$_id",
			"pitch":       1,
			"submittedAt": 1,
			"jobId":       "$job.baitoid",
			"title":       "$job.title",
			"location":    "$job.location",
			"wage":        "$job.wage",
		}}},
	}

	cursor, err := db.BaitoApplicationsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		log.Printf("Aggregate error: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch applications")
		return
	}

	findAndRespondBson(ctx, w, cursor)
}
