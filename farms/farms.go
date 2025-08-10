package farms

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"sort"
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
	"go.mongodb.org/mongo-driver/mongo"
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

	fileName, err := filemgr.SaveFormFile(r.MultipartForm, "photo", filemgr.EntityFarm, filemgr.PicPhoto, false)
	if err == nil && fileName != "" {
		farm.Photo = fileName
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

		fileName, err := filemgr.SaveFormFile(r.MultipartForm, "photo", filemgr.EntityFarm, filemgr.PicPhoto, false)
		if err == nil && fileName != "" {
			updateFields["photo"] = fileName
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

	if farm.Photo != "" {
		if err := os.Remove("." + farm.Photo); err != nil {
			// log error if needed
		}
	}

	go mq.Emit(ctx, "farm-deleted", models.Index{EntityType: "farm", EntityId: farmID, Method: "DELETE"})
	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true})
}
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
		utils.Error(w, http.StatusNotFound, "Crop not found")
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
		utils.Error(w, http.StatusInternalServerError, "Failed to fetch farms")
		return
	}

	farmMap := make(map[string]models.Farm)
	for _, f := range farms {
		farmMap[f.FarmID] = f
	}

	type CropListing struct {
		FarmID     string  `json:"farmId"`
		FarmName   string  `json:"farmName"`
		Location   string  `json:"location"`
		Breed      string  `json:"breed"`
		PricePerKg float64 `json:"pricePerKg"`
	}
	var listings []CropListing
	for _, crop := range crops {
		if breedFilter != "" && strings.ToLower(crop.Notes) != breedFilter {
			continue
		}
		if farm, ok := farmMap[crop.FarmID]; ok {
			listings = append(listings, CropListing{
				FarmID:     crop.FarmID,
				FarmName:   farm.Name,
				Location:   farm.Location,
				Breed:      crop.Notes,
				PricePerKg: crop.Price,
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

	utils.JSON(w, http.StatusOK, map[string]any{
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
		utils.Error(w, http.StatusBadRequest, "Missing crop name parameter")
		return
	}

	skip, limit := utils.ParsePagination(r, 10, 100)
	sortBy := r.URL.Query().Get("sortBy")
	sortOrder := r.URL.Query().Get("sortOrder")
	breedFilter := strings.ToLower(r.URL.Query().Get("breed"))

	filter := bson.M{"name": bson.M{"$regex": "^" + regexp.QuoteMeta(cropName) + "$", "$options": "i"}}
	crops, err := utils.FindAndDecode[models.Crop](ctx, db.CropsCollection, filter)
	if err != nil || len(crops) == 0 {
		utils.Error(w, http.StatusNotFound, "Crop type not found")
		return
	}

	cropCategory := crops[0].Category
	farmIDs := make([]string, len(crops))
	for i, c := range crops {
		farmIDs[i] = c.FarmID
	}

	farms, err := utils.FindAndDecode[models.Farm](ctx, db.FarmsCollection, bson.M{"farmid": bson.M{"$in": farmIDs}})
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "Failed to fetch farms")
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

	utils.JSON(w, http.StatusOK, map[string]any{
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
		utils.Error(w, http.StatusInternalServerError, "Error fetching farms")
		return
	}
	defer cursor.Close(ctx)

	var farms []models.Farm
	if err := cursor.All(ctx, &farms); err != nil {
		utils.Error(w, http.StatusInternalServerError, "Error decoding farms")
		return
	}

	total, _ := db.FarmsCollection.CountDocuments(ctx, bson.M{})
	utils.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"farms":   farms,
		"total":   total,
		"page":    int(skip/int64(limit)) + 1,
		"limit":   limit,
	})
}

// func GetCropFarms(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
// 	defer cancel()

// 	cropID := ps.ByName("cropid")

// 	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
// 	if page < 1 {
// 		page = 1
// 	}
// 	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
// 	if limit < 1 {
// 		limit = 10
// 	}
// 	skip := (page - 1) * limit

// 	sortBy := r.URL.Query().Get("sortBy")
// 	sortOrder := r.URL.Query().Get("sortOrder")
// 	breedFilter := strings.ToLower(r.URL.Query().Get("breed"))

// 	cursor, err := db.CropsCollection.Find(ctx, bson.M{"cropid": cropID})
// 	if err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to fetch crop data"})
// 		return
// 	}
// 	defer cursor.Close(ctx)

// 	var cropInstances []models.Crop
// 	if err := cursor.All(ctx, &cropInstances); err != nil || len(cropInstances) == 0 {
// 		utils.RespondWithJSON(w, http.StatusNotFound, utils.M{"success": false, "message": "Crop not found"})
// 		return
// 	}

// 	cropName := cropInstances[0].Name
// 	cropCategory := cropInstances[0].Category

// 	farmIDs := make([]string, len(cropInstances))
// 	for i, crop := range cropInstances {
// 		farmIDs[i] = crop.FarmID
// 	}

// 	farmCursor, err := db.FarmsCollection.Find(ctx, bson.M{"farmid": bson.M{"$in": farmIDs}})
// 	if err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to fetch farms"})
// 		return
// 	}
// 	defer farmCursor.Close(ctx)

// 	var farms []models.Farm
// 	if err := farmCursor.All(ctx, &farms); err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to decode farms"})
// 		return
// 	}

// 	farmMap := make(map[string]models.Farm)
// 	for _, f := range farms {
// 		farmMap[f.FarmID] = f
// 	}

// 	type CropListing struct {
// 		FarmID     string  `json:"farmId"`
// 		FarmName   string  `json:"farmName"`
// 		Location   string  `json:"location"`
// 		Breed      string  `json:"breed"`
// 		PricePerKg float64 `json:"pricePerKg"`
// 	}

// 	var listings []CropListing
// 	for _, crop := range cropInstances {
// 		if breedFilter != "" && strings.ToLower(crop.Notes) != breedFilter {
// 			continue
// 		}
// 		if farm, ok := farmMap[crop.FarmID]; ok {
// 			listings = append(listings, CropListing{
// 				FarmID:     crop.FarmID,
// 				FarmName:   farm.Name,
// 				Location:   farm.Location,
// 				Breed:      crop.Notes,
// 				PricePerKg: crop.Price,
// 			})
// 		}
// 	}

// 	switch sortBy {
// 	case "price":
// 		sort.Slice(listings, func(i, j int) bool {
// 			if sortOrder == "desc" {
// 				return listings[i].PricePerKg > listings[j].PricePerKg
// 			}
// 			return listings[i].PricePerKg < listings[j].PricePerKg
// 		})
// 	case "breed":
// 		sort.Slice(listings, func(i, j int) bool {
// 			if sortOrder == "desc" {
// 				return listings[i].Breed > listings[j].Breed
// 			}
// 			return listings[i].Breed < listings[j].Breed
// 		})
// 	}

// 	total := len(listings)
// 	end := skip + limit
// 	if skip > total {
// 		skip = total
// 	}
// 	if end > total {
// 		end = total
// 	}
// 	paginated := listings[skip:end]

// 	utils.RespondWithJSON(w, http.StatusOK, utils.M{
// 		"success":  true,
// 		"name":     cropName,
// 		"category": cropCategory,
// 		"listings": paginated,
// 		"total":    total,
// 		"page":     page,
// 		"limit":    limit,
// 	})
// }

// func GetCropTypeFarms(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
// 	defer cancel()

// 	cropName := ps.ByName("cropname")
// 	if cropName == "" {
// 		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Missing crop name parameter"})
// 		return
// 	}

// 	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
// 	if page < 1 {
// 		page = 1
// 	}
// 	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
// 	if limit < 1 {
// 		limit = 10
// 	}
// 	skip := (page - 1) * limit

// 	sortBy := r.URL.Query().Get("sortBy")
// 	sortOrder := r.URL.Query().Get("sortOrder")
// 	breedFilter := strings.ToLower(r.URL.Query().Get("breed"))

// 	filter := bson.M{
// 		"name": bson.M{"$regex": primitive.Regex{Pattern: "^" + regexp.QuoteMeta(cropName) + "$", Options: "i"}},
// 	}
// 	cursor, err := db.CropsCollection.Find(ctx, filter)
// 	if err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to fetch crop data"})
// 		return
// 	}
// 	defer cursor.Close(ctx)

// 	var cropInstances []models.Crop
// 	if err := cursor.All(ctx, &cropInstances); err != nil || len(cropInstances) == 0 {
// 		utils.RespondWithJSON(w, http.StatusNotFound, utils.M{"success": false, "message": "Crop type not found"})
// 		return
// 	}

// 	cropCategory := cropInstances[0].Category

// 	farmIDs := make([]string, len(cropInstances))
// 	for i, crop := range cropInstances {
// 		farmIDs[i] = crop.FarmID
// 	}

// 	farmCursor, err := db.FarmsCollection.Find(ctx, bson.M{"farmid": bson.M{"$in": farmIDs}})
// 	if err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to fetch farms"})
// 		return
// 	}
// 	defer farmCursor.Close(ctx)

// 	var farms []models.Farm
// 	if err := farmCursor.All(ctx, &farms); err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to decode farms"})
// 		return
// 	}

// 	farmMap := make(map[string]models.Farm)
// 	for _, f := range farms {
// 		farmMap[f.FarmID] = f
// 	}

// 	var listings []models.CropListing
// 	for _, crop := range cropInstances {
// 		if breedFilter != "" && strings.ToLower(crop.Notes) != breedFilter {
// 			continue
// 		}
// 		if farm, ok := farmMap[crop.FarmID]; ok {
// 			var harvestDate string
// 			if crop.HarvestDate != nil {
// 				harvestDate = crop.HarvestDate.Format(time.RFC3339)
// 			}
// 			listings = append(listings, models.CropListing{
// 				FarmID:         crop.FarmID,
// 				FarmName:       farm.Name,
// 				Location:       farm.Location,
// 				Breed:          crop.Notes,
// 				PricePerKg:     crop.Price,
// 				AvailableQtyKg: crop.Quantity,
// 				HarvestDate:    harvestDate,
// 				Tags:           farm.Tags,
// 			})
// 		}
// 	}

// 	switch sortBy {
// 	case "price":
// 		sort.Slice(listings, func(i, j int) bool {
// 			if sortOrder == "desc" {
// 				return listings[i].PricePerKg > listings[j].PricePerKg
// 			}
// 			return listings[i].PricePerKg < listings[j].PricePerKg
// 		})
// 	case "breed":
// 		sort.Slice(listings, func(i, j int) bool {
// 			if sortOrder == "desc" {
// 				return listings[i].Breed > listings[j].Breed
// 			}
// 			return listings[i].Breed < listings[j].Breed
// 		})
// 	}

// 	total := len(listings)
// 	end := skip + limit
// 	if skip > total {
// 		skip = total
// 	}
// 	if end > total {
// 		end = total
// 	}
// 	paginated := listings[skip:end]

// 	utils.RespondWithJSON(w, http.StatusOK, utils.M{
// 		"success":  true,
// 		"name":     cropName,
// 		"category": cropCategory,
// 		"listings": paginated,
// 		"total":    total,
// 		"page":     page,
// 		"limit":    limit,
// 	})
// }

// func GetPaginatedFarms(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
// 	defer cancel()

// 	page := utils.ParseInt(r.URL.Query().Get("page"))
// 	if page <= 0 {
// 		page = 1
// 	}
// 	limit := utils.ParseInt(r.URL.Query().Get("limit"))
// 	if limit <= 0 || limit > 100 {
// 		limit = 10
// 	}

// 	skip := (page - 1) * limit

// 	matchStage := bson.D{}
// 	search := r.URL.Query().Get("search")
// 	if search != "" {
// 		matchStage = bson.D{{Key: "$match", Value: bson.M{
// 			"$or": []bson.M{
// 				{"name": bson.M{"$regex": search, "$options": "i"}},
// 				{"location": bson.M{"$regex": search, "$options": "i"}},
// 				{"owner": bson.M{"$regex": search, "$options": "i"}},
// 			},
// 		}}}
// 	}

// 	sortStage := bson.D{{Key: "$sort", Value: bson.M{"createdAt": -1}}}

// 	lookupStage := bson.D{{Key: "$lookup", Value: bson.M{
// 		"from":         "crops",
// 		"localField":   "farmid",
// 		"foreignField": "farmId",
// 		"as":           "crops",
// 	}}}

// 	skipStage := bson.D{{Key: "$skip", Value: skip}}
// 	limitStage := bson.D{{Key: "$limit", Value: limit}}

// 	pipeline := mongo.Pipeline{}
// 	if len(matchStage) > 0 {
// 		pipeline = append(pipeline, matchStage)
// 	}
// 	pipeline = append(pipeline, sortStage, lookupStage, skipStage, limitStage)

// 	cursor, err := db.FarmsCollection.Aggregate(ctx, pipeline)
// 	if err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false})
// 		return
// 	}
// 	defer cursor.Close(ctx)

// 	var farms []models.Farm
// 	if err := cursor.All(ctx, &farms); err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false})
// 		return
// 	}

// 	total, err := db.FarmsCollection.CountDocuments(ctx, bson.M{})
// 	if err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false})
// 		return
// 	}

// 	utils.RespondWithJSON(w, http.StatusOK, utils.M{
// 		"success": true,
// 		"farms":   farms,
// 		"total":   total,
// 		"page":    page,
// 		"limit":   limit,
// 	})
// }

// package farms

// import (
// 	"context"
// 	"encoding/json"
// 	"net/http"
// 	"os"
// 	"regexp"
// 	"sort"
// 	"strconv"
// 	"strings"
// 	"time"

// 	"naevis/db"
// 	"naevis/filemgr"
// 	"naevis/globals"
// 	"naevis/models"
// 	"naevis/mq"
// 	"naevis/utils"

// 	"github.com/julienschmidt/httprouter"
// 	"go.mongodb.org/mongo-driver/bson"
// 	"go.mongodb.org/mongo-driver/bson/primitive"
// 	"go.mongodb.org/mongo-driver/mongo"
// )

// func GetFarm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	id := ps.ByName("id")

// 	var farm models.Farm
// 	if err := db.FarmsCollection.FindOne(context.Background(), bson.M{"farmid": id}).Decode(&farm); err != nil {
// 		utils.RespondWithJSON(w, http.StatusNotFound, utils.M{"success": false, "message": "Farm not found"})
// 		return
// 	}

// 	// Fetch crops from separate crops collection
// 	cursor, err := db.CropsCollection.Find(context.Background(), bson.M{"farmId": id})
// 	if err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to load crops"})
// 		return
// 	}
// 	defer cursor.Close(context.Background())

// 	var crops []models.Crop
// 	if err := cursor.All(context.Background(), &crops); err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to decode crops"})
// 		return
// 	}

// 	farm.Crops = crops

// 	utils.RespondWithJSON(w, http.StatusOK, utils.M{
// 		"success": true,
// 		"farm":    farm,
// 	})
// }
// func CreateFarm(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx := r.Context()
// 	err := r.ParseMultipartForm(10 << 20)
// 	if err != nil {
// 		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Failed to parse form"})
// 		return
// 	}

// 	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
// 	if !ok {
// 		http.Error(w, "Invalid user", http.StatusBadRequest)
// 		return
// 	}

// 	farmID := utils.GenerateID(14)

// 	farm := models.Farm{
// 		FarmID:             farmID,
// 		Name:               r.FormValue("name"),
// 		Location:           r.FormValue("location"),
// 		Description:        r.FormValue("description"),
// 		Owner:              r.FormValue("owner"),
// 		Contact:            r.FormValue("contact"),
// 		AvailabilityTiming: r.FormValue("availabilityTiming"),
// 		CreatedAt:          time.Now(),
// 		UpdatedAt:          time.Now(),
// 		Crops:              []models.Crop{},
// 		CreatedBy:          requestingUserID,
// 	}

// 	if farm.Name == "" || farm.Location == "" || farm.Owner == "" || farm.Contact == "" {
// 		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Missing required fields"})
// 		return
// 	}

// 	fileName, err := filemgr.SaveFormFile(r.MultipartForm, "photo", filemgr.EntityFarm, filemgr.PicPhoto, false)
// 	if err == nil && fileName != "" {
// 		farm.Photo = fileName
// 	}

// 	_, err = db.FarmsCollection.InsertOne(context.Background(), farm)
// 	if err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to insert farm"})
// 		return
// 	}

// 	go mq.Emit(ctx, "farm-created", models.Index{EntityType: "farm", EntityId: farm.FarmID, Method: "POST"})

// 	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true, "id": farm.FarmID})
// }

// func EditFarm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	ctx := r.Context()
// 	farmID := ps.ByName("id")

// 	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
// 	if !ok {
// 		http.Error(w, "Invalid user", http.StatusBadRequest)
// 		return
// 	}

// 	_ = requestingUserID

// 	updateFields := bson.M{}
// 	contentType := r.Header.Get("Content-Type")

// 	var input models.Farm

// 	if strings.HasPrefix(contentType, "multipart/form-data") {
// 		err := r.ParseMultipartForm(10 << 20)
// 		if err != nil {
// 			utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Malformed multipart data"})
// 			return
// 		}

// 		input.Name = r.FormValue("name")
// 		input.Location = r.FormValue("location")
// 		input.Description = r.FormValue("description")
// 		input.Owner = r.FormValue("owner")
// 		input.Contact = r.FormValue("contact")
// 		input.AvailabilityTiming = r.FormValue("availabilityTiming")

// 		fileName, err := filemgr.SaveFormFile(r.MultipartForm, "photo", filemgr.EntityFarm, filemgr.PicPhoto, false)
// 		if err == nil && fileName != "" {
// 			updateFields["photo"] = fileName
// 		}
// 	} else {
// 		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
// 			utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Invalid JSON body"})
// 			return
// 		}
// 	}

// 	if input.Name != "" {
// 		updateFields["name"] = input.Name
// 	}
// 	if input.Location != "" {
// 		updateFields["location"] = input.Location
// 	}
// 	if input.Description != "" {
// 		updateFields["description"] = input.Description
// 	}
// 	if input.Owner != "" {
// 		updateFields["owner"] = input.Owner
// 	}
// 	if input.Contact != "" {
// 		updateFields["contact"] = input.Contact
// 	}
// 	if input.AvailabilityTiming != "" {
// 		updateFields["availabilityTiming"] = input.AvailabilityTiming
// 	}

// 	if len(updateFields) == 0 {
// 		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "No fields to update"})
// 		return
// 	}

// 	updateFields["updatedAt"] = time.Now()

// 	_, err := db.FarmsCollection.UpdateOne(r.Context(), bson.M{"farmid": farmID}, bson.M{"$set": updateFields})
// 	if err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Database error"})
// 		return
// 	}

// 	go mq.Emit(ctx, "farm-updated", models.Index{EntityType: "farm", EntityId: farmID, Method: "PUT"})

// 	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true, "message": "Farm updated"})
// }

// func DeleteFarm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	ctx := r.Context()
// 	farmID := ps.ByName("id")
// 	// Retrieve user ID
// 	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
// 	if !ok {
// 		http.Error(w, "Invalid user", http.StatusBadRequest)
// 		return
// 	}
// 	_ = requestingUserID

// 	var farm models.Farm
// 	if err := db.FarmsCollection.FindOne(context.Background(), bson.M{"farmid": farmID}).Decode(&farm); err != nil {
// 		utils.RespondWithJSON(w, http.StatusNotFound, utils.M{"success": false, "message": "Not found"})
// 		return
// 	}

// 	_, err := db.FarmsCollection.DeleteOne(context.Background(), bson.M{"farmid": farmID})
// 	if err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false})
// 		return
// 	}

// 	if farm.Photo != "" {
// 		_ = os.Remove("." + farm.Photo)
// 	}

// 	go mq.Emit(ctx, "farm-deleted", models.Index{EntityType: "farm", EntityId: farmID, Method: "DELETE"})
// 	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true})
// }
// func GetCropFarms(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
// 	defer cancel()

// 	cropIDHex := ps.ByName("cropid")
// 	cropID := cropIDHex
// 	// Optional query params
// 	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
// 	if page < 1 {
// 		page = 1
// 	}
// 	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
// 	if limit < 1 {
// 		limit = 10
// 	}
// 	skip := (page - 1) * limit

// 	sortBy := r.URL.Query().Get("sortBy")
// 	sortOrder := r.URL.Query().Get("sortOrder")
// 	breedFilter := strings.ToLower(r.URL.Query().Get("breed"))

// 	// Find crops by ID
// 	cursor, err := db.CropsCollection.Find(ctx, bson.M{"cropid": cropID})
// 	if err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to fetch crop data"})
// 		return
// 	}
// 	defer cursor.Close(ctx)

// 	var cropInstances []models.Crop
// 	if err := cursor.All(ctx, &cropInstances); err != nil || len(cropInstances) == 0 {
// 		utils.RespondWithJSON(w, http.StatusNotFound, utils.M{"success": false, "message": "Crop not found"})
// 		return
// 	}

// 	cropName := cropInstances[0].Name
// 	cropCategory := cropInstances[0].Category

// 	// Fetch farms
// 	farmIDs := make([]string, len(cropInstances))
// 	for i, crop := range cropInstances {
// 		farmIDs[i] = crop.FarmID
// 	}
// 	farmCursor, err := db.FarmsCollection.Find(ctx, bson.M{"cropid": bson.M{"$in": farmIDs}})
// 	if err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to fetch farms"})
// 		return
// 	}
// 	defer farmCursor.Close(ctx)

// 	var farms []models.Farm
// 	if err := farmCursor.All(ctx, &farms); err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to decode farms"})
// 		return
// 	}

// 	// Build and optionally filter listings
// 	type CropListing struct {
// 		FarmID     string  `json:"farmId"`
// 		FarmName   string  `json:"farmName"`
// 		Location   string  `json:"location"`
// 		Breed      string  `json:"breed"`
// 		PricePerKg float64 `json:"pricePerKg"`
// 	}

// 	var listings []CropListing
// 	for _, crop := range cropInstances {
// 		if breedFilter != "" && strings.ToLower(crop.Notes) != breedFilter {
// 			continue
// 		}
// 		var farmName, location string
// 		for _, f := range farms {
// 			if f.FarmID == crop.FarmID {
// 				farmName = f.Name
// 				location = f.Location
// 				break
// 			}
// 		}
// 		listings = append(listings, CropListing{
// 			FarmID:     crop.FarmID,
// 			FarmName:   farmName,
// 			Location:   location,
// 			Breed:      crop.Notes,
// 			PricePerKg: crop.Price,
// 		})
// 	}

// 	// Sort
// 	switch sortBy {
// 	case "price":
// 		sort.Slice(listings, func(i, j int) bool {
// 			if sortOrder == "desc" {
// 				return listings[i].PricePerKg > listings[j].PricePerKg
// 			}
// 			return listings[i].PricePerKg < listings[j].PricePerKg
// 		})
// 	case "breed":
// 		sort.Slice(listings, func(i, j int) bool {
// 			if sortOrder == "desc" {
// 				return listings[i].Breed > listings[j].Breed
// 			}
// 			return listings[i].Breed < listings[j].Breed
// 		})
// 	}

// 	// Paginate
// 	total := len(listings)
// 	end := skip + limit
// 	if skip > total {
// 		skip = total
// 	}
// 	if end > total {
// 		end = total
// 	}
// 	paginated := listings[skip:end]

// 	// Respond
// 	utils.RespondWithJSON(w, http.StatusOK, utils.M{
// 		"success":  true,
// 		"name":     cropName,
// 		"category": cropCategory,
// 		"listings": paginated,
// 		"total":    total,
// 		"page":     page,
// 		"limit":    limit,
// 	})
// }

// func GetCropTypeFarms(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
// 	defer cancel()

// 	cropName := ps.ByName("cropname")
// 	if cropName == "" {
// 		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{
// 			"success": false,
// 			"message": "Missing crop name parameter",
// 		})
// 		return
// 	}

// 	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
// 	if page < 1 {
// 		page = 1
// 	}
// 	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
// 	if limit < 1 {
// 		limit = 10
// 	}
// 	skip := (page - 1) * limit

// 	sortBy := r.URL.Query().Get("sortBy")
// 	sortOrder := r.URL.Query().Get("sortOrder")
// 	breedFilter := strings.ToLower(r.URL.Query().Get("breed"))

// 	filter := bson.M{
// 		"name": bson.M{"$regex": primitive.Regex{Pattern: "^" + regexp.QuoteMeta(cropName) + "$", Options: "i"}},
// 	}
// 	cursor, err := db.CropsCollection.Find(ctx, filter)
// 	if err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{
// 			"success": false,
// 			"message": "Failed to fetch crop data",
// 		})
// 		return
// 	}
// 	defer cursor.Close(ctx)

// 	var cropInstances []models.Crop
// 	if err := cursor.All(ctx, &cropInstances); err != nil || len(cropInstances) == 0 {
// 		utils.RespondWithJSON(w, http.StatusNotFound, utils.M{
// 			"success": false,
// 			"message": "Crop type not found",
// 		})
// 		return
// 	}

// 	cropCategory := cropInstances[0].Category

// 	farmIDs := make([]string, len(cropInstances))
// 	for i, crop := range cropInstances {
// 		farmIDs[i] = crop.FarmID
// 	}

// 	farmCursor, err := db.FarmsCollection.Find(ctx, bson.M{"farmid": bson.M{"$in": farmIDs}})
// 	if err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{
// 			"success": false,
// 			"message": "Failed to fetch farms",
// 		})
// 		return
// 	}
// 	defer farmCursor.Close(ctx)

// 	var farms []models.Farm
// 	if err := farmCursor.All(ctx, &farms); err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{
// 			"success": false,
// 			"message": "Failed to decode farms",
// 		})
// 		return
// 	}

// 	farmMap := make(map[string]models.Farm)
// 	for _, f := range farms {
// 		farmMap[f.FarmID] = f
// 	}

// 	var listings []models.CropListing
// 	for _, crop := range cropInstances {
// 		if breedFilter != "" && strings.ToLower(crop.Notes) != breedFilter {
// 			continue
// 		}

// 		farm, ok := farmMap[crop.FarmID]
// 		if !ok {
// 			continue
// 		}

// 		var harvestDate string
// 		if crop.HarvestDate != nil {
// 			harvestDate = crop.HarvestDate.Format(time.RFC3339)
// 		}

// 		listings = append(listings, models.CropListing{
// 			FarmID:         crop.FarmID,
// 			FarmName:       farm.Name,
// 			Location:       farm.Location,
// 			Breed:          crop.Notes,
// 			PricePerKg:     crop.Price,
// 			AvailableQtyKg: crop.Quantity,
// 			HarvestDate:    harvestDate,
// 			Tags:           farm.Tags,
// 		})
// 	}

// 	// Sorting
// 	switch sortBy {
// 	case "price":
// 		sort.Slice(listings, func(i, j int) bool {
// 			if sortOrder == "desc" {
// 				return listings[i].PricePerKg > listings[j].PricePerKg
// 			}
// 			return listings[i].PricePerKg < listings[j].PricePerKg
// 		})
// 	case "breed":
// 		sort.Slice(listings, func(i, j int) bool {
// 			if sortOrder == "desc" {
// 				return listings[i].Breed > listings[j].Breed
// 			}
// 			return listings[i].Breed < listings[j].Breed
// 		})
// 	}

// 	// Pagination
// 	total := len(listings)
// 	end := skip + limit
// 	if skip > total {
// 		skip = total
// 	}
// 	if end > total {
// 		end = total
// 	}
// 	paginated := listings[skip:end]

// 	utils.RespondWithJSON(w, http.StatusOK, utils.M{
// 		"success":  true,
// 		"name":     cropName,
// 		"category": cropCategory,
// 		"listings": paginated,
// 		"total":    total,
// 		"page":     page,
// 		"limit":    limit,
// 	})
// }

// func GetPaginatedFarms(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
// 	defer cancel()

// 	page := utils.ParseInt(r.URL.Query().Get("page"))
// 	if page <= 0 {
// 		page = 1
// 	}
// 	limit := utils.ParseInt(r.URL.Query().Get("limit"))
// 	if limit <= 0 || limit > 100 {
// 		limit = 10
// 	}
// 	skip := (page - 1) * limit

// 	// Count total farms for pagination metadata
// 	total, err := db.FarmsCollection.CountDocuments(ctx, bson.M{})
// 	if err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to count farms"})
// 		return
// 	}

// 	// Aggregation with $lookup to join crops into farms
// 	pipeline := mongo.Pipeline{
// 		{{Key: "$sort", Value: bson.D{{Key: "updatedAt", Value: -1}}}},
// 		{{Key: "$skip", Value: int64(skip)}},
// 		{{Key: "$limit", Value: int64(limit)}},
// 		{{
// 			Key: "$lookup",
// 			Value: bson.D{
// 				{Key: "from", Value: "crops"},
// 				{Key: "localField", Value: "cropid"},
// 				{Key: "foreignField", Value: "farmId"},
// 				{Key: "as", Value: "crops"},
// 			},
// 		}},
// 	}

// 	cursor, err := db.FarmsCollection.Aggregate(ctx, pipeline)
// 	if err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to aggregate farms with crops"})
// 		return
// 	}
// 	defer cursor.Close(ctx)

// 	var farms []models.Farm
// 	if err := cursor.All(ctx, &farms); err != nil {
// 		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to decode result"})
// 		return
// 	}

// 	if len(farms) == 0 {
// 		farms = []models.Farm{}
// 	}

// 	utils.RespondWithJSON(w, http.StatusOK, utils.M{
// 		"success": true,
// 		"farms":   farms,
// 		"total":   total,
// 		"page":    page,
// 		"limit":   limit,
// 	})
// }
