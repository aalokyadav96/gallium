package artists

import (
	"context"
	"encoding/json"
	"log"
	"naevis/db"
	"naevis/middleware"
	"naevis/models"
	"naevis/mq"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func CreateArtistEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()

	var artistevent models.ArtistEvent
	if err := json.NewDecoder(r.Body).Decode(&artistevent); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	artistevent.ArtistID = ps.ByName("id")

	tokenString := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(tokenString)
	if err != nil {
		log.Printf("JWT validation error: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	artistevent.CreatorID = claims.UserID
	artistevent.EventID = utils.GenerateRandomString(14)

	insertResult, err := db.ArtistEventsCollection.InsertOne(ctx, artistevent)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	if _, err := addEventToDB(ctx, artistevent); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to add event")
		return
	}

	utils.RespondWithJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "ArtistEvent created successfully",
		"id":      insertResult.InsertedID,
	})
}

// Update Artist Event
func UpdateArtistEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	artisteventID := ps.ByName("id")

	var updateData bson.M
	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	result, err := db.ArtistEventsCollection.UpdateOne(context.TODO(), bson.M{"eventid": artisteventID}, bson.M{"$set": updateData})
	if err != nil || result.ModifiedCount == 0 {
		utils.RespondWithError(w, http.StatusNotFound, "ArtistEvent not found or update failed")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "ArtistEvent updated successfully"})
}

// Delete Artist Event
func DeleteArtistEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	artisteventID := ps.ByName("id")

	result, err := db.ArtistEventsCollection.DeleteOne(context.TODO(), bson.M{"eventid": artisteventID})
	if err != nil || result.DeletedCount == 0 {
		utils.RespondWithError(w, http.StatusNotFound, "ArtistEvent not found or deletion failed")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "ArtistEvent deleted successfully"})
}

func addEventToDB(ctx context.Context, artistEvent models.ArtistEvent) (string, error) {
	var event models.Event

	dateString := artistEvent.Date
	layout := "2006-01-02"

	dateToSave, _ := time.Parse(layout, dateString)

	event.CreatorID = artistEvent.CreatorID
	event.CreatedAt = time.Now().UTC()
	event.Date = dateToSave.UTC()
	event.Status = "active"
	event.FAQs = []models.FAQ{}
	event.EventID = artistEvent.EventID
	event.Artists = []string{artistEvent.ArtistID}
	event.Title = artistEvent.Title
	event.Location = artistEvent.Venue
	event.Published = "draft"
	event.Category = "concert"

	// Use ctx here
	result, err := db.EventsCollection.InsertOne(ctx, event)
	if err != nil || result.InsertedID == nil {
		log.Printf("Error inserting event into MongoDB: %v", err)
		return "", err
	}

	userdata.SetUserData("event", event.EventID, artistEvent.ArtistID, "", "")

	// Pass ctx to Emit
	go mq.Emit(ctx, "event-created", models.Index{
		EntityType: "event", EntityId: event.EventID, Method: "POST",
	})

	return "", err
}

func AddArtistToEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	type RequestPayload struct {
		EventID  string `json:"eventid"`
		ArtistID string `json:"artistid"`
	}

	var payload RequestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Get artist ID from URL parameter if passed
	payload.ArtistID = ps.ByName("id")

	// Fetch event details from EventsCollection
	var event models.Event
	err := db.EventsCollection.FindOne(context.TODO(), bson.M{"eventid": payload.EventID}).Decode(&event)
	if err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "Event not found")
		return
	}

	// Check if ArtistEvent already exists
	filter := bson.M{"eventid": payload.EventID, "artistid": payload.ArtistID}
	count, err := db.ArtistEventsCollection.CountDocuments(context.TODO(), filter)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Error checking for existing artist event")
		return
	}
	if count > 0 {
		utils.RespondWithError(w, http.StatusConflict, "Artist already added to this event")
		return
	}

	// Create a new ArtistEvent object
	artistEvent := models.ArtistEvent{
		EventID:   event.EventID,
		ArtistID:  payload.ArtistID,
		Title:     event.Title,
		Date:      event.Date.Format("2006-01-02"),
		Venue:     event.PlaceName,
		City:      "", // optional: extract from event.Location
		Country:   "", // optional: extract from event.Location
		CreatorID: event.CreatorID,
		TicketURL: event.WebsiteURL,
	}

	// Insert into ArtistEventsCollection
	_, err = db.ArtistEventsCollection.InsertOne(context.TODO(), artistEvent)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to add artist to artist events")
		return
	}

	// Add artist ID to Event's Artists array
	update := bson.M{
		"$addToSet": bson.M{"artists": payload.ArtistID},
	}
	_, err = db.EventsCollection.UpdateOne(context.TODO(), bson.M{"eventid": payload.EventID}, update)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update event with artist")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Artist successfully added to event"})
}
