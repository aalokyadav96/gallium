package baito

import (
	"log"
	"naevis/db"
	"naevis/filemgr"
	"naevis/models"
	"naevis/mq"
	"naevis/utils"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func parseBaitoForm(r *http.Request, isUpdate bool) (models.Baito, bson.M, error) {
	// ctx := r.Context()
	b := models.Baito{}
	update := bson.M{"$set": bson.M{}}

	if err := r.ParseMultipartForm(20 << 20); err != nil {
		return b, update, err
	}
	defer r.MultipartForm.RemoveAll()

	// Common fields
	title := strings.TrimSpace(r.FormValue("title"))
	description := strings.TrimSpace(r.FormValue("description"))
	category := strings.TrimSpace(r.FormValue("category"))
	subcategory := strings.TrimSpace(r.FormValue("subcategory"))
	location := strings.TrimSpace(r.FormValue("location"))
	wage := strings.TrimSpace(r.FormValue("wage"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	requirements := strings.TrimSpace(r.FormValue("requirements"))
	workHours := strings.TrimSpace(r.FormValue("workHours"))
	benefits := strings.TrimSpace(r.FormValue("benefits"))
	email := strings.TrimSpace(r.FormValue("email"))
	tagsStr := strings.TrimSpace(r.FormValue("tags"))

	var tags []string
	if tagsStr != "" {
		for _, t := range strings.Split(tagsStr, ",") {
			if trimmed := strings.TrimSpace(t); trimmed != "" {
				tags = append(tags, trimmed)
			}
		}
	}

	// File uploads
	banner, _ := filemgr.SaveFormFile(r.MultipartForm, "banner", filemgr.EntityBaito, filemgr.PicBanner, false)
	images, _ := filemgr.SaveFormFiles(r.MultipartForm, "images", filemgr.EntityBaito, filemgr.PicPhoto, false)

	if isUpdate {
		set := update["$set"].(bson.M)
		set["title"] = title
		set["description"] = description
		set["category"] = category
		set["subcategory"] = subcategory
		set["location"] = location
		set["wage"] = wage
		set["phone"] = phone
		set["requirements"] = requirements
		set["workHours"] = workHours
		set["benefits"] = benefits
		set["email"] = email
		set["tags"] = tags
		set["updatedAt"] = time.Now()

		if banner != "" {
			set["bannerURL"] = banner
		}
		if len(images) > 0 {
			set["images"] = images
		}
	} else {
		b = models.Baito{
			BaitoId:      utils.GenerateRandomString(15),
			Title:        title,
			Description:  description,
			Category:     category,
			SubCategory:  subcategory,
			Location:     location,
			Wage:         wage,
			Phone:        phone,
			Requirements: requirements,
			WorkHours:    workHours,
			Benefits:     benefits,
			Email:        email,
			Tags:         tags,
			OwnerID:      utils.GetUserIDFromRequest(r),
			CreatedAt:    time.Now(),
		}
		if banner != "" {
			b.BannerURL = banner
		}
		if len(images) > 0 {
			b.Images = images
		}
	}

	return b, update, nil
}

func CreateBaito(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	b, _, err := parseBaitoForm(r, false)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid form data")
		return
	}

	if b.Title == "" || b.Description == "" || b.Category == "" || b.SubCategory == "" ||
		b.Location == "" || b.Wage == "" || b.Phone == "" || b.Requirements == "" || b.WorkHours == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Missing required fields")
		return
	}

	if _, err := db.BaitoCollection.InsertOne(ctx, b); err != nil {
		log.Printf("Insert error: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save baito")
		return
	}

	go mq.Emit(ctx, "baito-created", models.Index{
		EntityType: "baito", EntityId: b.BaitoId, Method: "POST",
	})
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"baitoid": b.BaitoId})
}

func UpdateBaito(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	_, update, err := parseBaitoForm(r, true)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid form data")
		return
	}

	filter := bson.M{
		"baitoid": ps.ByName("baitoid"),
		"ownerId": utils.GetUserIDFromRequest(r),
	}

	result, err := db.BaitoCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		log.Printf("Update error: %v", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update baito")
		return
	}
	if result.MatchedCount == 0 {
		utils.RespondWithError(w, http.StatusNotFound, "Baito not found or unauthorized")
		return
	}

	go mq.Emit(ctx, "baito-updated", models.Index{
		EntityType: "baito", EntityId: ps.ByName("baitoid"), Method: "PUT",
	})
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Baito updated",
		"baitoid": ps.ByName("baitoid"),
	})
}
