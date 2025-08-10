package places

import (
	"context"
	"encoding/json"
	"fmt"
	"naevis/autocom"
	"naevis/db"
	"naevis/filemgr"
	"naevis/globals"
	"naevis/models"
	"naevis/mq"
	"naevis/rdx"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// var bannerDir string = "./static/placepic"

// Places
func GetPlaces(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Try cache
	if cached, _ := rdx.RdxGet("places"); cached != "" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cached))
		return
	}

	places, err := utils.FindAndDecode[models.Place](ctx, db.PlacesCollection, bson.M{})
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "Failed to fetch places")
		return
	}

	data := utils.ToJSON(places) // or json.Marshal
	rdx.RdxSet("places", string(data))
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// func GetPlaces(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	w.Header().Set("Content-Type", "application/json")

// 	// // Check if places are cached
// 	// cachedPlaces, err := rdx.RdxGet("places")
// 	// if err == nil && cachedPlaces != "" {
// 	// 	// Return cached places if available
// 	// 	w.Write([]byte(cachedPlaces))
// 	// 	return
// 	// }

// 	cursor, err := db.PlacesCollection.Find(context.TODO(), bson.M{})
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(context.TODO())

// 	var places []models.Place
// 	if err = cursor.All(context.TODO(), &places); err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	// Cache the result
// 	placesJSON, _ := json.Marshal(places)
// 	rdx.RdxSet("places", string(placesJSON))

// 	if places == nil {
// 		places = []models.Place{}
// 	}

// 	// Encode and return places data
// 	json.NewEncoder(w).Encode(places)
// }

