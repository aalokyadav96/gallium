package farms

import (
	"net/http"
	"strings"
	"time"

	"naevis/db"
	"naevis/filemgr"
	"naevis/globals"
	"naevis/models"
	"naevis/mq"
	"naevis/utils"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func getUserIDFromContext(r *http.Request) (string, bool) {
	userID, ok := r.Context().Value(globals.UserIDKey).(string)
	return userID, ok
}

func AddCrop(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	farmID := ps.ByName("id")
	userid := utils.GetUserIDFromRequest(r)
	if farmID == "" {
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Invalid farm ID"})
		return
	}

	if _, ok := getUserIDFromContext(r); !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Invalid form"})
		return
	}

	name := r.FormValue("name")
	if name == "" {
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Name is required"})
		return
	}

	crop := parseCropForm(r)
	crop.FarmID = farmID
	crop.CreatedBy = userid

	filename, err := filemgr.SaveFormFile(r.MultipartForm, "image", filemgr.EntityCrop, filemgr.PicPhoto, false)
	if err == nil && filename != "" {
		crop.ImageURL = filename
	}

	_, err = db.CropsCollection.InsertOne(ctx, crop)
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Insert failed"})
		return
	}

	go mq.Emit(ctx, "crop-created", models.Index{
		EntityType: "crop", EntityId: crop.CropId, Method: "POST",
	})
	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true, "cropId": crop.CropId})
}

func EditCrop(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	cropID := ps.ByName("cropid")

	if _, ok := getUserIDFromContext(r); !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Invalid form"})
		return
	}

	update := bson.M{"updatedAt": time.Now()}

	if v := r.FormValue("name"); v != "" {
		update["name"] = v
	}
	if v := r.FormValue("unit"); v != "" {
		update["unit"] = v
	}
	if v := r.FormValue("price"); v != "" {
		update["price"] = utils.ParseFloat(v)
	}
	if v := r.FormValue("quantity"); v != "" {
		update["quantity"] = utils.ParseInt(v)
	}
	if v := r.FormValue("notes"); v != "" {
		update["notes"] = v
	}
	if v := r.FormValue("category"); v != "" {
		update["category"] = v
	}
	if v := r.FormValue("featured"); v != "" {
		update["featured"] = v == "true"
	}
	if v := r.FormValue("outOfStock"); v != "" {
		update["outOfStock"] = v == "true"
	}

	if d := utils.ParseDate(r.FormValue("harvestDate")); d != nil {
		update["harvestDate"] = d
	}
	if d := utils.ParseDate(r.FormValue("expiryDate")); d != nil {
		update["expiryDate"] = d
	}

	if filename, err := filemgr.SaveFormFile(r.MultipartForm, "image", filemgr.EntityCrop, filemgr.PicPhoto, false); err == nil && filename != "" {
		update["imageUrl"] = filename
	}

	if len(update) <= 1 { // only updatedAt present
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "No valid fields to update"})
		return
	}

	_, err := db.CropsCollection.UpdateOne(ctx, bson.M{"cropid": cropID}, bson.M{"$set": update})
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false})
		return
	}

	go mq.Emit(ctx, "crop-updated", models.Index{
		EntityType: "crop", EntityId: cropID, Method: "PUT",
	})
	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true})
}

func parseCropForm(r *http.Request) models.Crop {
	cropName := r.FormValue("name")
	formatted := strings.ToLower(strings.ReplaceAll(cropName, " ", "_"))
	crop := models.Crop{
		Name:       r.FormValue("name"),
		Price:      utils.ParseFloat(r.FormValue("price")),
		Quantity:   utils.ParseInt(r.FormValue("quantity")),
		Unit:       r.FormValue("unit"),
		Notes:      r.FormValue("notes"),
		Category:   r.FormValue("category"),
		Featured:   r.FormValue("featured") == "true",
		OutOfStock: r.FormValue("outOfStock") == "true",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		CropId:     formatted,
	}

	if d := utils.ParseDate(r.FormValue("harvestDate")); d != nil {
		crop.HarvestDate = d
	}
	if d := utils.ParseDate(r.FormValue("expiryDate")); d != nil {
		crop.ExpiryDate = d
	}
	return crop
}

func DeleteCrop(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	cropID := ps.ByName("cropid")

	if _, ok := getUserIDFromContext(r); !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	res, err := db.CropsCollection.DeleteOne(ctx, bson.M{"cropid": cropID})
	if err != nil || res.DeletedCount == 0 {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to delete crop"})
		return
	}

	go mq.Emit(ctx, "crop-deleted", models.Index{
		EntityType: "crop", EntityId: cropID, Method: "DELETE",
	})
	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true})
}
