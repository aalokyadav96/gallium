package events

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
	"naevis/db"
	"naevis/globals"
	"naevis/mq"
	"naevis/structs"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"os"
	"path/filepath"
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

	// Banner
	if err := handleBannerUpload(r, &event); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Seating
	if err := handleSeatingUpload(r, &event); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Insert
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

func handleBannerUpload(r *http.Request, event *structs.Event) error {
	file, _, err := r.FormFile("banner")
	if err != nil {
		if err == http.ErrMissingFile {
			return nil
		}
		return err
	}
	defer file.Close()

	if err := validateImage(file); err != nil {
		return err
	}
	file.Seek(0, io.SeekStart)

	dir := filepath.Join(eventpicUploadPath, "banner")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	filename := filepath.Join(dir, event.EventID+".jpg")
	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		return err
	}

	event.BannerImage = filepath.Base(filename)
	utils.CreateThumb(event.EventID, dir, ".jpg", 300, 200)

	return nil
}

func handleSeatingUpload(r *http.Request, event *structs.Event) error {
	file, _, err := r.FormFile("event-seating")
	if err != nil {
		if err == http.ErrMissingFile {
			return nil
		}
		return err
	}
	defer file.Close()

	if err := validateImage(file); err != nil {
		return err
	}
	file.Seek(0, io.SeekStart)

	dir := filepath.Join(eventpicUploadPath, "seating")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	filename := filepath.Join(dir, event.EventID+"seating.jpg")
	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		return err
	}

	event.SeatingPlanImage = filepath.Base(filename)
	return nil
}

func validateImage(file multipart.File) error {
	buff := make([]byte, 512)
	if _, err := file.Read(buff); err != nil {
		return err
	}
	contentType := http.DetectContentType(buff)
	if !strings.HasPrefix(contentType, "image/") {
		return http.ErrNotSupported
	}
	return nil
}

// package events

// import (
// 	"context"
// 	"encoding/json"
// 	"io"
// 	"log"
// 	"naevis/db"
// 	"naevis/globals"
// 	"naevis/mq"
// 	"naevis/structs"
// 	"naevis/userdata"
// 	"naevis/utils"
// 	"net/http"
// 	"os"
// 	"path/filepath"
// 	"strings"
// 	"time"

// 	"github.com/julienschmidt/httprouter"
// 	"go.mongodb.org/mongo-driver/bson"
// )

// var eventpicUploadPath = "./static/eventpic"

// func CreateEvent(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

// 	// Parse multipart form
// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		http.Error(w, "Unable to parse form", http.StatusBadRequest)
// 		return
// 	}

// 	var event structs.Event

// 	// Get event data from form
// 	if r.FormValue("event") == "" {
// 		http.Error(w, "Missing event data", http.StatusBadRequest)
// 		return
// 	}

// 	err := json.Unmarshal([]byte(r.FormValue("event")), &event)
// 	if err != nil {
// 		http.Error(w, "Invalid input", http.StatusBadRequest)
// 		return
// 	}

// 	log.Println("dhnfdg----------", event)

// 	// Retrieve user ID
// 	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
// 	if !ok {
// 		http.Error(w, "Invalid user", http.StatusBadRequest)
// 		return
// 	}
// 	event.CreatorID = requestingUserID
// 	event.CreatedAt = time.Now().UTC() // ✅ Ensure UTC timestamp
// 	event.Date = event.Date.UTC()      // ✅ Force UTC before saving
// 	event.Status = "active"
// 	event.FAQs = []structs.FAQ{}

// 	// Generate a unique EventID
// 	event.EventID = utils.GenerateID(14)

// 	// Check for EventID collisions
// 	// collection := client.Database("eventdb").Collection("events")
// 	exists := db.EventsCollection.FindOne(context.TODO(), bson.M{"eventid": event.EventID}).Err()
// 	if exists == nil {
// 		http.Error(w, "Event ID collision, try again", http.StatusInternalServerError)
// 		return
// 	}

// 	// Parse attached artists (optional)
// 	var artistIDs []string
// 	artistsData := r.FormValue("artists")
// 	if artistsData != "" {
// 		if err := json.Unmarshal([]byte(artistsData), &artistIDs); err != nil {
// 			log.Println("Error parsing artist IDs:", err)
// 			http.Error(w, "Invalid artists data", http.StatusBadRequest)
// 			return
// 		}
// 		event.Artists = artistIDs
// 	}

// 	// Handle the banner image upload (if present)
// 	bannerFile, _, err := r.FormFile("banner")
// 	if err != nil && err != http.ErrMissingFile {
// 		http.Error(w, "Error retrieving banner file", http.StatusBadRequest)
// 		return
// 	}

