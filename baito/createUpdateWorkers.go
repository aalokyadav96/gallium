package baito

import (
	"log"
	"naevis/db"
	"naevis/filemgr"
	"naevis/models"
	"naevis/mq"
	"naevis/utils"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// parseWorkerForm parses form data for create or update
func parseWorkerForm(r *http.Request, isUpdate bool) (models.BaitoWorker, bson.M, error) {
	var worker models.BaitoWorker
	update := bson.M{"$set": bson.M{}}

	if err := r.ParseMultipartForm(20 << 20); err != nil {
		return worker, update, err
	}
	defer r.MultipartForm.RemoveAll()

	ageStr := r.FormValue("age")
	age, _ := strconv.Atoi(ageStr)

	preferredRoles := r.FormValue("roles")
	roles := []string{}
	if preferredRoles != "" {
		for _, r := range strings.Split(preferredRoles, ",") {
			if trimmed := strings.TrimSpace(r); trimmed != "" {
				roles = append(roles, trimmed)
			}
		}
	}

	picture, _ := filemgr.SaveFormFile(r.MultipartForm, "picture", filemgr.EntityWorker, filemgr.PicPhoto, false)
	documents, _ := filemgr.SaveFormFiles(r.MultipartForm, "documents", filemgr.EntityWorker, filemgr.PicPhoto, false)

	if isUpdate {
		set := update["$set"].(bson.M)
		set["name"] = r.FormValue("name")
		set["age"] = age
		set["phone"] = r.FormValue("phone")
		set["location"] = r.FormValue("location")
		set["preferred_roles"] = roles
		set["bio"] = r.FormValue("bio")
		set["email"] = r.FormValue("email")
		set["experience"] = r.FormValue("experience")
		set["skills"] = r.FormValue("skills")
		set["availability"] = r.FormValue("availability")
		set["expected_wage"] = r.FormValue("expected_wage")
		set["languages"] = r.FormValue("languages")
		set["updatedAt"] = time.Now()

		if picture != "" {
			set["profile_pic"] = picture
		}
		if len(documents) > 0 {
			set["documents"] = documents
		}
	} else {
		worker = models.BaitoWorker{
			UserID:       utils.GetUserIDFromRequest(r),
			BaitoUserID:  utils.GenerateRandomString(12),
			Name:         r.FormValue("name"),
			Age:          age,
			Phone:        r.FormValue("phone"),
			Location:     r.FormValue("location"),
			Preferred:    roles,
			Bio:          r.FormValue("bio"),
			Email:        r.FormValue("email"),
			Experience:   r.FormValue("experience"),
			Skills:       r.FormValue("skills"),
			Availability: r.FormValue("availability"),
			ExpectedWage: r.FormValue("expected_wage"),
			Languages:    r.FormValue("languages"),
			ProfilePic:   picture,
			Documents:    documents,
			CreatedAt:    time.Now(),
		}
	}

	return worker, update, nil
}

// CreateWorkerProfile handles creating a new worker profile
func CreateWorkerProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	userID := utils.GetUserIDFromRequest(r)

	// Check if worker profile already exists
	var existingWorker models.BaitoWorker
	err := db.BaitoWorkerCollection.FindOne(ctx, bson.M{"userid": userID}).Decode(&existingWorker)
	if err == nil {
		utils.RespondWithError(w, http.StatusConflict, "Worker profile already exists")
		return
	} else if err != mongo.ErrNoDocuments {
		log.Printf("DB error: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// Parse form data
	worker, _, err := parseWorkerForm(r, false)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid form data")
		return
	}

	// Validate required fields
	if worker.Name == "" || worker.Age < 16 || worker.Phone == "" || worker.Location == "" || len(worker.Preferred) == 0 || worker.Bio == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Missing required fields")
		return
	}

	// Set user ID explicitly in worker object
	worker.BaitoUserID = userID

	// Insert new worker profile
	if _, err := db.BaitoWorkerCollection.InsertOne(ctx, worker); err != nil {
		log.Printf("Insert error: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save worker profile")
		return
	}

	// Update user role and timestamp
	_, err = db.UserCollection.UpdateOne(ctx,
		bson.M{"userid": userID},
		bson.M{
			"$addToSet": bson.M{"role": "worker"},
			"$set":      bson.M{"updated_at": time.Now()},
		},
	)
	if err != nil {
		log.Printf("User update error: %v", err)
		// Not fatal, so we continue
	}

	// Emit event asynchronously
	go mq.Emit(ctx, "worker-created", models.Index{
		EntityType: "worker",
		EntityId:   worker.BaitoUserID,
		Method:     "POST",
	})

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Worker profile created successfully"})
}

// UpdateWorkerProfile handles updating an existing worker profile
func UpdateWorkerProfile(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	userID := utils.GetUserIDFromRequest(r)
	workerID := ps.ByName("id")

	_, update, err := parseWorkerForm(r, true)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid form data")
		return
	}

	filter := bson.M{"baitouserid": workerID, "userid": userID}
	result, err := db.BaitoWorkerCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		log.Printf("Update error: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update worker profile")
		return
	}

	if result.MatchedCount == 0 {
		utils.RespondWithError(w, http.StatusNotFound, "Worker profile not found or unauthorized")
		return
	}

	go mq.Emit(ctx, "worker-updated", models.Index{
		EntityType: "worker", EntityId: workerID, Method: "PUT",
	})
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{
		"message":  "Worker profile updated successfully",
		"workerId": workerID,
	})
}
