package merch

import (
	"context"
	"naevis/db"
	"naevis/models"
	"naevis/utils"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// Merch
func GetMerchs(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	eventID := ps.ByName("eventid")
	entityType := ps.ByName("entityType")
	// cacheKey := fmt.Sprintf("merchlist:%s", eventID)

	// if cached, _ := rdx.RdxGet(cacheKey); cached != "" {
	// 	w.Header().Set("Content-Type", "application/json")
	// 	w.Write([]byte(cached))
	// 	return
	// }

	filter := bson.M{"entity_type": entityType, "entity_id": eventID}
	merchList, err := utils.FindAndDecode[models.Merch](ctx, db.MerchCollection, filter)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch merchandise")
		return
	}

	// Ensure we return [] instead of null
	if merchList == nil {
		merchList = []models.Merch{}
	}

	// data := utils.ToJSON(merchList)
	// rdx.RdxSet(cacheKey, string(data))

	utils.RespondWithJSON(w, http.StatusOK, merchList)
}
