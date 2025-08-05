package baito

import (
	"context"
	"naevis/db"
	"naevis/filemgr"
	"naevis/models"
	"naevis/utils"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func CreateBaitoUserProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := context.Background()
	userID := utils.GetUserIDFromRequest(r)

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	var existing models.BaitoUserProfile
	err := db.BaitoWorkerCollection.FindOne(ctx, bson.M{"user_id": userID}).Decode(&existing)
	if err == nil {
		http.Error(w, "Worker profile already exists", http.StatusConflict)
		return
	} else if err != mongo.ErrNoDocuments {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	name := r.FormValue("name")
	ageStr := r.FormValue("age")
	phone := r.FormValue("phone")
	location := r.FormValue("location")
	roles := r.FormValue("roles")
	bio := r.FormValue("bio")

	if name == "" || ageStr == "" || phone == "" || location == "" || roles == "" || bio == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	age, err := strconv.Atoi(ageStr)
	if err != nil || age < 16 {
		http.Error(w, "Invalid age", http.StatusBadRequest)
		return
	}

	profilePic, _ := filemgr.SaveFormFile(r.MultipartForm, "picture", filemgr.EntityBaito, filemgr.PicPhoto, false)

	profile := models.BaitoUserProfile{
		UserID:      userID,
		BaitoUserID: utils.GenerateID(12),
		Name:        name,
		Age:         age,
		Phone:       phone,
		Location:    location,
		Preferred:   roles,
		Bio:         bio,
		ProfilePic:  profilePic,
		CreatedAt:   time.Now(),
	}

	if _, err := db.BaitoWorkerCollection.InsertOne(ctx, profile); err != nil {
		http.Error(w, "Failed to save worker profile", http.StatusInternalServerError)
		return
	}

	_, _ = db.UserCollection.UpdateOne(ctx,
		bson.M{"userid": userID},
		bson.M{
			"$addToSet": bson.M{"role": "worker"},
			"$set":      bson.M{"updated_at": time.Now()},
		},
	)

	utils.RespondWithJSON(w, http.StatusOK, utils.M{"message": "Worker profile created successfully"})
}

func CreateBaito(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	b := models.Baito{
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
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	banner, _ := filemgr.SaveFormFile(r.MultipartForm, "banner", filemgr.EntityBaito, filemgr.PicBanner, false)
	if banner != "" {
		b.BannerURL = banner
	}

	images, _ := filemgr.SaveFormFiles(r.MultipartForm, "images", filemgr.EntityBaito, filemgr.PicImage, false)
	if len(images) > 0 {
		b.Images = images
	}

	res, err := db.BaitoCollection.InsertOne(context.TODO(), b)
	if err != nil {
		http.Error(w, "Failed to save baito", http.StatusInternalServerError)
		return
	}

	b.ID = res.InsertedID.(primitive.ObjectID)
	utils.RespondWithJSON(w, http.StatusOK, utils.M{"baitoid": b.ID.Hex()})
}

func UpdateBaito(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	baitoId := ps.ByName("id")
	objID, err := primitive.ObjectIDFromHex(baitoId)
	if err != nil {
		http.Error(w, "Invalid baito ID", http.StatusBadRequest)
		return
	}

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

	if images, _ := filemgr.SaveFormFiles(r.MultipartForm, "images", filemgr.EntityBaito, filemgr.PicImage, false); len(images) > 0 {
		update["$set"].(bson.M)["images"] = images
	}

	filter := bson.M{
		"_id":     objID,
		"ownerId": utils.GetUserIDFromRequest(r),
	}

	result, err := db.BaitoCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		http.Error(w, "Failed to update baito", http.StatusInternalServerError)
		return
	}
	if result.MatchedCount == 0 {
		http.Error(w, "Baito not found or unauthorized", http.StatusNotFound)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, utils.M{
		"message": "Baito updated",
		"baitoid": baitoId,
	})
}

// const uploadDir = "static/uploads/baitos"

// func CreateBaitoUserProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx := context.Background()
// 	userID := utils.GetUserIDFromRequest(r)

// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		http.Error(w, "Invalid form data", http.StatusBadRequest)
// 		return
// 	}

// 	var existing models.BaitoUserProfile
// 	err := db.BaitoWorkerCollection.FindOne(ctx, bson.M{"user_id": userID}).Decode(&existing)
// 	if err == nil {
// 		http.Error(w, "Worker profile already exists", http.StatusConflict)
// 		return
// 	} else if err != mongo.ErrNoDocuments {
// 		http.Error(w, "Database error", http.StatusInternalServerError)
// 		return
// 	}

// 	name := r.FormValue("name")
// 	ageStr := r.FormValue("age")
// 	phone := r.FormValue("phone")
// 	location := r.FormValue("location")
// 	roles := r.FormValue("roles")
// 	bio := r.FormValue("bio")

// 	if name == "" || ageStr == "" || phone == "" || location == "" || roles == "" || bio == "" {
// 		http.Error(w, "All fields are required", http.StatusBadRequest)
// 		return
// 	}

// 	age, err := strconv.Atoi(ageStr)
// 	if err != nil || age < 16 {
// 		http.Error(w, "Invalid age", http.StatusBadRequest)
// 		return
// 	}

// 	profilePic, _ := filemgr.SaveFormFile(r, "picture", uploadDir, false)

// 	profile := models.BaitoUserProfile{
// 		UserID:      userID,
// 		BaitoUserID: utils.GenerateID(12),
// 		Name:        name,
// 		Age:         age,
// 		Phone:       phone,
// 		Location:    location,
// 		Preferred:   roles,
// 		Bio:         bio,
// 		ProfilePic:  profilePic,
// 		CreatedAt:   time.Now(),
// 	}

// 	if _, err := db.BaitoWorkerCollection.InsertOne(ctx, profile); err != nil {
// 		http.Error(w, "Failed to save worker profile", http.StatusInternalServerError)
// 		return
// 	}

// 	_, _ = db.UserCollection.UpdateOne(ctx,
// 		bson.M{"userid": userID},
// 		bson.M{
// 			"$addToSet": bson.M{"role": "worker"},
// 			"$set":      bson.M{"updated_at": time.Now()},
// 		},
// 	)

// 	utils.RespondWithJSON(w, http.StatusOK, utils.M{"message": "Worker profile created successfully"})
// }

// func CreateBaito(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	if err := r.ParseMultipartForm(20 << 20); err != nil {
// 		http.Error(w, "Invalid form", http.StatusBadRequest)
// 		return
// 	}

// 	b := models.Baito{
// 		Title:        strings.TrimSpace(r.FormValue("title")),
// 		Description:  strings.TrimSpace(r.FormValue("description")),
// 		Category:     strings.TrimSpace(r.FormValue("category")),
// 		SubCategory:  strings.TrimSpace(r.FormValue("subcategory")),
// 		Location:     strings.TrimSpace(r.FormValue("location")),
// 		Wage:         strings.TrimSpace(r.FormValue("wage")),
// 		Phone:        strings.TrimSpace(r.FormValue("phone")),
// 		Requirements: strings.TrimSpace(r.FormValue("requirements")),
// 		WorkHours:    strings.TrimSpace(r.FormValue("workHours")),
// 		OwnerID:      utils.GetUserIDFromRequest(r),
// 		CreatedAt:    time.Now(),
// 	}

// 	if b.Title == "" || b.Description == "" || b.Category == "" || b.SubCategory == "" ||
// 		b.Location == "" || b.Wage == "" || b.Phone == "" || b.Requirements == "" || b.WorkHours == "" {
// 		http.Error(w, "Missing required fields", http.StatusBadRequest)
// 		return
// 	}

// 	banner, _ := filemgr.SaveFormFile(r, "banner", uploadDir, false)
// 	if banner != "" {
// 		b.BannerURL = banner
// 	}

// 	images, _ := filemgr.SaveFormFiles(r, "images", uploadDir, false)
// 	if len(images) > 0 {
// 		b.Images = images
// 	}

// 	res, err := db.BaitoCollection.InsertOne(context.TODO(), b)
// 	if err != nil {
// 		http.Error(w, "Failed to save baito", http.StatusInternalServerError)
// 		return
// 	}

// 	b.ID = res.InsertedID.(primitive.ObjectID)
// 	utils.RespondWithJSON(w, http.StatusOK, utils.M{"baitoid": b.ID.Hex()})
// }

// func UpdateBaito(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	if err := r.ParseMultipartForm(20 << 20); err != nil {
// 		http.Error(w, "Invalid form", http.StatusBadRequest)
// 		return
// 	}

// 	baitoId := ps.ByName("id")
// 	objID, err := primitive.ObjectIDFromHex(baitoId)
// 	if err != nil {
// 		http.Error(w, "Invalid baito ID", http.StatusBadRequest)
// 		return
// 	}

// 	update := bson.M{
// 		"$set": bson.M{
// 			"title":        strings.TrimSpace(r.FormValue("title")),
// 			"description":  strings.TrimSpace(r.FormValue("description")),
// 			"category":     strings.TrimSpace(r.FormValue("category")),
// 			"subcategory":  strings.TrimSpace(r.FormValue("subcategory")),
// 			"location":     strings.TrimSpace(r.FormValue("location")),
// 			"wage":         strings.TrimSpace(r.FormValue("wage")),
// 			"phone":        strings.TrimSpace(r.FormValue("phone")),
// 			"requirements": strings.TrimSpace(r.FormValue("requirements")),
// 			"workHours":    strings.TrimSpace(r.FormValue("workHours")),
// 			"updatedAt":    time.Now(),
// 		},
// 	}

// 	if banner, _ := filemgr.SaveFormFile(r, "banner", uploadDir, false); banner != "" {
// 		update["$set"].(bson.M)["banner"] = banner
// 	}

// 	if images, _ := filemgr.SaveFormFiles(r, "images", uploadDir, false); len(images) > 0 {
// 		update["$set"].(bson.M)["images"] = images
// 	}

// 	filter := bson.M{
// 		"_id":     objID,
// 		"ownerId": utils.GetUserIDFromRequest(r),
// 	}

// 	result, err := db.BaitoCollection.UpdateOne(context.TODO(), filter, update)
// 	if err != nil {
// 		http.Error(w, "Failed to update baito", http.StatusInternalServerError)
// 		return
// 	}
// 	if result.MatchedCount == 0 {
// 		http.Error(w, "Baito not found or unauthorized", http.StatusNotFound)
// 		return
// 	}

// 	utils.RespondWithJSON(w, http.StatusOK, utils.M{
// 		"message": "Baito updated",
// 		"baitoid": baitoId,
// 	})
// }
