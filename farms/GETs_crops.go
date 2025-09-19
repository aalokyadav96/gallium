package farms

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"naevis/db"
	"naevis/models"
	"naevis/rdx"
	"naevis/utils"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Filtered Crops
func GetFilteredCrops(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	params := r.URL.Query()
	filter := bson.M{}

	if c := params.Get("category"); c != "" {
		filter["category"] = c
	}
	if region := params.Get("region"); region != "" {
		filter["farmLocation"] = region
	}
	if params.Get("inStock") == "true" {
		filter["quantity"] = bson.M{"$gt": 0}
	}

	price := bson.M{}
	if min := utils.ParseFloat(params.Get("minPrice")); min > 0 {
		price["$gte"] = min
	}
	if max := utils.ParseFloat(params.Get("maxPrice")); max > 0 {
		price["$lte"] = max
	}
	if len(price) > 0 {
		filter["price"] = price
	}

	crops, err := utils.FindAndDecode[models.Crop](ctx, db.CropsCollection, filter)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch crops")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{"success": true, "crops": crops})
}

func GetPreCropCatalogue(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	const redisKey = "crop_catalogue"
	var crops []models.CropCatalogueItem

	if val, err := rdx.Conn.Get(ctx, redisKey).Result(); err == nil && val != "" {
		if err := json.Unmarshal([]byte(val), &crops); err == nil {
			utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true, "crops": crops})
			return
		}
	}

	cursor, err := db.CatalogueCollection.Find(ctx, bson.M{})
	if err == nil {
		if err := cursor.All(ctx, &crops); err == nil && len(crops) > 0 {
			if jsonBytes, err := json.Marshal(crops); err == nil {
				_ = rdx.Conn.Set(ctx, redisKey, jsonBytes, 2*time.Hour).Err()
			}
			utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true, "crops": crops})
			return
		}
	}

	file, err := os.Open("data/pre_crop_catalogue.csv")
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to retrieve catalogue"})
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	headers, err := reader.Read()
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Invalid CSV"})
		return
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(record) != len(headers) {
			continue
		}

		entry := models.CropCatalogueItem{}
		for i, field := range headers {
			switch strings.ToLower(field) {
			case "name":
				entry.Name = record[i]
			case "category":
				entry.Category = record[i]
			case "imageurl":
				entry.ImageURL = record[i]
			case "stock":
				entry.Stock, _ = strconv.Atoi(record[i])
			case "unit":
				entry.Unit = record[i]
			case "featured":
				entry.Featured = strings.ToLower(record[i]) == "true"
			case "pricerange":
				parts := strings.Split(record[i], "-")
				if len(parts) == 2 {
					min, _ := strconv.Atoi(parts[0])
					max, _ := strconv.Atoi(parts[1])
					entry.PriceRange = []int{min, max}
				}
			}
		}
		crops = append(crops, entry)
	}

	if jsonBytes, err := json.Marshal(crops); err == nil {
		_ = rdx.Conn.Set(ctx, redisKey, jsonBytes, 2*time.Hour).Err()
	}

	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true, "crops": crops})
}

func GetCropCatalogue(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	cursor, err := db.CropsCollection.Find(ctx, bson.M{})
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to fetch crop catalogue"})
		return
	}
	defer cursor.Close(ctx)

	var allCrops []models.Crop
	if err := cursor.All(ctx, &allCrops); err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to decode crops"})
		return
	}

	seen := make(map[string]bool)
	uniqueCrops := []models.Crop{}
	for _, crop := range allCrops {
		key := strings.ToLower(crop.Name + crop.CatalogueId)
		if !seen[key] {
			seen[key] = true
			uniqueCrops = append(uniqueCrops, crop)
		}
	}

	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true, "crops": uniqueCrops})
}

func GetCropTypes(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()

	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":      "$name", // group by crop name
			"minPrice": bson.M{"$min": "$price"},
			"maxPrice": bson.M{"$max": "$price"},
			"availableCount": bson.M{
				"$sum": bson.M{
					"$cond": []interface{}{
						bson.M{"$gt": []interface{}{"$quantity", 0}}, 1, 0,
					},
				},
			},
			"imageUrl": bson.M{"$first": "$imageUrl"}, // match actual DB field name
			"unit":     bson.M{"$first": "$unit"},
		}}},
		{{Key: "$project", Value: bson.M{
			"name":           "$_id",
			"minPrice":       1,
			"maxPrice":       1,
			"availableCount": 1,
			"imageUrl":       1,
			"unit":           1,
			"_id":            0,
		}}},
		{{Key: "$sort", Value: bson.M{"name": 1}}},
	}

	cursor, err := db.CropsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "error": err.Error()})
		return
	}
	var cropTypes []bson.M
	if err := cursor.All(ctx, &cropTypes); err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "error": err.Error()})
		return
	}

	if len(cropTypes) == 0 {
		cropTypes = []bson.M{}
	}

	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true, "cropTypes": cropTypes})
}
