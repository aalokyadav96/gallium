package farms

import (
	"context"
	"naevis/db"
	"naevis/models"
	"naevis/utils"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func GetFarmDash(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// id, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	// if err != nil {
	// 	utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Invalid farm ID"})
	// 	return
	// }
	userid := utils.GetUserIDFromRequest(r)

	var farm models.Farm
	if err := db.FarmsCollection.FindOne(context.Background(), bson.M{"createdBy": userid}).Decode(&farm); err != nil {
		utils.RespondWithJSON(w, http.StatusNotFound, utils.M{"success": false, "message": "Farm not found"})
		return
	}

	// Fetch crops from separate crops collection
	cursor, err := db.CropsCollection.Find(context.Background(), bson.M{"farmId": farm.FarmID})
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to load crops"})
		return
	}
	defer cursor.Close(context.Background())

	var crops []models.Crop
	if err := cursor.All(context.Background(), &crops); err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to decode crops"})
		return
	}

	farm.Crops = crops

	utils.RespondWithJSON(w, http.StatusOK, utils.M{
		"success": true,
		"farm":    farm,
	})
}