func GetPlace(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.ByName("placeid")

	// Aggregation pipeline to fetch place along with related tickets, media, and merch
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "placeid", Value: id}}}},
	}

	// Execute the aggregation query
	cursor, err := db.PlacesCollection.Aggregate(context.TODO(), pipeline)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var place models.Place
	if cursor.Next(context.TODO()) {
		if err := cursor.Decode(&place); err != nil {
			http.Error(w, "Failed to decode place data", http.StatusInternalServerError)
			return
		}
	} else {
		// http.Error(w, "Place not found", http.StatusNotFound)
		// Respond with success
		w.WriteHeader(http.StatusNotFound)
		response := map[string]any{
			"status":  http.StatusNoContent,
			"message": "Place not found",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Encode the place as JSON and write to response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(place); err != nil {
		http.Error(w, "Failed to encode place data", http.StatusInternalServerError)
	}
}

func GetPlaceQ(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	id := r.URL.Query().Get("id")

	// Aggregation pipeline to fetch place along with related tickets, media, and merch
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "placeid", Value: id}}}},
	}

	// Execute the aggregation query
	cursor, err := db.PlacesCollection.Aggregate(context.TODO(), pipeline)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var place models.Place
	if cursor.Next(context.TODO()) {
		if err := cursor.Decode(&place); err != nil {
			http.Error(w, "Failed to decode place data", http.StatusInternalServerError)
			return
		}
	} else {
		// http.Error(w, "Place not found", http.StatusNotFound)
		// Respond with success
		w.WriteHeader(http.StatusNotFound)
		response := map[string]any{
			"status":  http.StatusNoContent,
			"message": "Place not found",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Encode the place as JSON and write to response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(place); err != nil {
		http.Error(w, "Failed to encode place data", http.StatusInternalServerError)
	}
}

// // Handles file upload and returns the banner file name
// func handleBannerUpload(w http.ResponseWriter, r *http.Request, placeID string) (string, error) {
// 	bannerFile, header, err := r.FormFile("banner")
// 	if err != nil {
// 		if err == http.ErrMissingFile {
// 			return "", nil // No file uploaded, continue without it
// 		}
// 		return "", fmt.Errorf("error retrieving banner file")
// 	}
// 	defer bannerFile.Close()

// 	if !utils.ValidateImageFileType(w, header) {
// 		return "", fmt.Errorf("invalid banner file type. Only jpeg, png, webp, gif, bmp, tiff are allowed")
// 	}

// 	// Ensure the directory exists
// 	// bannerDir := "./static/placepic"
// 	if err := os.MkdirAll(bannerDir, os.ModePerm); err != nil {
// 		return "", fmt.Errorf("error creating directory for banner")
// 	}

// 	// Save the banner image
// 	bannerPath := fmt.Sprintf("%s/%s.jpg", bannerDir, placeID)
// 	out, err := os.Create(bannerPath)
// 	if err != nil {
// 		return "", fmt.Errorf("error saving banner")
// 	}
// 	defer out.Close()

// 	if _, err := io.Copy(out, bannerFile); err != nil {
// 		os.Remove(bannerPath) // Cleanup partial files
// 		return "", fmt.Errorf("error saving banner")
// 	}

//		return fmt.Sprintf("%s.jpg", placeID), nil
//	}

// func handleBannerUpload(_ http.ResponseWriter, r *http.Request, placeID string) (string, error) {
// 	_ = placeID
// 	if r.MultipartForm == nil {
// 		if err := r.ParseMultipartForm(10 << 20); err != nil {
// 			return "", fmt.Errorf("parse form: %w", err)
// 		}
// 	}

// 	fileName, err := filemgr.SaveFormFile(r.MultipartForm, "banner", filemgr.EntityPlace, filemgr.PicBanner, false)
// 	if err != nil {
// 		return "", fmt.Errorf("saving banner: %w", err)
// 	}

// 	return fileName, nil
// }

// // Parses and validates form data for places
func parsePlaceFormData(_ http.ResponseWriter, r *http.Request) (models.Place, error) {
	err := r.ParseMultipartForm(10 << 20) // 10MB limit
	if err != nil {
		return models.Place{}, fmt.Errorf("unable to parse form")
	}

	name := strings.TrimSpace(r.FormValue("name"))
	address := strings.TrimSpace(r.FormValue("address"))
	description := strings.TrimSpace(r.FormValue("description"))
	category := strings.TrimSpace(r.FormValue("category"))
	capacityStr := strings.TrimSpace(r.FormValue("capacity"))

	if name == "" || address == "" || description == "" || category == "" || capacityStr == "" {
		return models.Place{}, fmt.Errorf("all required fields must be filled")
	}

	capacity, err := strconv.Atoi(capacityStr)
	if err != nil || capacity <= 0 {
		return models.Place{}, fmt.Errorf("capacity must be a positive integer")
	}

	// Optional fields
	city := strings.TrimSpace(r.FormValue("city"))
	country := strings.TrimSpace(r.FormValue("country"))
	zipcode := strings.TrimSpace(r.FormValue("zipCode"))
	phone := strings.TrimSpace(r.FormValue("phone"))

	// Optional banner upload via filemgr
	var bannerFilename string
	if r.MultipartForm != nil {
		bannerFilename, _ = filemgr.SaveFormFile(r.MultipartForm, "banner", filemgr.EntityPlace, filemgr.PicBanner, false)
	}

	return models.Place{
		PlaceID:     utils.GenerateRandomString(14),
		Name:        name,
		Address:     address,
		Description: description,
		Category:    category,
		Capacity:    capacity,
		Banner:      bannerFilename,
		Phone:       phone,
		City:        city,
		Country:     country,
		ZipCode:     zipcode,
		CreatedAt:   time.Now(),
		ReviewCount: 0,
		Status:      "active",
	}, nil
}

// func parsePlaceFormData(_ http.ResponseWriter, r *http.Request) (models.Place, error) {
// 	err := r.ParseMultipartForm(10 << 20) // 10MB limit
// 	if err != nil {
// 		return models.Place{}, fmt.Errorf("unable to parse form")
// 	}

// 	// Required fields
// 	name := strings.TrimSpace(r.FormValue("name"))
// 	address := strings.TrimSpace(r.FormValue("address"))
// 	description := strings.TrimSpace(r.FormValue("description"))
// 	category := strings.TrimSpace(r.FormValue("category"))
// 	capacityStr := strings.TrimSpace(r.FormValue("capacity"))

// 	if name == "" || address == "" || description == "" || category == "" || capacityStr == "" {
// 		return models.Place{}, fmt.Errorf("all required fields must be filled")
// 	}

// 	capacity, err := strconv.Atoi(capacityStr)
// 	if err != nil || capacity <= 0 {
// 		return models.Place{}, fmt.Errorf("capacity must be a positive integer")
// 	}

// 	// Optional fields
// 	city := strings.TrimSpace(r.FormValue("city"))
// 	country := strings.TrimSpace(r.FormValue("country"))
// 	zipcode := strings.TrimSpace(r.FormValue("zipCode"))
// 	phone := strings.TrimSpace(r.FormValue("phone"))

// 	// Banner handling (optional)
// 	var bannerFilename string
// 	file, handler, err := r.FormFile("banner")
// 	if err == nil && file != nil {
// 		defer file.Close()
// 		// Save or process the file here, for now we just read filename
// 		bannerFilename = handler.Filename
// 		// Actual storage logic goes here...
// 	}

// 	return models.Place{
// 		PlaceID:     utils.GenerateID(14),
// 		Name:        name,
// 		Address:     address,
// 		Description: description,
// 		Category:    category,
// 		Capacity:    capacity,
// 		Banner:      bannerFilename,
// 		Phone:       phone,
// 		City:        city,
// 		Country:     country,
// 		ZipCode:     zipcode,
// 		CreatedAt:   time.Now(),
// 		ReviewCount: 0,
// 		Status:      "active",
// 	}, nil
// }

// Creates a new place
func CreatePlace(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	place, err := parsePlaceFormData(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Retrieve user ID
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}
	place.CreatedBy = requestingUserID

	// Insert into MongoDB
	_, err = db.PlacesCollection.InsertOne(context.TODO(), place)
	if err != nil {
		http.Error(w, "Error creating place", http.StatusInternalServerError)
		return
	}

	autocom.AddPlaceToAutocorrect(rdx.Conn, place.PlaceID, place.Name)

	userdata.SetUserData("place", place.PlaceID, requestingUserID, "", "")
	go mq.Emit(ctx, "place-created", models.Index{EntityType: "place", EntityId: place.PlaceID, Method: "POST"})

	utils.RespondWithJSON(w, http.StatusCreated, place)
}
