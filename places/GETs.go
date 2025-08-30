package places

import (
	"context"
	"encoding/json"
	"naevis/db"
	"naevis/models"
	"naevis/utils"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// Places
func GetPlaces(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	places, err := utils.FindAndDecode[models.Place](ctx, db.PlacesCollection, bson.M{})
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch places")
		return
	}

	var result []models.PlacesResponse
	for _, p := range places {
		desc := p.Description
		if len(desc) > 60 {
			desc = desc[:60] + "..."
		}

		tags := p.Tags
		if len(tags) > 5 {
			tags = tags[:5]
		}

		result = append(result, models.PlacesResponse{
			PlaceID:        p.PlaceID,
			Name:           p.Name,
			ShortDesc:      desc,
			Address:        p.Address,
			Distance:       p.Distance,
			OperatingHours: p.OperatingHours,
			Category:       p.Category,
			Tags:           tags,
			Banner:         p.Banner,
		})
	}

	data, err := json.Marshal(result)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to encode places")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
