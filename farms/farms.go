package farms

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"naevis/db"
	"naevis/globals"
	"naevis/models"
	"naevis/mq"
	"naevis/utils"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func CreateFarm(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Failed to parse form"})
		return
	}

	// Retrieve user ID
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}
	farm := models.Farm{
		FarmID:             primitive.NewObjectID(),
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

	if path, err := handleFarmPhotoUpload(r, farm.FarmID); err == nil {
		farm.Photo = path
	}

	_, err = db.FarmsCollection.InsertOne(context.Background(), farm)
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to insert farm"})
		return
	}

	go mq.Emit("farm-created", mq.Index{EntityType: "farm", EntityId: farm.FarmID.String(), Method: "POST"})

	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true, "id": farm.FarmID.Hex()})
}

func GetFarm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	if err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Invalid farm ID"})
		return
	}

	var farm models.Farm
	if err := db.FarmsCollection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&farm); err != nil {
		utils.RespondWithJSON(w, http.StatusNotFound, utils.M{"success": false, "message": "Farm not found"})
		return
	}

	// Fetch crops from separate crops collection
	cursor, err := db.CropsCollection.Find(context.Background(), bson.M{"farmId": id})
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
func handleFarmPhotoUpload(r *http.Request, farmID primitive.ObjectID) (string, error) {
	file, handler, err := r.FormFile("photo")
	if err != nil {
		return "", err
	}
	defer file.Close()

	os.MkdirAll("./static/uploads/farms", os.ModePerm)
	filename := fmt.Sprintf("%s_%s", farmID.Hex(), handler.Filename)
	filePath := "./static/uploads/farms/" + filename
	out, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		return "", err
	}
	return "/uploads/farms/" + filename, nil
}

func EditFarm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	farmID, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	if err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Invalid farm ID"})
		return
	}

	// Retrieve user ID
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	_ = requestingUserID
	updateFields := bson.M{}
	contentType := r.Header.Get("Content-Type")

	var input models.Farm

	if strings.HasPrefix(contentType, "multipart/form-data") {
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Malformed multipart data"})
			return
		}

		input.Name = r.FormValue("name")
		input.Location = r.FormValue("location")
		input.Description = r.FormValue("description")
		input.Owner = r.FormValue("owner")
		input.Contact = r.FormValue("contact")
		input.AvailabilityTiming = r.FormValue("availabilityTiming")

		if path, err := handleFarmPhotoUpload(r, farmID); err == nil {
			updateFields["photo"] = path
		}

	} else {
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Invalid JSON body"})
			return
		}
	}

	// Build update map
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

	_, err = db.FarmsCollection.UpdateOne(r.Context(), bson.M{"_id": farmID}, bson.M{"$set": updateFields})
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Database error"})
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true, "message": "Farm updated"})
}