// 	if bannerFile != nil {
// 		defer bannerFile.Close()

// 		// Validate file type
// 		buff := make([]byte, 512)
// 		if _, err := bannerFile.Read(buff); err != nil {
// 			http.Error(w, "Error reading file", http.StatusInternalServerError)
// 			return
// 		}
// 		contentType := http.DetectContentType(buff)
// 		if !strings.HasPrefix(contentType, "image/") {
// 			http.Error(w, "Invalid file type", http.StatusBadRequest)
// 			return
// 		}
// 		bannerFile.Seek(0, io.SeekStart) // Reset the file pointer

// 		// Ensure the directory exists
// 		if err := os.MkdirAll(eventpicUploadPath, 0755); err != nil {
// 			http.Error(w, "Error creating directory for banner", http.StatusInternalServerError)
// 			return
// 		}

// 		// Sanitize and save the banner image
// 		sanitizedFileName := filepath.Join(eventpicUploadPath, "/banner/", filepath.Base(event.EventID+".jpg"))
// 		out, err := os.Create(sanitizedFileName)
// 		if err != nil {
// 			http.Error(w, "Error saving banner", http.StatusInternalServerError)
// 			return
// 		}
// 		defer out.Close()

// 		if _, err := io.Copy(out, bannerFile); err != nil {
// 			http.Error(w, "Error saving banner", http.StatusInternalServerError)
// 			return
// 		}

// 		// Set the event's banner image field with the saved image path
// 		event.BannerImage = filepath.Base(sanitizedFileName)
// 		thumFile := filepath.Join(eventpicUploadPath, "/banner/")
// 		utils.CreateThumb(event.EventID, thumFile, ".jpg", 300, 200)
// 	}

// 	// Handle the seating image upload (if present)
// 	seatingPlanFile, _, err := r.FormFile("event-seating")
// 	if err != nil && err != http.ErrMissingFile {
// 		http.Error(w, "Error retrieving seating file", http.StatusBadRequest)
// 		return
// 	}

// 	if seatingPlanFile != nil {
// 		defer seatingPlanFile.Close()

// 		// Validate file type
// 		buff := make([]byte, 512)
// 		if _, err := seatingPlanFile.Read(buff); err != nil {
// 			http.Error(w, "Error reading file", http.StatusInternalServerError)
// 			return
// 		}
// 		contentType := http.DetectContentType(buff)
// 		if !strings.HasPrefix(contentType, "image/") {
// 			http.Error(w, "Invalid file type", http.StatusBadRequest)
// 			return
// 		}
// 		seatingPlanFile.Seek(0, io.SeekStart) // Reset the file pointer

// 		// Ensure the directory exists
// 		if err := os.MkdirAll(eventpicUploadPath, 0755); err != nil {
// 			http.Error(w, "Error creating directory for seating plan", http.StatusInternalServerError)
// 			return
// 		}

// 		// Sanitize and save the seating plan image
// 		sanitizedFileName := filepath.Join(eventpicUploadPath, "/seating/", filepath.Base(event.EventID+"seating.jpg"))
// 		out, err := os.Create(sanitizedFileName)
// 		if err != nil {
// 			http.Error(w, "Error saving seating plan", http.StatusInternalServerError)
// 			return
// 		}
// 		defer out.Close()

// 		if _, err := io.Copy(out, seatingPlanFile); err != nil {
// 			http.Error(w, "Error saving seating plan", http.StatusInternalServerError)
// 			return
// 		}

// 		// Set the event's seating plan image field with the saved image path
// 		event.SeatingPlanImage = filepath.Base(sanitizedFileName)
// 	}

// 	// Insert the event into MongoDB
// 	result, err := db.EventsCollection.InsertOne(context.TODO(), event)
// 	if err != nil || result.InsertedID == nil {
// 		log.Printf("Error inserting event into MongoDB: %v", err)
// 		http.Error(w, "Error saving event", http.StatusInternalServerError)
// 		return
// 	}

// 	userdata.SetUserData("event", event.EventID, requestingUserID, "", "")

// 	// ✅ Emit event for messaging queue (if needed)
// 	go mq.Emit("event-created", mq.Index{
// 		EntityType: "event", EntityId: event.EventID, Method: "POST",
// 	})

// 	// ✅ Respond with created event
// 	w.WriteHeader(http.StatusCreated)
// 	if err := json.NewEncoder(w).Encode(event); err != nil {
// 		log.Printf("Error encoding event response: %v", err)
// 		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
// 	}
// }
