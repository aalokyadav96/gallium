package farms

import (
	"context"
	"naevis/db"
	"naevis/models"
	"naevis/utils"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func GetCropFarms(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	cropID := ps.ByName("cropid")
	skip, limit := utils.ParsePagination(r, 10, 100)
	sortBy := r.URL.Query().Get("sortBy")
	sortOrder := r.URL.Query().Get("sortOrder")
	breedFilter := strings.ToLower(r.URL.Query().Get("breed"))

	crops, err := utils.FindAndDecode[models.Crop](ctx, db.CropsCollection, bson.M{"cropid": cropID})
	if err != nil || len(crops) == 0 {
		utils.RespondWithError(w, http.StatusNotFound, "Crop not found")
		return
	}

	cropName := crops[0].Name
	cropCategory := crops[0].Category

	farmIDs := make([]string, len(crops))
	for i, c := range crops {
		farmIDs[i] = c.FarmID
	}
	farms, err := utils.FindAndDecode[models.Farm](ctx, db.FarmsCollection, bson.M{"farmid": bson.M{"$in": farmIDs}})
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch farms")
		return
	}

	farmMap := make(map[string]models.Farm)
	for _, f := range farms {
		farmMap[f.FarmID] = f
	}

	// type CropListing struct {
	// 	FarmID     string  `json:"farmId"`
	// 	FarmName   string  `json:"farmName"`
	// 	Location   string  `json:"location"`
	// 	Breed      string  `json:"breed"`
	// 	PricePerKg float64 `json:"pricePerKg"`
	// }
	var listings []models.CropListing
	for _, crop := range crops {
		if breedFilter != "" && strings.ToLower(crop.Notes) != breedFilter {
			continue
		}
		if farm, ok := farmMap[crop.FarmID]; ok {
			listings = append(listings, models.CropListing{
				FarmID:     crop.FarmID,
				FarmName:   farm.Name,
				Location:   farm.Location,
				Breed:      crop.Notes,
				PricePerKg: crop.Price,
				ImageURL:   crop.ImageURL,
			})
		}
	}

	switch sortBy {
	case "price":
		sort.Slice(listings, func(i, j int) bool {
			if sortOrder == "desc" {
				return listings[i].PricePerKg > listings[j].PricePerKg
			}
			return listings[i].PricePerKg < listings[j].PricePerKg
		})
	case "breed":
		sort.Slice(listings, func(i, j int) bool {
			if sortOrder == "desc" {
				return listings[i].Breed > listings[j].Breed
			}
			return listings[i].Breed < listings[j].Breed
		})
	}

	total := len(listings)
	if int(skip) > total {
		skip = int64(total)
	}
	end := int(skip) + int(limit)
	if end > total {
		end = total
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"name":     cropName,
		"category": cropCategory,
		"listings": listings[skip:end],
		"total":    total,
		"page":     int(skip/int64(limit)) + 1,
		"limit":    limit,
	})
}
func GetCropTypeFarms(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	cropName := ps.ByName("cropname")
	if cropName == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Missing crop name parameter")
		return
	}

	skip, limit := utils.ParsePagination(r, 10, 100)
	sortBy := r.URL.Query().Get("sortBy")
	sortOrder := r.URL.Query().Get("sortOrder")
	breedFilter := strings.ToLower(r.URL.Query().Get("breed"))

	filter := bson.M{"name": bson.M{"$regex": "^" + regexp.QuoteMeta(cropName) + "$", "$options": "i"}}
	crops, err := utils.FindAndDecode[models.Crop](ctx, db.CropsCollection, filter)
	if err != nil || len(crops) == 0 {
		utils.RespondWithError(w, http.StatusNotFound, "Crop type not found")
		return
	}

	cropCategory := crops[0].Category
	farmIDs := make([]string, len(crops))
	for i, c := range crops {
		farmIDs[i] = c.FarmID
	}

	farms, err := utils.FindAndDecode[models.Farm](ctx, db.FarmsCollection, bson.M{"farmid": bson.M{"$in": farmIDs}})
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch farms")
		return
	}

	farmMap := make(map[string]models.Farm)
	for _, f := range farms {
		farmMap[f.FarmID] = f
	}

	var listings []models.CropListing
	for _, crop := range crops {
		if breedFilter != "" && strings.ToLower(crop.Notes) != breedFilter {
			continue
		}
		if farm, ok := farmMap[crop.FarmID]; ok {
			var harvestDate string
			if crop.HarvestDate != nil {
				harvestDate = crop.HarvestDate.Format(time.RFC3339)
			}
			listings = append(listings, models.CropListing{
				FarmID:         crop.FarmID,
				FarmName:       farm.Name,
				Location:       farm.Location,
				Breed:          crop.Notes,
				PricePerKg:     crop.Price,
				AvailableQtyKg: crop.Quantity,
				HarvestDate:    harvestDate,
				Tags:           farm.Tags,
				ImageURL:       crop.ImageURL,
			})
		}
	}

	switch sortBy {
	case "price":
		sort.Slice(listings, func(i, j int) bool {
			if sortOrder == "desc" {
				return listings[i].PricePerKg > listings[j].PricePerKg
			}
			return listings[i].PricePerKg < listings[j].PricePerKg
		})
	case "breed":
		sort.Slice(listings, func(i, j int) bool {
			if sortOrder == "desc" {
				return listings[i].Breed > listings[j].Breed
			}
			return listings[i].Breed < listings[j].Breed
		})
	}

	total := len(listings)
	if int(skip) > total {
		skip = int64(total)
	}
	end := int(skip) + int(limit)
	if end > total {
		end = total
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"name":     cropName,
		"category": cropCategory,
		"listings": listings[skip:end],
		"total":    total,
		"page":     int(skip/int64(limit)) + 1,
		"limit":    limit,
	})
}

func GetPaginatedFarms(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	skip, limit := utils.ParsePagination(r, 10, 100)
	search := r.URL.Query().Get("search")

	matchStage := bson.D{}
	if search != "" {
		matchStage = bson.D{{Key: "$match", Value: bson.M{
			"$or": []bson.M{
				utils.RegexFilter("name", search),
				utils.RegexFilter("location", search),
				utils.RegexFilter("owner", search),
			},
		}}}
	}

	pipeline := mongo.Pipeline{}
	if len(matchStage) > 0 {
		pipeline = append(pipeline, matchStage)
	}
	pipeline = append(pipeline,
		bson.D{{Key: "$sort", Value: bson.M{"createdAt": -1}}},
		bson.D{{Key: "$lookup", Value: bson.M{
			"from":         "crops",
			"localField":   "farmid",
			"foreignField": "farmId",
			"as":           "crops",
		}}},
		bson.D{{Key: "$skip", Value: skip}},
		bson.D{{Key: "$limit", Value: limit}},
	)

	cursor, err := db.FarmsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Error fetching farms")
		return
	}
	defer cursor.Close(ctx)

	var farms []models.Farm
	if err := cursor.All(ctx, &farms); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Error decoding farms")
		return
	}

	total, _ := db.FarmsCollection.CountDocuments(ctx, bson.M{})
	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"farms":   farms,
		"total":   total,
		"page":    int(skip/int64(limit)) + 1,
		"limit":   limit,
	})
}
