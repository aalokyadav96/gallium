package baito

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"naevis/db"
	"naevis/models"
	"naevis/utils"
)

// CreateBaito handles POST /api/baitos/baito
func CreateBaito(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	// gather and trim
	title := strings.TrimSpace(r.FormValue("title"))
	description := strings.TrimSpace(r.FormValue("description"))
	category := strings.TrimSpace(r.FormValue("category"))
	subcategory := strings.TrimSpace(r.FormValue("subcategory"))
	location := strings.TrimSpace(r.FormValue("location"))
	wage := strings.TrimSpace(r.FormValue("wage"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	requirements := strings.TrimSpace(r.FormValue("requirements"))
	workHours := strings.TrimSpace(r.FormValue("workHours"))

	// require all
	if title == "" || description == "" || category == "" ||
		subcategory == "" || location == "" || wage == "" ||
		phone == "" || requirements == "" || workHours == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// optional banner
	var bannerURL string
	if fhArr, ok := r.MultipartForm.File["banner"]; ok && len(fhArr) > 0 {
		url, err := saveUploadedFile(fhArr[0])
		if err != nil {
			http.Error(w, "Banner upload failed", http.StatusInternalServerError)
			return
		}
		bannerURL = url
	}

	// optional multiple images
	var imageURLs []string
	if imageFiles, ok := r.MultipartForm.File["images"]; ok {
		for _, fileHeader := range imageFiles {
			url, err := saveUploadedFile(fileHeader)
			if err != nil {
				http.Error(w, "Image upload failed", http.StatusInternalServerError)
				return
			}
			imageURLs = append(imageURLs, url)
		}
	}

	// build model, including OwnerID
	b := models.Baito{
		Title:        title,
		Description:  description,
		Category:     category,
		SubCategory:  subcategory,
		Location:     location,
		Wage:         wage,
		Phone:        phone,
		Requirements: requirements,
		WorkHours:    workHours,
		BannerURL:    bannerURL,
		Images:       imageURLs,
		CreatedAt:    time.Now(),
		OwnerID:      utils.GetUserIDFromRequest(r),
	}

	res, err := db.BaitoCollection.InsertOne(context.TODO(), b)
	if err != nil {
		http.Error(w, "Failed to save baito", http.StatusInternalServerError)
		return
	}
	b.ID = res.InsertedID.(primitive.ObjectID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"baitoid": b.ID.Hex(),
	})
}

// GetLatestBaitos handles GET /api/baitos/latest
func GetLatestBaitos(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	cursor, err := db.BaitoCollection.Find(context.TODO(),
		bson.M{}, db.OptionsFindLatest(20))
	if err != nil {
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var baitos []models.Baito
	if err := cursor.All(context.TODO(), &baitos); err != nil {
		http.Error(w, "Parse error", http.StatusInternalServerError)
		return
	}

	if len(baitos) == 0 {
		baitos = []models.Baito{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(baitos)
}

// GetRelatedBaitos handles GET /api/baitos/latest
func GetRelatedBaitos(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	cursor, err := db.BaitoCollection.Find(context.TODO(),
		bson.M{}, db.OptionsFindLatest(20))
	if err != nil {
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var baitos []models.Baito
	if err := cursor.All(context.TODO(), &baitos); err != nil {
		http.Error(w, "Parse error", http.StatusInternalServerError)
		return
	}

	if len(baitos) == 0 {
		baitos = []models.Baito{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(baitos)
}

// GetBaitoByID handles GET /api/baitos/baito/:id
func GetBaitoByID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	if err != nil {
		http.Error(w, "Invalid baito ID", http.StatusBadRequest)
		return
	}

	var b models.Baito
	if err := db.BaitoCollection.FindOne(context.TODO(), bson.M{"_id": id}).Decode(&b); err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(b)
}

// ApplyToBaito handles POST /api/baitos/baito/:id/apply
func ApplyToBaito(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	baitoID, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	if err != nil {
		http.Error(w, "Invalid baito ID", http.StatusBadRequest)
		return
	}

	pitch := strings.TrimSpace(r.FormValue("pitch"))
	if pitch == "" {
		http.Error(w, "Pitch message required", http.StatusBadRequest)
		return
	}

	// record who’s applying
	app := models.BaitoApplication{
		BaitoID:     baitoID,
		UserID:      utils.GetUserIDFromRequest(r),
		Username:    utils.GetUsernameFromRequest(r),
		Pitch:       pitch,
		SubmittedAt: time.Now(),
	}

	if _, err := db.BaitoApplicationsCollection.InsertOne(context.TODO(), app); err != nil {
		http.Error(w, "Failed to save application", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Application submitted",
	})
}

// GetMyBaitos handles GET /api/baitos/mine
func GetMyBaitos(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	userID := utils.GetUserIDFromRequest(r)
	cursor, err := db.BaitoCollection.Find(context.TODO(),
		bson.M{"ownerId": userID})
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var myJobs []models.Baito
	if err := cursor.All(context.TODO(), &myJobs); err != nil {
		http.Error(w, "Decode error", http.StatusInternalServerError)
		return
	}
	if len(myJobs) == 0 {
		myJobs = []models.Baito{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(myJobs)
}

// GetBaitoApplicants handles GET /api/baitos/baito/:id/applicants
func GetBaitoApplicants(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	baitoID, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	if err != nil {
		http.Error(w, "Invalid baito ID", http.StatusBadRequest)
		return
	}

	cursor, err := db.BaitoApplicationsCollection.Find(context.TODO(),
		bson.M{"baitoId": baitoID})
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var apps []models.BaitoApplication
	if err := cursor.All(context.TODO(), &apps); err != nil {
		http.Error(w, "Decode error", http.StatusInternalServerError)
		return
	}

	if len(apps) == 0 {
		apps = []models.BaitoApplication{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apps)
}

// func GetMyApplications(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	userID := utils.GetUserIDFromRequest(r)

// 	cursor, err := db.BaitoApplicationsCollection.Find(context.TODO(),
// 		bson.M{"userid": userID})
// 	if err != nil {
// 		http.Error(w, "Database error", http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(context.TODO())

// 	var apps []models.BaitoApplication
// 	if err := cursor.All(context.TODO(), &apps); err != nil {
// 		http.Error(w, "Decode error", http.StatusInternalServerError)
// 		return
// 	}
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(apps)
// }

// // func GetMyApplications(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// // 	userID := utils.GetUserIDFromRequest(r) // returns string
// // 	match := bson.D{{Key: "$match", Value: bson.D{{Key: "userid", Value: userID}}}}

// // 	lookup := bson.D{{Key: "$lookup", Value: bson.D{
// // 		{Key: "from", Value: "baitos"},
// // 		{Key: "localField", Value: "baitoId"},
// // 		{Key: "foreignField", Value: "_id"},
// // 		{Key: "as", Value: "job"},
// // 	}}}

// // 	unwind := bson.D{{Key: "$unwind", Value: "$job"}}

// // 	project := bson.D{{Key: "$project", Value: bson.D{
// // 		{Key: "_id", Value: 1},
// // 		{Key: "pitch", Value: 1},
// // 		{Key: "submittedAt", Value: 1},
// // 		{Key: "jobId", Value: "$job._id"},
// // 		{Key: "title", Value: "$job.title"},
// // 		{Key: "location", Value: "$job.location"},
// // 		{Key: "wage", Value: "$job.wage"},
// // 	}}}

// // 	cursor, err := db.BaitoApplicationsCollection.Aggregate(context.TODO(),
// // 		mongo.Pipeline{match, lookup, unwind, project})
// // 	if err != nil {
// // 		http.Error(w, "Failed to fetch applications", http.StatusInternalServerError)
// // 		return
// // 	}
// // 	defer cursor.Close(context.TODO())

// // 	var results []bson.M
// // 	if err := cursor.All(context.TODO(), &results); err != nil {
// // 		http.Error(w, "Decode error", http.StatusInternalServerError)
// // 		return
// // 	}

// // 	w.Header().Set("Content-Type", "application/json")
// // 	json.NewEncoder(w).Encode(results)
// // }

func GetMyApplications(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	userID := utils.GetUserIDFromRequest(r)

	match := bson.D{{Key: "$match", Value: bson.D{{Key: "userid", Value: userID}}}}

	lookup := bson.D{{Key: "$lookup", Value: bson.D{
		{Key: "from", Value: "baito"}, // ← use your actual collection name
		{Key: "localField", Value: "baitoId"},
		{Key: "foreignField", Value: "_id"},
		{Key: "as", Value: "job"},
	}}}

	unwind := bson.D{{Key: "$unwind", Value: "$job"}}

	project := bson.D{{Key: "$project", Value: bson.D{
		{Key: "_id", Value: 1},
		{Key: "pitch", Value: 1},
		{Key: "submittedAt", Value: 1},
		{Key: "jobId", Value: "$job._id"},
		{Key: "title", Value: "$job.title"},
		{Key: "location", Value: "$job.location"},
		{Key: "wage", Value: "$job.wage"},
	}}}

	cursor, err := db.BaitoApplicationsCollection.Aggregate(
		context.TODO(),
		mongo.Pipeline{match, lookup, unwind, project},
		// mongo.Pipeline{match, lookup, project},
	)
	if err != nil {
		http.Error(w, "Failed to fetch applications", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var results []bson.M
	if err := cursor.All(context.TODO(), &results); err != nil {
		http.Error(w, "Decode error", http.StatusInternalServerError)
		return
	}

	if len(results) == 0 {
		results = []bson.M{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// helper: save file
func saveUploadedFile(fh *multipart.FileHeader) (string, error) {
	src, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	ext := filepath.Ext(fh.Filename)
	name := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	dstPath := filepath.Join("static", "uploads", "baitos", name)

	dst, err := os.Create(dstPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err = io.Copy(dst, src); err != nil {
		return "", err
	}
	// return "/uploads/baitos/" + name, nil
	return name, nil
}
