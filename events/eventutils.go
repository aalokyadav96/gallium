package events

import (
	"context"
	"encoding/json"
	"fmt"
	"naevis/db"
	"naevis/filemgr"
	"naevis/utils"
	"net/http"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func updateEventFields(r *http.Request) (bson.M, error) {
	// Parse the multipart form with a 10MB limit
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return nil, fmt.Errorf("unable to parse form: %v", err)
	}

	updateFields := bson.M{}

	// Extract "event" field from form-data
	eventJSON := r.FormValue("event")
	if eventJSON == "" {
		return nil, fmt.Errorf("missing event data")
	}

	// Define a struct to parse the JSON
	var eventData struct {
		Title       string `json:"title"`
		Date        string `json:"date"`
		Category    string `json:"category"`
		Location    string `json:"location"`
		PlaceId     string `json:"placeid"`
		PlaceName   string `json:"placename"`
		Description string `json:"description"`
	}

	// Decode the JSON
	if err := json.Unmarshal([]byte(eventJSON), &eventData); err != nil {
		return nil, fmt.Errorf("invalid JSON format: %v", err)
	}

	// Map the fields to updateFields
	if eventData.Category != "" {
		updateFields["category"] = eventData.Category
	}
	if eventData.Title != "" {
		updateFields["title"] = eventData.Title
	}
	if eventData.Date != "" {
		parsedDateTime, err := time.Parse(time.RFC3339, eventData.Date)
		if err != nil {
			return nil, fmt.Errorf("invalid date format, expected RFC3339 (YYYY-MM-DDTHH:MM:SSZ)")
		}
		updateFields["date"] = parsedDateTime.UTC()
	}
	if eventData.Location != "" {
		updateFields["location"] = eventData.Location
	}
	if eventData.PlaceId != "" {
		updateFields["placeid"] = eventData.PlaceId
	}
	if eventData.PlaceName != "" {
		updateFields["placename"] = eventData.PlaceName
	}
	if eventData.Description != "" {
		updateFields["description"] = eventData.Description
	}

	return updateFields, nil
}

// Validate required fields
func validateUpdateFields(updateFields bson.M) error {
	if updateFields["category"] == "" || updateFields["title"] == "" || updateFields["location"] == "" || updateFields["description"] == "" {
		return fmt.Errorf("category, title, location, and description are required")
	}
	return nil
}

// Delete related data (tickets, media, merch) from collections
func deleteRelatedData(eventID string) error {
	// Delete related data from collections
	_, err := db.TicketsCollection.DeleteMany(context.TODO(), bson.M{"eventid": eventID})
	if err != nil {
		return fmt.Errorf("error deleting related tickets")
	}

	_, err = db.MediaCollection.DeleteMany(context.TODO(), bson.M{"eventid": eventID})
	if err != nil {
		return fmt.Errorf("error deleting related media")
	}

	_, err = db.MerchCollection.DeleteMany(context.TODO(), bson.M{"eventid": eventID})
	if err != nil {
		return fmt.Errorf("error deleting related merch")
	}

	_, err = db.ArtistEventsCollection.DeleteOne(context.TODO(), bson.M{"eventid": eventID})
	if err != nil {
		return fmt.Errorf("error deleting related artistevent")
	}

	return nil
}

func processEventImageUpload(r *http.Request, fieldName string, entity filemgr.EntityType, picture filemgr.PictureType, eventID string, resize bool) (string, error) {
	if r.MultipartForm == nil {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			return "", err
		}
	}

	fileName, err := filemgr.SaveFormFile(r.MultipartForm, fieldName, entity, picture, resize)
	if err != nil || fileName == "" {
		return "", nil // skip silently if not present
	}

	if resize && eventID != "" {
		utils.CreateThumb(eventID, filepath.Join("static", "uploads", string(entity), string(picture)), ".jpg", 300, 200)
	}

	return fileName, nil
}
