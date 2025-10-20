package artists

import (
	"encoding/json"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"strings"

	"naevis/db"
	"naevis/filemgr"
	"naevis/models"
	"naevis/mq"
	"naevis/utils"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func GetArtistByID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	artistId := ps.ByName("id")
	var artist models.Artist

	// Fetch artist info
	if err := db.ArtistsCollection.FindOne(ctx, bson.M{"artistid": artistId}).Decode(&artist); err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "Artist not found")
		return
	}

	// Default: not subscribed
	isSubscribed := false

	// Get current logged-in user ID
	currentUserID := utils.GetUserIDFromRequest(r)
	if currentUserID != "" {
		// Check if the user has subscribed to this artist
		count, err := db.SubscribersCollection.CountDocuments(ctx, bson.M{
			"userid": currentUserID,
			"subscribed": bson.M{
				"$in": []string{artistId},
			},
		})
		if err == nil && count > 0 {
			isSubscribed = true
		}
	}

	// Response struct: embed artist + subscription info
	resp := struct {
		models.Artist
		IsSubscribed bool `json:"isSubscribed"`
	}{
		Artist:       artist,
		IsSubscribed: isSubscribed,
	}

	utils.RespondWithJSON(w, http.StatusOK, resp)
}

// func GetArtistByID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	ctx := r.Context()
// 	artistId := ps.ByName("id")
// 	var artist models.Artist

// 	if err := db.ArtistsCollection.FindOne(ctx, bson.M{"artistid": artistId}).Decode(&artist); err != nil {
// 		utils.RespondWithError(w, http.StatusNotFound, "Artist not found")
// 		return
// 	}

// 	utils.RespondWithJSON(w, http.StatusOK, artist)
// }

func GetArtistsByEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	eventID := ps.ByName("eventid")

	cursor, err := db.ArtistsCollection.Find(ctx, bson.M{"events": eventID})
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Error fetching artists")
		return
	}
	defer cursor.Close(ctx)

	var artists []models.Artist
	if err := cursor.All(ctx, &artists); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Error decoding artists")
		return
	}

	if len(artists) == 0 {
		artists = []models.Artist{}
	}

	utils.RespondWithJSON(w, http.StatusOK, artists)
}

func parseArtistFormData(r *http.Request, existing *models.Artist) (models.Artist, bson.M, []string, error) {
	var artist models.Artist
	updateData := bson.M{}
	filesToDelete := []string{}

	// Preserve IDs when updating
	if existing != nil {
		artist.ArtistID = existing.ArtistID
		artist.EventIDs = existing.EventIDs
	}

	// Simple fields
	assignField := func(key string, target *string, existingVal string) {
		if val := r.FormValue(key); val != "" {
			*target = val
			updateData[key] = val
		} else {
			*target = existingVal
		}
	}

	assignField("name", &artist.Name, existingValue(existing, "Name"))
	assignField("bio", &artist.Bio, existingValue(existing, "Bio"))
	assignField("category", &artist.Category, existingValue(existing, "Category"))
	assignField("dob", &artist.DOB, existingValue(existing, "DOB"))
	assignField("place", &artist.Place, existingValue(existing, "Place"))
	assignField("country", &artist.Country, existingValue(existing, "Country"))

	artist.CreatorID = utils.GetUserIDFromRequest(r)
	if artist.CreatorID != "" {
		updateData["creatorid"] = artist.CreatorID
	} else if existing != nil {
		artist.CreatorID = existing.CreatorID
	}

	// Genres
	if val := r.FormValue("genres"); val != "" {
		var genres []string
		for _, g := range strings.Split(val, ",") {
			if g = strings.TrimSpace(g); g != "" {
				genres = append(genres, g)
			}
		}
		artist.Genres = genres
		updateData["genres"] = genres
	} else if existing != nil {
		artist.Genres = existing.Genres
	}

	// Socials
	if val := r.FormValue("socials"); val != "" {
		var socials map[string]string
		if err := json.Unmarshal([]byte(val), &socials); err == nil {
			artist.Socials = socials
			updateData["socials"] = socials
		} else {
			artist.Socials = map[string]string{"raw": val}
			updateData["socials"] = artist.Socials
		}
	} else if existing != nil {
		artist.Socials = existing.Socials
	}

	// Uploads
	handleImageUpload := func(formKey string, picType filemgr.PictureType, existingFile string) string {
		if img, err := filemgr.SaveFormFile(r.MultipartForm, formKey, filemgr.EntityArtist, picType, false); err == nil && img != "" {
			updateData[formKey] = img
			if existingFile != "" && existingFile != img {
				filesToDelete = append(filesToDelete, filepath.Join(filemgr.ResolvePath(filemgr.EntityArtist, picType), existingFile))
			}
			return img
		}
		return existingFile
	}

	artist.Banner = handleImageUpload("banner", filemgr.PicBanner, existingValue(existing, "Banner"))
	artist.Photo = handleImageUpload("photo", filemgr.PicPhoto, existingValue(existing, "Photo"))

	// Members
	if raw := r.FormValue("members"); raw != "" {
		var members []models.BandMember
		if err := json.Unmarshal([]byte(raw), &members); err == nil {
			for i := range members {
				fileKey := fmt.Sprintf("memberImage_%d", i)
				file, header, err := r.FormFile(fileKey)
				if err == nil {
					defer file.Close()
					filename, ext, ferr := filemgr.SaveFileForEntity(file, header, filemgr.EntityArtist, filemgr.PicMember)
					if ferr != nil {
						return artist, updateData, filesToDelete, fmt.Errorf("failed to save member image: %v", ferr)
					}
					if existing != nil && i < len(existing.Members) && existing.Members[i].Image != filename+ext {
						filesToDelete = append(filesToDelete, filepath.Join(filemgr.ResolvePath(filemgr.EntityArtist, filemgr.PicMember), existing.Members[i].Image))
					}
					members[i].Image = filename + ext
				} else if existing != nil && i < len(existing.Members) {
					members[i].Image = existing.Members[i].Image
				}
			}
			artist.Members = members
			updateData["members"] = members
		}
	} else if existing != nil {
		artist.Members = existing.Members
	}

	return artist, updateData, filesToDelete, nil
}

