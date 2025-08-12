package events

import (
	"context"
	"encoding/json"
	"log"
	"naevis/db"
	"naevis/filemgr"
	"naevis/globals"
	"naevis/models"
	"naevis/mq"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// var eventpicUploadPath = "./static/eventpic"

func CreateEvent(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	event, err := parseEventData(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	prepareEventDefaults(&event, requestingUserID)

	if err := parseArtistData(r, &event); err != nil {
		http.Error(w, "Invalid artists data", http.StatusBadRequest)
		return
	}

	// Banner upload
	if name, err := processEventImageUpload(r, "banner", filemgr.EntityEvent, filemgr.PicBanner, event.EventID, true); err == nil && name != "" {
		event.Banner = name
	} else if err != nil {
		http.Error(w, "Banner upload failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Seating plan upload
	if name, err := processEventImageUpload(r, "event-seating", filemgr.EntityEvent, filemgr.PicSeating, event.EventID, false); err == nil && name != "" {
		event.SeatingPlanImage = name
	} else if err != nil {
		http.Error(w, "Seating plan upload failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Insert to DB
	result, err := db.EventsCollection.InsertOne(context.TODO(), event)
	if err != nil || result.InsertedID == nil {
		log.Printf("DB insert error: %v", err)
		http.Error(w, "Error saving event", http.StatusInternalServerError)
		return
	}

	userdata.SetUserData("event", event.EventID, requestingUserID, "", "")
	go mq.Emit(ctx, "event-created", models.Index{EntityType: "event", EntityId: event.EventID, Method: "POST"})

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(event); err != nil {
		log.Printf("Encoding response error: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func prepareEventDefaults(event *models.Event, userID string) {
	event.CreatorID = userID
	event.CreatedAt = time.Now().UTC()
	event.Date = event.Date.UTC()
	event.Status = "active"
	event.FAQs = []models.FAQ{}
	event.Artists = []string{}
	event.Tags = []string{}
	event.Merch = []models.Merch{}
	event.Tickets = []models.Ticket{}
	event.OrganizerName = strings.TrimSpace(event.OrganizerName)
	event.OrganizerContact = strings.TrimSpace(event.OrganizerContact)
	// event.CustomFields = []models.SocialMediaLinks{}
	// event.SocialLinks = []models.SocialMediaLinks{}
	// event.AccessibilityInfo = strings.TrimSpace(event.AccessibilityInfo)

	event.EventID = utils.GenerateRandomString(14)

	// Ensure no collision
	if db.EventsCollection.FindOne(context.TODO(), bson.M{"eventid": event.EventID}).Err() == nil {
		event.EventID = utils.GenerateRandomString(14) // regenerate once
	}
}

func parseArtistData(r *http.Request, event *models.Event) error {
	artistStr := r.FormValue("artists")
	if artistStr == "" {
		return nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(artistStr), &ids); err != nil {
		return err
	}
	event.Artists = ids
	return nil
}

func parseEventData(r *http.Request) (models.Event, error) {
	var event models.Event
	data := r.FormValue("event")
	if data == "" {
		return event, http.ErrMissingFile
	}
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return event, err
	}
	return event, nil
}
