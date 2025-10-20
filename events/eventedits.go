package events

import (
	"context"
	"log"
	"naevis/db"
	"naevis/filemgr"
	"naevis/globals"
	"naevis/models"
	"naevis/mq"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func EditEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	eventID := ps.ByName("eventid")
	if eventID == "" {
		http.Error(w, "Missing event ID", http.StatusBadRequest)
		return
	}

	updateFields, err := updateEventFields(r)
	if err != nil {
		log.Printf("Invalid update fields for event %s: %v", eventID, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validateUpdateFields(updateFields); err != nil {
		log.Printf("Validation failed for event %s: %v", eventID, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Banner image
	bannerName, err := processEventImageUpload(r, "event-banner", filemgr.EntityEvent, filemgr.PicBanner, eventID, true)
	if err != nil {
		http.Error(w, "Banner upload failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	if bannerName != "" {
		updateFields["banner_image"] = bannerName
	}

	// Seating plan image
	seatingName, err := processEventImageUpload(r, "event-seating", filemgr.EntityEvent, filemgr.PicSeating, eventID, false)
	if err != nil {
		http.Error(w, "Seating plan upload failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	if seatingName != "" {
		updateFields["seating"] = seatingName
	}

	updateFields["updated_at"] = time.Now()

	// Update in DB
	result, err := db.EventsCollection.UpdateOne(
		context.TODO(),
		bson.M{"eventid": eventID},
		bson.M{"$set": updateFields},
	)
	if err != nil {
		log.Printf("Error updating event %s: %v", eventID, err)
		http.Error(w, "Error updating event", http.StatusInternalServerError)
		return
	}

	if result.MatchedCount == 0 {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	// Fetch updated event
	var updatedEvent models.Event
	if err := db.EventsCollection.FindOne(context.TODO(), bson.M{"eventid": eventID}).Decode(&updatedEvent); err != nil {
		http.Error(w, "Error retrieving updated event", http.StatusInternalServerError)
		return
	}

	go mq.Emit(ctx, "event-updated", models.Index{EntityType: "event", EntityId: eventID, Method: "PUT"})
	utils.RespondWithJSON(w, http.StatusOK, updatedEvent)
}

// Handle deleting event
func DeleteEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	eventID := ps.ByName("eventid")

	// Get the ID of the requesting user from the context
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	// Get the event details to verify the creator
	// collection := client.Database("eventdb").Collection("events")
	var event models.Event
	err := db.EventsCollection.FindOne(context.TODO(), bson.M{"eventid": eventID}).Decode(&event)
	if err != nil {
		http.Error(w, "Event not found", http.StatusNotFound)
		return
	}

	// Check if the requesting user is the creator of the event
	if event.CreatorID != requestingUserID {
		log.Printf("User %s attempted to delete an event they did not create. models.EventID: %s", requestingUserID, eventID)
		http.Error(w, "Unauthorized to delete this event", http.StatusForbidden)
		return
	}

	// Delete the event from MongoDB
	_, err = db.EventsCollection.DeleteOne(context.TODO(), bson.M{"eventid": eventID})
	if err != nil {
		http.Error(w, "error deleting event", http.StatusInternalServerError)
		return
	}

	// Delete related data (tickets, media, merch)
	if err := deleteRelatedData(eventID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	userdata.DelUserData("event", event.EventID, requestingUserID)

	m := models.Index{EntityType: "event", EntityId: eventID, Method: "DELETE"}
	go mq.Emit(ctx, "event-deleted", m)

	// Send success response
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Event deleted successfully"})
}
