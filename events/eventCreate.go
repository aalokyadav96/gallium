package events

import (
	"context"
	"encoding/json"
	"log"
	"naevis/db"
	"naevis/filemgr"
	"naevis/globals"
	"naevis/mq"
	"naevis/structs"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

var eventpicUploadPath = "./static/eventpic"

func CreateEvent(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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
		event.BannerImage = name
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
	go mq.Emit("event-created", mq.Index{EntityType: "event", EntityId: event.EventID, Method: "POST"})

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(event); err != nil {
		log.Printf("Encoding response error: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// func CreateEvent(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		http.Error(w, "Unable to parse form", http.StatusBadRequest)
// 		return
// 	}

// 	event, err := parseEventData(r)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusBadRequest)
// 		return
// 	}

// 	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
// 	if !ok {
// 		http.Error(w, "Invalid user", http.StatusBadRequest)
// 		return
// 	}

// 	prepareEventDefaults(&event, requestingUserID)

// 	if err := parseArtistData(r, &event); err != nil {
// 		http.Error(w, "Invalid artists data", http.StatusBadRequest)
// 		return
// 	}

// 	// ⬇️ Banner
// 	if name, err := processBannerxUpload(r, event.EventID); err == nil && name != "" {
// 		event.BannerImage = name
// 	} else if err != nil {
// 		http.Error(w, "Banner upload failed: "+err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	// ⬇️ Seating
// 	if name, err := processSeatingUpload(r, event.EventID); err == nil && name != "" {
// 		event.SeatingPlanImage = name
// 	} else if err != nil {
// 		http.Error(w, "Seating plan upload failed: "+err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	// Insert
// 	result, err := db.EventsCollection.InsertOne(context.TODO(), event)
// 	if err != nil || result.InsertedID == nil {
// 		log.Printf("DB insert error: %v", err)
// 		http.Error(w, "Error saving event", http.StatusInternalServerError)
// 		return
// 	}

// 	userdata.SetUserData("event", event.EventID, requestingUserID, "", "")
// 	go mq.Emit("event-created", mq.Index{EntityType: "event", EntityId: event.EventID, Method: "POST"})

// 	w.WriteHeader(http.StatusCreated)
// 	if err := json.NewEncoder(w).Encode(event); err != nil {
// 		log.Printf("Encoding response error: %v", err)
// 		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
// 	}
// }

// func processBannerxUpload(r *http.Request, eventID string) (string, error) {
// 	dir := filepath.Join(eventpicUploadPath, "banner")
// 	fileName, err := filemgr.SaveFormFile(r, "banner", dir, false)
// 	if err != nil || fileName == "" {
// 		return "", err
// 	}
// 	utils.CreateThumb(eventID, dir, ".jpg", 300, 200)
// 	return fileName, nil
// }

// func processSeatingUpload(r *http.Request, eventID string) (string, error) {
// 	_ = eventID
// 	dir := filepath.Join(eventpicUploadPath, "seating")
// 	fileName, err := filemgr.SaveFormFile(r, "event-seating", dir, false)
// 	if err != nil || fileName == "" {
// 		return "", err
// 	}
// 	return fileName, nil
// }

func prepareEventDefaults(event *structs.Event, userID string) {
	event.CreatorID = userID
	event.CreatedAt = time.Now().UTC()
	event.Date = event.Date.UTC()
	event.Status = "active"
	event.FAQs = []structs.FAQ{}
	event.Artists = []string{}
	event.Tags = []string{}
	event.Merch = []structs.Merch{}
	event.Tickets = []structs.Ticket{}
	event.OrganizerName = strings.TrimSpace(event.OrganizerName)
	event.OrganizerContact = strings.TrimSpace(event.OrganizerContact)
	// event.CustomFields = []structs.SocialMediaLinks{}
	// event.SocialLinks = []structs.SocialMediaLinks{}
	// event.AccessibilityInfo = strings.TrimSpace(event.AccessibilityInfo)

	event.EventID = utils.GenerateID(14)

	// Ensure no collision
	if db.EventsCollection.FindOne(context.TODO(), bson.M{"eventid": event.EventID}).Err() == nil {
		event.EventID = utils.GenerateID(14) // regenerate once
	}
}

func parseArtistData(r *http.Request, event *structs.Event) error {
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

func parseEventData(r *http.Request) (structs.Event, error) {
	var event structs.Event
	data := r.FormValue("event")
	if data == "" {
		return event, http.ErrMissingFile
	}
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return event, err
	}
	return event, nil
}

// func CreateEvent(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		http.Error(w, "Unable to parse form", http.StatusBadRequest)
// 		return
// 	}

// 	event, err := parseEventData(r)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusBadRequest)
// 		return
// 	}

// 	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
// 	if !ok {
// 		http.Error(w, "Invalid user", http.StatusBadRequest)
// 		return
// 	}

// 	prepareEventDefaults(&event, requestingUserID)

// 	if err := parseArtistData(r, &event); err != nil {
// 		http.Error(w, "Invalid artists data", http.StatusBadRequest)
// 		return
// 	}

// 	// Banner
// 	if err := handleBannerUpload(r, &event); err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	// Seating
// 	if err := handleSeatingUpload(r, &event); err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	// Insert
// 	result, err := db.EventsCollection.InsertOne(context.TODO(), event)
// 	if err != nil || result.InsertedID == nil {
// 		log.Printf("DB insert error: %v", err)
// 		http.Error(w, "Error saving event", http.StatusInternalServerError)
// 		return
// 	}

// 	userdata.SetUserData("event", event.EventID, requestingUserID, "", "")
// 	go mq.Emit("event-created", mq.Index{EntityType: "event", EntityId: event.EventID, Method: "POST"})

// 	w.WriteHeader(http.StatusCreated)
// 	if err := json.NewEncoder(w).Encode(event); err != nil {
// 		log.Printf("Encoding response error: %v", err)
// 		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
// 	}
// }
// func parseEventData(r *http.Request) (structs.Event, error) {
// 	var event structs.Event
// 	data := r.FormValue("event")
// 	if data == "" {
// 		return event, http.ErrMissingFile
// 	}
// 	if err := json.Unmarshal([]byte(data), &event); err != nil {
// 		return event, err
// 	}
// 	return event, nil
// }

// func prepareEventDefaults(event *structs.Event, userID string) {
// 	event.CreatorID = userID
// 	event.CreatedAt = time.Now().UTC()
// 	event.Date = event.Date.UTC()
// 	event.Status = "active"
// 	event.FAQs = []structs.FAQ{}
// 	event.Artists = []string{}
// 	event.Tags = []string{}
// 	event.Merch = []structs.Merch{}
// 	event.Tickets = []structs.Ticket{}
// 	event.OrganizerName = strings.TrimSpace(event.OrganizerName)
// 	event.OrganizerContact = strings.TrimSpace(event.OrganizerContact)
// 	// event.CustomFields = []structs.SocialMediaLinks{}
// 	// event.SocialLinks = []structs.SocialMediaLinks{}
// 	// event.AccessibilityInfo = strings.TrimSpace(event.AccessibilityInfo)

// 	event.EventID = utils.GenerateID(14)

// 	// Ensure no collision
// 	if db.EventsCollection.FindOne(context.TODO(), bson.M{"eventid": event.EventID}).Err() == nil {
// 		event.EventID = utils.GenerateID(14) // regenerate once
// 	}
// }

// func parseArtistData(r *http.Request, event *structs.Event) error {
// 	artistStr := r.FormValue("artists")
// 	if artistStr == "" {
// 		return nil
// 	}
// 	var ids []string
// 	if err := json.Unmarshal([]byte(artistStr), &ids); err != nil {
// 		return err
// 	}
// 	event.Artists = ids
// 	return nil
// }

// func handleBannerUpload(r *http.Request, event *structs.Event) error {
// 	file, _, err := r.FormFile("banner")
// 	if err != nil {
// 		if err == http.ErrMissingFile {
// 			return nil
// 		}
// 		return err
// 	}
// 	defer file.Close()

// 	if err := validateImage(file); err != nil {
// 		return err
// 	}
// 	file.Seek(0, io.SeekStart)

// 	dir := filepath.Join(eventpicUploadPath, "banner")
// 	if err := os.MkdirAll(dir, 0755); err != nil {
// 		return err
// 	}

// 	filename := filepath.Join(dir, event.EventID+".jpg")
// 	out, err := os.Create(filename)
// 	if err != nil {
// 		return err
// 	}
// 	defer out.Close()
// 	if _, err := io.Copy(out, file); err != nil {
// 		return err
// 	}

// 	event.BannerImage = filepath.Base(filename)
// 	utils.CreateThumb(event.EventID, dir, ".jpg", 300, 200)

// 	return nil
// }

// func handleSeatingUpload(r *http.Request, event *structs.Event) error {
// 	file, _, err := r.FormFile("event-seating")
// 	if err != nil {
// 		if err == http.ErrMissingFile {
// 			return nil
// 		}
// 		return err
// 	}
// 	defer file.Close()

// 	if err := validateImage(file); err != nil {
// 		return err
// 	}
// 	file.Seek(0, io.SeekStart)

// 	dir := filepath.Join(eventpicUploadPath, "seating")
// 	if err := os.MkdirAll(dir, 0755); err != nil {
// 		return err
// 	}

// 	filename := filepath.Join(dir, event.EventID+"seating.jpg")
// 	out, err := os.Create(filename)
// 	if err != nil {
// 		return err
// 	}
// 	defer out.Close()
// 	if _, err := io.Copy(out, file); err != nil {
// 		return err
// 	}

// 	event.SeatingPlanImage = filepath.Base(filename)
// 	return nil
// }

// func validateImage(file multipart.File) error {
// 	buff := make([]byte, 512)
// 	if _, err := file.Read(buff); err != nil {
// 		return err
// 	}
// 	contentType := http.DetectContentType(buff)
// 	if !strings.HasPrefix(contentType, "image/") {
// 		return http.ErrNotSupported
// 	}
// 	return nil
// }
