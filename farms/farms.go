package farms

import (
	"encoding/json"
	"net/http"
	"os"
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

func GetFarm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	id := ps.ByName("id")

	var farm models.Farm
	if err := db.FarmsCollection.FindOne(ctx, bson.M{"farmid": id}).Decode(&farm); err != nil {
		utils.RespondWithJSON(w, http.StatusNotFound, utils.M{"success": false, "message": "Farm not found"})
		return
	}

	cursor, err := db.CropsCollection.Find(ctx, bson.M{"farmId": id})
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to load crops"})
		return
	}
	defer cursor.Close(ctx)

	var crops []models.Crop
	if err := cursor.All(ctx, &crops); err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to decode crops"})
		return
	}

	farm.Crops = crops

	utils.RespondWithJSON(w, http.StatusOK, utils.M{
		"success": true,
		"farm":    farm,
	})
}

func CreateFarm(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Failed to parse form"})
		return
	}

	requestingUserID, ok := ctx.Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	farmID := utils.GenerateRandomString(14)

	farm := models.Farm{
		FarmID:             farmID,
		Name:               r.FormValue("name"),
		Location:           r.FormValue("location"),
		Description:        r.FormValue("description"),
		Owner:              r.FormValue("owner"),
		Contact:            r.FormValue("contact"),
		AvailabilityTiming: r.FormValue("availabilityTiming"),
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
		Crops:              []models.Crop{},
		CreatedBy:          requestingUserID,
	}

	if farm.Name == "" || farm.Location == "" || farm.Owner == "" || farm.Contact == "" {
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Missing required fields"})
		return
	}

	fileName, err := filemgr.SaveFormFile(r.MultipartForm, "photo", filemgr.EntityFarm, filemgr.PicBanner, false)
	if err == nil && fileName != "" {
		farm.Banner = fileName
	}

	if _, err := db.FarmsCollection.InsertOne(ctx, farm); err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to insert farm"})
		return
	}

	go mq.Emit(ctx, "farm-created", models.Index{EntityType: "farm", EntityId: farm.FarmID, Method: "POST"})

	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true, "id": farm.FarmID})
}

func EditFarm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	farmID := ps.ByName("id")

	requestingUserID, ok := ctx.Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}
	_ = requestingUserID

	updateFields := bson.M{}
	contentType := r.Header.Get("Content-Type")

	var input models.Farm

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Malformed multipart data"})
			return
		}

		input.Name = r.FormValue("name")
		input.Location = r.FormValue("location")
		input.Description = r.FormValue("description")
		input.Owner = r.FormValue("owner")
		input.Contact = r.FormValue("contact")
		input.AvailabilityTiming = r.FormValue("availabilityTiming")

		fileName, err := filemgr.SaveFormFile(r.MultipartForm, "banner", filemgr.EntityFarm, filemgr.PicBanner, false)
		if err == nil && fileName != "" {
			updateFields["banner"] = fileName
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Invalid JSON body"})
			return
		}
	}

	if input.Name != "" {
		updateFields["name"] = input.Name
	}
	if input.Location != "" {
		updateFields["location"] = input.Location
	}
	if input.Description != "" {
		updateFields["description"] = input.Description
	}
	if input.Owner != "" {
		updateFields["owner"] = input.Owner
	}
	if input.Contact != "" {
		updateFields["contact"] = input.Contact
	}
	if input.AvailabilityTiming != "" {
		updateFields["availabilityTiming"] = input.AvailabilityTiming
	}

	if len(updateFields) == 0 {
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "No fields to update"})
		return
	}

	updateFields["updatedAt"] = time.Now()

	if _, err := db.FarmsCollection.UpdateOne(ctx, bson.M{"farmid": farmID}, bson.M{"$set": updateFields}); err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Database error"})
		return
	}

	go mq.Emit(ctx, "farm-updated", models.Index{EntityType: "farm", EntityId: farmID, Method: "PUT"})

	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true, "message": "Farm updated"})
}

func DeleteFarm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	farmID := ps.ByName("id")

	requestingUserID, ok := ctx.Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}
	_ = requestingUserID

	var farm models.Farm
	if err := db.FarmsCollection.FindOne(ctx, bson.M{"farmid": farmID}).Decode(&farm); err != nil {
		utils.RespondWithJSON(w, http.StatusNotFound, utils.M{"success": false, "message": "Not found"})
		return
	}

	if _, err := db.FarmsCollection.DeleteOne(ctx, bson.M{"farmid": farmID}); err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false})
		return
	}

	if farm.Banner != "" {
		if err := os.Remove("." + farm.Banner); err != nil {
			// log error if needed
		}
	}

	go mq.Emit(ctx, "farm-deleted", models.Index{EntityType: "farm", EntityId: farmID, Method: "DELETE"})
	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true})
}
