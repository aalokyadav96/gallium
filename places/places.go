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
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// var bannerDir string = "./static/placepic"

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

// parseAndBuildPlace parses form data and returns a Place object and updateFields map.
// mode: "create" or "update"
func parseAndBuildPlace(r *http.Request, mode string) (models.Place, bson.M, error) {
	err := r.ParseMultipartForm(10 << 20) // 10MB
	if err != nil {
		return models.Place{}, nil, fmt.Errorf("unable to parse form")
	}

	place := models.Place{}
	updateFields := bson.M{}

	// Helper to set both struct and update map
	setField := func(fieldName string, value interface{}) {
		updateFields[fieldName] = value
		reflect.ValueOf(&place).Elem().FieldByNameFunc(func(s string) bool {
			return strings.EqualFold(s, fieldName)
		}).Set(reflect.ValueOf(value))
	}

	// Required in create, optional in update
	name := strings.TrimSpace(r.FormValue("name"))
	if mode == "create" && name == "" {
		return models.Place{}, nil, fmt.Errorf("name is required")
	}
	if name != "" {
		setField("name", name)
	}

	address := strings.TrimSpace(r.FormValue("address"))
	if mode == "create" && address == "" {
		return models.Place{}, nil, fmt.Errorf("address is required")
	}
	if address != "" {
		setField("address", address)
	}

	description := strings.TrimSpace(r.FormValue("description"))
	if mode == "create" && description == "" {
		return models.Place{}, nil, fmt.Errorf("description is required")
	}
	if description != "" {
		setField("description", description)
	}

	category := strings.TrimSpace(r.FormValue("category"))
	if mode == "create" && category == "" {
		return models.Place{}, nil, fmt.Errorf("category is required")
	}
	if category != "" {
		setField("category", category)
	}

	capacityStr := strings.TrimSpace(r.FormValue("capacity"))
	if mode == "create" && capacityStr == "" {
		return models.Place{}, nil, fmt.Errorf("capacity is required")
	}
	if capacityStr != "" {
		capacity, err := strconv.Atoi(capacityStr)
		if err != nil || capacity <= 0 {
			return models.Place{}, nil, fmt.Errorf("capacity must be a positive integer")
		}
		setField("capacity", capacity)
	}

	// Optional fields
	if city := strings.TrimSpace(r.FormValue("city")); city != "" {
		setField("city", city)
	}
	if country := strings.TrimSpace(r.FormValue("country")); country != "" {
		setField("country", country)
	}
	if zipcode := strings.TrimSpace(r.FormValue("zipCode")); zipcode != "" {
		setField("zipCode", zipcode)
	}
	if phone := strings.TrimSpace(r.FormValue("phone")); phone != "" {
		setField("phone", phone)
	}

	// Banner upload
	if r.MultipartForm != nil {
		banner, _ := filemgr.SaveFormFile(r.MultipartForm, "banner", filemgr.EntityPlace, filemgr.PicBanner, false)
		if banner != "" {
			setField("banner", banner)
		}
	}

	// Common timestamps
	if mode == "create" {
		setField("placeID", utils.GenerateRandomString(14))
		setField("createdAt", time.Now())
		setField("reviewCount", 0)
		setField("status", "active")
	} else {
		setField("updatedAt", time.Now())
	}

	return place, updateFields, nil
}

// CreatePlace endpoint
func CreatePlace(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	place, _, err := parseAndBuildPlace(r, "create")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}
	place.CreatedBy = requestingUserID

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
