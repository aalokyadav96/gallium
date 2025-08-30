package farms

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
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Items
func GetItems(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	filter := bson.M{}
	if t := r.URL.Query().Get("type"); t != "" {
		filter["type"] = t
	}
	if c := r.URL.Query().Get("category"); c != "" {
		filter["category"] = c
	}
	if s := r.URL.Query().Get("search"); s != "" {
		filter["name"] = utils.RegexFilter("name", s)["name"]
	}

	skip, limit := utils.ParsePagination(r, 10, 100)
	sort := utils.ParseSort(r.URL.Query().Get("sort"), bson.D{{Key: "name", Value: 1}}, map[string]bson.D{
		"price_asc":  {{Key: "price", Value: 1}},
		"price_desc": {{Key: "price", Value: -1}},
		"name_asc":   {{Key: "name", Value: 1}},
		"name_desc":  {{Key: "name", Value: -1}},
	})

	opts := options.Find().SetSort(sort).SetSkip(skip).SetLimit(limit)
	items, err := utils.FindAndDecode[models.Product](ctx, db.ProductCollection, filter, opts)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch items")
		return
	}

	total, err := db.ProductCollection.CountDocuments(ctx, filter)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to count items")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"items": items,
		"total": total,
	})
}

func GetItemCategories(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	itemType := r.URL.Query().Get("type")

	var categories []string

	switch itemType {
	case "tool":
		categories = []string{
			"Cutting Tools",
			"Irrigation Tools",
			"Harvesting Tools",
			"Hand Tools",
			"Protective Gear",
			"Fertilizer Applicators",
		}
	case "product":
		fallthrough
	default:
		categories = []string{
			"Spices",
			"Pickles",
			"Flour",
			"Oils",
			"Honey",
			"Tea & Coffee",
			"Dry Fruits",
			"Natural Sweeteners",
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(categories); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