func DeleteFarm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	farmID, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	if err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false})
		return
	}

	// Retrieve user ID
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}
	_ = requestingUserID

	var farm models.Farm
	if err := db.FarmsCollection.FindOne(context.Background(), bson.M{"_id": farmID}).Decode(&farm); err != nil {
		utils.RespondWithJSON(w, http.StatusNotFound, utils.M{"success": false, "message": "Not found"})
		return
	}

	_, err = db.FarmsCollection.DeleteOne(context.Background(), bson.M{"_id": farmID})
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false})
		return
	}

	if farm.Photo != "" {
		_ = os.Remove("." + farm.Photo)
	}

	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true})
}
func GetCropFarms(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cropIDHex := ps.ByName("cropid")
	if cropIDHex == "" {
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Missing cropId parameter"})
		return
	}

	cropID, err := primitive.ObjectIDFromHex(cropIDHex)
	if err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Invalid cropId"})
		return
	}

	// Optional query params
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 10
	}
	skip := (page - 1) * limit

	sortBy := r.URL.Query().Get("sortBy")
	sortOrder := r.URL.Query().Get("sortOrder")
	breedFilter := strings.ToLower(r.URL.Query().Get("breed"))

	// Find crops by ID
	cursor, err := db.CropsCollection.Find(ctx, bson.M{"_id": cropID})
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to fetch crop data"})
		return
	}
	defer cursor.Close(ctx)

	var cropInstances []models.Crop
	if err := cursor.All(ctx, &cropInstances); err != nil || len(cropInstances) == 0 {
		utils.RespondWithJSON(w, http.StatusNotFound, utils.M{"success": false, "message": "Crop not found"})
		return
	}

	cropName := cropInstances[0].Name
	cropCategory := cropInstances[0].Category

	// Fetch farms
	farmIDs := make([]primitive.ObjectID, len(cropInstances))
	for i, crop := range cropInstances {
		farmIDs[i] = crop.FarmID
	}
	farmCursor, err := db.FarmsCollection.Find(ctx, bson.M{"_id": bson.M{"$in": farmIDs}})
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to fetch farms"})
		return
	}
	defer farmCursor.Close(ctx)

	var farms []models.Farm
	if err := farmCursor.All(ctx, &farms); err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to decode farms"})
		return
	}

	// Build and optionally filter listings
	type CropListing struct {
		FarmID     string  `json:"farmId"`
		FarmName   string  `json:"farmName"`
		Location   string  `json:"location"`
		Breed      string  `json:"breed"`
		PricePerKg float64 `json:"pricePerKg"`
	}

	var listings []CropListing
	for _, crop := range cropInstances {
		if breedFilter != "" && strings.ToLower(crop.Notes) != breedFilter {
			continue
		}
		var farmName, location string
		for _, f := range farms {
			if f.FarmID == crop.FarmID {
				farmName = f.Name
				location = f.Location
				break
			}
		}
		listings = append(listings, CropListing{
			FarmID:     crop.FarmID.Hex(),
			FarmName:   farmName,
			Location:   location,
			Breed:      crop.Notes,
			PricePerKg: crop.Price,
		})
	}

	// Sort
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

	// Paginate
	total := len(listings)
	end := skip + limit
	if skip > total {
		skip = total
	}
	if end > total {
		end = total
	}
	paginated := listings[skip:end]

	// Respond
	utils.RespondWithJSON(w, http.StatusOK, utils.M{
		"success":  true,
		"name":     cropName,
		"category": cropCategory,
		"listings": paginated,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

func GetCropTypeFarms(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cropName := ps.ByName("cropname")
	if cropName == "" {
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{
			"success": false,
			"message": "Missing crop name parameter",
		})
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 10
	}
	skip := (page - 1) * limit

	sortBy := r.URL.Query().Get("sortBy")
	sortOrder := r.URL.Query().Get("sortOrder")
	breedFilter := strings.ToLower(r.URL.Query().Get("breed"))

	filter := bson.M{
		"name": bson.M{"$regex": primitive.Regex{Pattern: "^" + regexp.QuoteMeta(cropName) + "$", Options: "i"}},
	}
	cursor, err := db.CropsCollection.Find(ctx, filter)
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{
			"success": false,
			"message": "Failed to fetch crop data",
		})
		return
	}
	defer cursor.Close(ctx)

	var cropInstances []models.Crop
	if err := cursor.All(ctx, &cropInstances); err != nil || len(cropInstances) == 0 {
		utils.RespondWithJSON(w, http.StatusNotFound, utils.M{
			"success": false,
			"message": "Crop type not found",
		})
		return
	}

	cropCategory := cropInstances[0].Category

	farmIDs := make([]primitive.ObjectID, len(cropInstances))
	for i, crop := range cropInstances {
		farmIDs[i] = crop.FarmID
	}

	farmCursor, err := db.FarmsCollection.Find(ctx, bson.M{"_id": bson.M{"$in": farmIDs}})
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{
			"success": false,
			"message": "Failed to fetch farms",
		})
		return
	}
	defer farmCursor.Close(ctx)

	var farms []models.Farm
	if err := farmCursor.All(ctx, &farms); err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{
			"success": false,
			"message": "Failed to decode farms",
		})
		return
	}

	farmMap := make(map[primitive.ObjectID]models.Farm)
	for _, f := range farms {
		farmMap[f.FarmID] = f
	}

	// type CropListing struct {
	// 	FarmID         string   `json:"farmId"`
	// 	FarmName       string   `json:"farmName"`
	// 	Location       string   `json:"location"`
	// 	Breed          string   `json:"breed"`
	// 	PricePerKg     float64  `json:"pricePerKg"`
	// 	AvailableQtyKg int      `json:"availableQtyKg,omitempty"`
	// 	HarvestDate    string   `json:"harvestDate,omitempty"`
	// 	Tags           []string `json:"tags,omitempty"`
	// }

	var listings []models.CropListing
	for _, crop := range cropInstances {
		if breedFilter != "" && strings.ToLower(crop.Notes) != breedFilter {
			continue
		}

		farm, ok := farmMap[crop.FarmID]
		if !ok {
			continue
		}

		var harvestDate string
		if crop.HarvestDate != nil {
			harvestDate = crop.HarvestDate.Format(time.RFC3339)
		}

		listings = append(listings, models.CropListing{
			FarmID:         crop.FarmID.Hex(),
			FarmName:       farm.Name,
			Location:       farm.Location,
			Breed:          crop.Notes,
			PricePerKg:     crop.Price,
			AvailableQtyKg: crop.Quantity,
			HarvestDate:    harvestDate,
			Tags:           farm.Tags,
		})
	}

	// Sorting
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

	// Pagination
	total := len(listings)
	end := skip + limit
	if skip > total {
		skip = total
	}
	if end > total {
		end = total
	}
	paginated := listings[skip:end]

	utils.RespondWithJSON(w, http.StatusOK, utils.M{
		"success":  true,
		"name":     cropName,
		"category": cropCategory,
		"listings": paginated,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

func GetPaginatedFarms(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	page := utils.ParseInt(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	limit := utils.ParseInt(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	skip := (page - 1) * limit

	// Count total farms for pagination metadata
	total, err := db.FarmsCollection.CountDocuments(ctx, bson.M{})
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to count farms"})
		return
	}

	// Aggregation with $lookup to join crops into farms
	pipeline := mongo.Pipeline{
		{{Key: "$sort", Value: bson.D{{Key: "updatedAt", Value: -1}}}},
		{{Key: "$skip", Value: int64(skip)}},
		{{Key: "$limit", Value: int64(limit)}},
		{{
			Key: "$lookup",
			Value: bson.D{
				{Key: "from", Value: "crops"},
				{Key: "localField", Value: "_id"},
				{Key: "foreignField", Value: "farmId"},
				{Key: "as", Value: "crops"},
			},
		}},
	}

	cursor, err := db.FarmsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to aggregate farms with crops"})
		return
	}
	defer cursor.Close(ctx)

	var farms []models.Farm
	if err := cursor.All(ctx, &farms); err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to decode result"})
		return
	}

	if len(farms) == 0 {
		farms = []models.Farm{}
	}

	utils.RespondWithJSON(w, http.StatusOK, utils.M{
		"success": true,
		"farms":   farms,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}
