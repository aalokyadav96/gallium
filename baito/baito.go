package baito

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"naevis/db"
	"naevis/filemgr"
	"naevis/models"
	"naevis/mq"
	"naevis/utils"
)

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("JSON encode error: %v", err)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// Replaces generic function: explicitly for baito slice
func findAndRespondBaitos(ctx context.Context, w http.ResponseWriter, cursor *mongo.Cursor) {
	defer cursor.Close(ctx)
	var results []models.Baito
	if err := cursor.All(ctx, &results); err != nil {
		log.Printf("Cursor decode error: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to parse results")
		return
	}

	if len(results) == 0 {
		results = []models.Baito{}
	}

	respondJSON(w, http.StatusOK, results)
}

// For baito applications
func findAndRespondApplications(ctx context.Context, w http.ResponseWriter, cursor *mongo.Cursor) {
	defer cursor.Close(ctx)
	var results []models.BaitoApplication
	if err := cursor.All(ctx, &results); err != nil {
		log.Printf("Cursor decode error: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to parse results")
		return
	}
	if len(results) == 0 {
		results = []models.BaitoApplication{}
	}

	respondJSON(w, http.StatusOK, results)
}

// For bson.M list (aggregation pipelines)
func findAndRespondBson(ctx context.Context, w http.ResponseWriter, cursor *mongo.Cursor) {
	defer cursor.Close(ctx)
	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		log.Printf("Cursor decode error: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to parse results")
		return
	}
	if len(results) == 0 {
		results = []bson.M{}
	}

	respondJSON(w, http.StatusOK, results)
}

// ------------------- Handlers -------------------

func GetLatestBaitos(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	opts := db.OptionsFindLatest(20).SetSort(bson.M{"createdAt": -1})

	cursor, err := db.BaitoCollection.Find(ctx, bson.M{}, opts)
	if err != nil {
		log.Printf("DB error: %v", err)
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	findAndRespondBaitos(ctx, w, cursor)
}

func GetRelatedBaitos(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	GetLatestBaitos(w, r, nil)
}

func GetBaitoByID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	id := ps.ByName("id")

	var b models.Baito
	if err := db.BaitoCollection.FindOne(ctx, bson.M{"baitoid": id}).Decode(&b); err != nil {
		if err == mongo.ErrNoDocuments {
			respondError(w, http.StatusNotFound, "Not found")
		} else {
			log.Printf("DB error: %v", err)
			respondError(w, http.StatusInternalServerError, "Database error")
		}
		return
	}

	respondJSON(w, http.StatusOK, b)
}

func ApplyToBaito(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid form data")
		return
	}
	defer r.MultipartForm.RemoveAll()

	pitch := strings.TrimSpace(r.FormValue("pitch"))
	if pitch == "" {
		respondError(w, http.StatusBadRequest, "Pitch message required")
		return
	}

	app := models.BaitoApplication{
		BaitoID:     ps.ByName("id"),
		UserID:      utils.GetUserIDFromRequest(r),
		Username:    utils.GetUsernameFromRequest(r),
		Pitch:       pitch,
		SubmittedAt: time.Now(),
	}

	if _, err := db.BaitoApplicationsCollection.InsertOne(ctx, app); err != nil {
		log.Printf("Insert error: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to save application")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
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
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	findAndRespondBaitos(ctx, w, cursor)
}

func GetBaitoApplicants(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	cursor, err := db.BaitoApplicationsCollection.Find(ctx, bson.M{"baitoid": ps.ByName("id")})
	if err != nil {
		log.Printf("DB error: %v", err)
		respondError(w, http.StatusInternalServerError, "Database error")
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
		respondError(w, http.StatusInternalServerError, "Failed to fetch applications")
		return
	}

	findAndRespondBson(ctx, w, cursor)
}

func CreateBaitoUserProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	userID := utils.GetUserIDFromRequest(r)

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid form data")
		return
	}
	defer r.MultipartForm.RemoveAll()

	var existing models.BaitoWorker
	if err := db.BaitoWorkerCollection.FindOne(ctx, bson.M{"userid": userID}).Decode(&existing); err == nil {
		respondError(w, http.StatusConflict, "Worker profile already exists")
		return
	} else if err != mongo.ErrNoDocuments {
		log.Printf("DB error: %v", err)
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}

	age, err := strconv.Atoi(r.FormValue("age"))
	if err != nil || age < 16 {
		respondError(w, http.StatusBadRequest, "Invalid age")
		return
	}

	profilePic, _ := filemgr.SaveFormFile(r.MultipartForm, "picture", filemgr.EntityBaito, filemgr.PicPhoto, false)

	profile := models.BaitoWorker{
		UserID:      userID,
		BaitoUserID: utils.GenerateRandomString(12),
		Name:        r.FormValue("name"),
		Age:         age,
		Phone:       r.FormValue("phone"),
		Location:    r.FormValue("location"),
		Preferred:   r.FormValue("roles"),
		Bio:         r.FormValue("bio"),
		ProfilePic:  profilePic,
		CreatedAt:   time.Now(),
	}

	if _, err := db.BaitoWorkerCollection.InsertOne(ctx, profile); err != nil {
		log.Printf("Insert error: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to save worker profile")
		return
	}

	_, _ = db.UserCollection.UpdateOne(ctx,
		bson.M{"userid": userID},
		bson.M{
			"$addToSet": bson.M{"role": "worker"},
			"$set":      bson.M{"updated_at": time.Now()},
		},
	)

	go mq.Emit(ctx, "worker-created", models.Index{
		EntityType: "worker", EntityId: profile.BaitoUserID, Method: "POST",
	})
	respondJSON(w, http.StatusOK, map[string]string{"message": "Worker profile created successfully"})
}