func CreateArtist(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form data")
		return
	}

	artist, _, _, err := parseArtistFormData(r, nil)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	artist.ArtistID = utils.GenerateRandomString(12)
	artist.EventIDs = []string{}

	if _, err := db.ArtistsCollection.InsertOne(ctx, artist); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create artist")
		return
	}

	go mq.Emit(ctx, "artist-created", models.Index{
		EntityType: "artist", EntityId: artist.ArtistID, Method: "POST",
	})

	utils.RespondWithJSON(w, http.StatusCreated, artist)
}

func UpdateArtist(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	idParam := ps.ByName("id")

	if err := r.ParseMultipartForm(20 << 20); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form data")
		return
	}

	var existing models.Artist
	if err := db.ArtistsCollection.FindOne(ctx, bson.M{"artistid": idParam}).Decode(&existing); err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "Artist not found")
		return
	}

	updated, updateData, filesToDelete, err := parseArtistFormData(r, &existing)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = updated
	if len(updateData) == 0 {
		utils.RespondWithJSON(w, http.StatusOK, bson.M{"message": "No changes detected"})
		return
	}

	_, err = db.ArtistsCollection.UpdateOne(ctx, bson.M{"artistid": idParam}, bson.M{"$set": updateData})
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update artist")
		return
	}

	// Only delete files if DB update succeeded
	for _, path := range filesToDelete {
		_ = os.Remove(path)
	}

	go mq.Emit(ctx, "artist-updated", models.Index{
		EntityType: "artist", EntityId: idParam, Method: "PUT",
	})

	utils.RespondWithJSON(w, http.StatusOK, bson.M{"message": "Artist updated"})
}

func DeleteArtistByID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	artistID := ps.ByName("id")

	if artistID == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "artistID is required")
		return
	}

	filter := bson.M{"artistid": artistID}
	update := bson.M{"$set": bson.M{"deleted": true}}

	_, err := db.ArtistsCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to delete artist")
		return
	}

	go mq.Emit(ctx, "artist-deleted", models.Index{
		EntityType: "artist", EntityId: artistID, Method: "DELETE",
	})

	utils.RespondWithJSON(w, http.StatusOK, bson.M{"message": "Artist deleted successfully"})
}

// Helpers to get existing values without reflection
func existingValue(existing *models.Artist, field string) string {
	if existing == nil {
		return ""
	}
	switch field {
	case "Name":
		return existing.Name
	case "Bio":
		return existing.Bio
	case "Category":
		return existing.Category
	case "DOB":
		return existing.DOB
	case "Place":
		return existing.Place
	case "Country":
		return existing.Country
	case "Banner":
		return existing.Banner
	case "Photo":
		return existing.Photo
	default:
		return ""
	}
}