func CreateBaito(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid form data")
		return
	}
	defer r.MultipartForm.RemoveAll()

	b := models.Baito{
		BaitoId:      utils.GenerateRandomString(15),
		Title:        strings.TrimSpace(r.FormValue("title")),
		Description:  strings.TrimSpace(r.FormValue("description")),
		Category:     strings.TrimSpace(r.FormValue("category")),
		SubCategory:  strings.TrimSpace(r.FormValue("subcategory")),
		Location:     strings.TrimSpace(r.FormValue("location")),
		Wage:         strings.TrimSpace(r.FormValue("wage")),
		Phone:        strings.TrimSpace(r.FormValue("phone")),
		Requirements: strings.TrimSpace(r.FormValue("requirements")),
		WorkHours:    strings.TrimSpace(r.FormValue("workHours")),
		OwnerID:      utils.GetUserIDFromRequest(r),
		CreatedAt:    time.Now(),
	}

	if b.Title == "" || b.Description == "" || b.Category == "" || b.SubCategory == "" ||
		b.Location == "" || b.Wage == "" || b.Phone == "" || b.Requirements == "" || b.WorkHours == "" {
		respondError(w, http.StatusBadRequest, "Missing required fields")
		return
	}

	if banner, _ := filemgr.SaveFormFile(r.MultipartForm, "banner", filemgr.EntityBaito, filemgr.PicBanner, false); banner != "" {
		b.BannerURL = banner
	}
	if images, _ := filemgr.SaveFormFiles(r.MultipartForm, "images", filemgr.EntityBaito, filemgr.PicPhoto, false); len(images) > 0 {
		b.Images = images
	}

	if _, err := db.BaitoCollection.InsertOne(ctx, b); err != nil {
		log.Printf("Insert error: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to save baito")
		return
	}

	go mq.Emit(ctx, "baito-created", models.Index{
		EntityType: "baito", EntityId: b.BaitoId, Method: "POST",
	})
	respondJSON(w, http.StatusOK, map[string]string{"baitoid": b.BaitoId})
}

func UpdateBaito(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid form data")
		return
	}
	defer r.MultipartForm.RemoveAll()

	update := bson.M{
		"$set": bson.M{
			"title":        strings.TrimSpace(r.FormValue("title")),
			"description":  strings.TrimSpace(r.FormValue("description")),
			"category":     strings.TrimSpace(r.FormValue("category")),
			"subcategory":  strings.TrimSpace(r.FormValue("subcategory")),
			"location":     strings.TrimSpace(r.FormValue("location")),
			"wage":         strings.TrimSpace(r.FormValue("wage")),
			"phone":        strings.TrimSpace(r.FormValue("phone")),
			"requirements": strings.TrimSpace(r.FormValue("requirements")),
			"workHours":    strings.TrimSpace(r.FormValue("workHours")),
			"updatedAt":    time.Now(),
		},
	}

	if banner, _ := filemgr.SaveFormFile(r.MultipartForm, "banner", filemgr.EntityBaito, filemgr.PicBanner, false); banner != "" {
		update["$set"].(bson.M)["banner"] = banner
	}
	if images, _ := filemgr.SaveFormFiles(r.MultipartForm, "images", filemgr.EntityBaito, filemgr.PicPhoto, false); len(images) > 0 {
		update["$set"].(bson.M)["images"] = images
	}

	filter := bson.M{
		"baitoid": ps.ByName("id"),
		"ownerId": utils.GetUserIDFromRequest(r),
	}

	result, err := db.BaitoCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		log.Printf("Update error: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to update baito")
		return
	}
	if result.MatchedCount == 0 {
		respondError(w, http.StatusNotFound, "Baito not found or unauthorized")
		return
	}

	go mq.Emit(ctx, "baito-updated", models.Index{
		EntityType: "baito", EntityId: ps.ByName("id"), Method: "PUT",
	})
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Baito updated",
		"baitoid": ps.ByName("id"),
	})
}

// Workers
func GetWorkers(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	search := strings.TrimSpace(r.URL.Query().Get("search"))
	skill := strings.TrimSpace(r.URL.Query().Get("skill"))

	filter := bson.M{}
	if search != "" {
		filter["$or"] = bson.A{
			utils.RegexFilter("name", search),
			utils.RegexFilter("address", search),
		}
	}
	if skill != "" {
		filter["preferred_roles"] = skill
	}

	skip, limit := utils.ParsePagination(r, 10, 100)
	opts := options.Find().SetSkip(skip).SetLimit(limit).SetSort(bson.M{"created_at": -1})

	workers, err := utils.FindAndDecode[models.BaitoWorker](ctx, db.BaitoWorkerCollection, filter, opts)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "Failed to fetch workers")
		return
	}

	if len(workers) == 0 {
		workers = []models.BaitoWorker{}
	}

	total, _ := db.BaitoWorkerCollection.CountDocuments(ctx, filter)
	utils.JSON(w, http.StatusOK, map[string]any{
		"data":  workers,
		"total": total,
	})
}

func GetWorkerSkills(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	values, err := db.BaitoWorkerCollection.Distinct(ctx, "preferred_roles", bson.M{})
	if err != nil {
		log.Printf("DB error: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to fetch skills")
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

	respondJSON(w, http.StatusOK, skills)
}

func GetWorkerById(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var worker models.BaitoWorker
	err := db.BaitoWorkerCollection.FindOne(ctx, bson.M{"baito_user_id": ps.ByName("workerId")}).Decode(&worker)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			respondError(w, http.StatusNotFound, "Worker not found")
		} else {
			log.Printf("DB error: %v", err)
			respondError(w, http.StatusInternalServerError, "Failed to fetch worker")
		}
		return
	}

	respondJSON(w, http.StatusOK, worker)
}
