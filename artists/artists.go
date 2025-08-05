package artists

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"strings"
	"time"

	"naevis/db"
	"naevis/filemgr"
	"naevis/models"
	"naevis/utils"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func GetAllArtists(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := db.ArtistsCollection.Find(ctx, bson.M{})
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Error fetching artists")
		return
	}
	defer cursor.Close(ctx)

	var artists []models.Artist
	if err := cursor.All(ctx, &artists); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Error parsing artist data")
		return
	}

	if len(artists) == 0 {
		artists = []models.Artist{}
	}

	utils.RespondWithJSON(w, http.StatusOK, artists)
}

func GetArtistByID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	artistId := ps.ByName("id")
	// objID, err := primitive.ObjectIDFromHex(idParam)
	// if err != nil {
	// 	utils.RespondWithError(w, http.StatusBadRequest, "Invalid artist ID")
	// 	return
	// }

	var artist models.Artist
	err := db.ArtistsCollection.FindOne(r.Context(), bson.M{"artistid": artistId}).Decode(&artist)
	if err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "Artist not found")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, artist)
}

func GetArtistsByEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventIDParam := ps.ByName("eventid")
	// eventObjID, err := primitive.ObjectIDFromHex(eventIDParam)
	// if err != nil {
	// 	utils.RespondWithError(w, http.StatusBadRequest, "Invalid event ID")
	// 	return
	// }

	filter := bson.M{"events": eventIDParam}
	cursor, err := db.ArtistsCollection.Find(r.Context(), filter)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Error fetching artists")
		return
	}
	defer cursor.Close(r.Context())

	var artists []models.Artist
	if err := cursor.All(r.Context(), &artists); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Error decoding artist list")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, artists)
}
func parseArtistFormData(r *http.Request, existing *models.Artist) (models.Artist, bson.M, error) {
	var artist models.Artist
	var updateData bson.M = bson.M{}

	artist.Name = r.FormValue("name")
	artist.Bio = r.FormValue("bio")
	artist.Category = r.FormValue("category")
	artist.DOB = r.FormValue("dob")
	artist.Place = r.FormValue("place")
	artist.Country = r.FormValue("country")

	if val := r.FormValue("genres"); val != "" {
		genres := []string{}
		for _, g := range strings.Split(val, ",") {
			g = strings.TrimSpace(g)
			if g != "" {
				genres = append(genres, g)
			}
		}
		artist.Genres = genres
		updateData["genres"] = genres
	}

	if val := r.FormValue("socials"); val != "" {
		var socials map[string]string
		if err := json.Unmarshal([]byte(val), &socials); err == nil {
			artist.Socials = socials
			updateData["socials"] = socials
		} else {
			artist.Socials = map[string]string{"raw": val}
			updateData["socials"] = artist.Socials
		}
	}

	// Banner upload
	if banner, err := filemgr.SaveFormFile(r.MultipartForm, "banner", filemgr.EntityArtist, filemgr.PicBanner, false); err == nil && banner != "" {
		artist.Banner = banner
		updateData["banner"] = banner
		if existing != nil && existing.Banner != "" && existing.Banner != banner {
			_ = os.Remove(filepath.Join(filemgr.ResolvePath(filemgr.EntityArtist, filemgr.PicBanner), existing.Banner))
		}
	} else if existing != nil {
		artist.Banner = existing.Banner
	}

	// Photo upload
	if photo, err := filemgr.SaveFormFile(r.MultipartForm, "photo", filemgr.EntityArtist, filemgr.PicPhoto, false); err == nil && photo != "" {
		artist.Photo = photo
		updateData["photo"] = photo
		if existing != nil && existing.Photo != "" && existing.Photo != photo {
			_ = os.Remove(filepath.Join(filemgr.ResolvePath(filemgr.EntityArtist, filemgr.PicPhoto), existing.Photo))
		}
	} else if existing != nil {
		artist.Photo = existing.Photo
	}

	// Parse members and handle member image uploads
	var members []models.BandMember
	if raw := r.FormValue("members"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &members); err == nil {
			for i := range members {
				fileKey := fmt.Sprintf("memberImage_%d", i)
				file, header, err := r.FormFile(fileKey)
				if err == nil {
					defer file.Close()
					filename, err := filemgr.SaveFileForEntity(file, header, filemgr.EntityArtist, filemgr.PicMember)
					if err != nil {
						return artist, updateData, fmt.Errorf("failed to save member image: %v", err)
					}
					// Remove old image if changed
					if existing != nil && i < len(existing.Members) && existing.Members[i].Image != filename {
						_ = os.Remove(filepath.Join(filemgr.ResolvePath(filemgr.EntityArtist, filemgr.PicMember), existing.Members[i].Image))
					}
					members[i].Image = filename
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

	return artist, updateData, nil
}

func CreateArtist(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form data")
		return
	}

	artist, _, err := parseArtistFormData(r, nil)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	artist.ArtistID = utils.GenerateID(12)
	artist.EventIDs = []string{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := db.ArtistsCollection.InsertOne(ctx, artist); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create artist")
		return
	}

	utils.RespondWithJSON(w, http.StatusCreated, artist)
}

func UpdateArtist(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	idParam := ps.ByName("id")

	if err := r.ParseMultipartForm(20 << 20); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form data")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var existing models.Artist
	if err := db.ArtistsCollection.FindOne(ctx, bson.M{"artistid": idParam}).Decode(&existing); err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "Artist not found")
		return
	}

	updated, updateData, err := parseArtistFormData(r, &existing)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_ = updated

	if len(updateData) == 0 {
		utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "No changes detected"})
		return
	}

	_, err = db.ArtistsCollection.UpdateOne(ctx, bson.M{"artistid": idParam}, bson.M{"$set": updateData})
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update artist")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Artist updated"})
}

// func CreateArtist(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form data")
// 		return
// 	}

// 	var socials map[string]string
// 	if data := r.FormValue("socials"); data != "" {
// 		if err := json.Unmarshal([]byte(data), &socials); err != nil {
// 			socials = map[string]string{"default": data}
// 		}
// 	}

// 	var members []models.BandMember
// 	if data := r.FormValue("members"); data != "" {
// 		if err := json.Unmarshal([]byte(data), &members); err != nil {
// 			members = []models.BandMember{}
// 		}
// 	}

// 	for i := range members {
// 		fileKey := fmt.Sprintf("memberImage_%d", i)
// 		file, header, err := r.FormFile(fileKey)
// 		if err == nil {
// 			defer file.Close()
// 			filename, err := filemgr.SaveFileForEntity(file, header, filemgr.EntityArtist, filemgr.PicMember)
// 			if err != nil {
// 				utils.RespondWithError(w, http.StatusInternalServerError, "Error saving member image")
// 				return
// 			}
// 			members[i].Image = filename
// 		}
// 	}

// 	var genres []string
// 	if raw := strings.TrimSpace(r.FormValue("genres")); raw != "" {
// 		for _, g := range strings.Split(raw, ",") {
// 			g = strings.TrimSpace(g)
// 			if g != "" {
// 				genres = append(genres, g)
// 			}
// 		}
// 	}

// 	artist := models.Artist{
// 		ArtistID: utils.GenerateID(12),
// 		Name:     r.FormValue("name"),
// 		Bio:      r.FormValue("bio"),
// 		Category: r.FormValue("category"),
// 		DOB:      r.FormValue("dob"),
// 		Place:    r.FormValue("place"),
// 		Country:  r.FormValue("country"),
// 		Genres:   genres,
// 		Socials:  socials,
// 		Members:  members,
// 		EventIDs: []string{},
// 	}

// 	if banner, err := filemgr.SaveFormFile(r.MultipartForm, "banner", filemgr.EntityArtist, filemgr.PicBanner, false); err == nil && banner != "" {
// 		artist.Banner = banner
// 	}
// 	if photo, err := filemgr.SaveFormFile(r.MultipartForm, "photo", filemgr.EntityArtist, filemgr.PicPhoto, false); err == nil && photo != "" {
// 		artist.Photo = photo
// 	}

// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	if _, err := db.ArtistsCollection.InsertOne(ctx, artist); err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create artist")
// 		return
// 	}

// 	utils.RespondWithJSON(w, http.StatusCreated, artist)
// }

// func UpdateArtist(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	idParam := ps.ByName("id")

// 	if err := r.ParseMultipartForm(20 << 20); err != nil {
// 		utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form data")
// 		return
// 	}

// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	var existing models.Artist
// 	if err := db.ArtistsCollection.FindOne(ctx, bson.M{"artistid": idParam}).Decode(&existing); err != nil {
// 		utils.RespondWithError(w, http.StatusNotFound, "Artist not found")
// 		return
// 	}

// 	updateData := bson.M{}
// 	formFields := map[string]string{
// 		"category": r.FormValue("category"),
// 		"name":     r.FormValue("name"),
// 		"bio":      r.FormValue("bio"),
// 		"dob":      r.FormValue("dob"),
// 		"place":    r.FormValue("place"),
// 		"country":  r.FormValue("country"),
// 		"genres":   r.FormValue("genres"),
// 		"socials":  r.FormValue("socials"),
// 	}

// 	for key, val := range formFields {
// 		if strings.TrimSpace(val) == "" {
// 			continue
// 		}
// 		switch key {
// 		case "genres":
// 			updateData["genres"] = strings.Split(val, ",")
// 		case "socials":
// 			var socials map[string]string
// 			if err := json.Unmarshal([]byte(val), &socials); err != nil {
// 				socials = map[string]string{"raw": val}
// 			}
// 			updateData["socials"] = socials
// 		default:
// 			updateData[key] = val
// 		}
// 	}

// 	if banner, err := filemgr.SaveFormFile(r.MultipartForm, "banner", filemgr.EntityArtist, filemgr.PicBanner, false); err == nil && banner != "" {
// 		updateData["banner"] = banner
// 		if existing.Banner != "" && existing.Banner != banner {
// 			_ = os.Remove(filepath.Join(filemgr.ResolvePath(filemgr.EntityArtist, filemgr.PicBanner), existing.Banner))
// 		}
// 	}

// 	if photo, err := filemgr.SaveFormFile(r.MultipartForm, "photo", filemgr.EntityArtist, filemgr.PicPhoto, false); err == nil && photo != "" {
// 		updateData["photo"] = photo
// 		if existing.Photo != "" && existing.Photo != photo {
// 			_ = os.Remove(filepath.Join(filemgr.ResolvePath(filemgr.EntityArtist, filemgr.PicPhoto), existing.Photo))
// 		}
// 	}

// 	if membersRaw := r.FormValue("members"); membersRaw != "" {
// 		var newMembers []models.BandMember
// 		if err := json.Unmarshal([]byte(membersRaw), &newMembers); err == nil {
// 			for i := range newMembers {
// 				fileKey := fmt.Sprintf("memberImage_%d", i)
// 				file, header, err := r.FormFile(fileKey)
// 				if err == nil {
// 					defer file.Close()
// 					filename, err := filemgr.SaveFileForEntity(file, header, filemgr.EntityArtist, filemgr.PicMember)
// 					if err != nil {
// 						utils.RespondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save image for member %d", i))
// 						return
// 					}
// 					if i < len(existing.Members) && existing.Members[i].Image != filename {
// 						_ = os.Remove(filepath.Join(filemgr.ResolvePath(filemgr.EntityArtist, filemgr.PicMember), existing.Members[i].Image))
// 					}
// 					newMembers[i].Image = filename
// 				} else if i < len(existing.Members) {
// 					newMembers[i].Image = existing.Members[i].Image
// 				}
// 			}
// 			updateData["members"] = newMembers
// 		}
// 	}

// 	if len(updateData) == 0 {
// 		utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "No changes detected"})
// 		return
// 	}

// 	result, err := db.ArtistsCollection.UpdateOne(ctx, bson.M{"artistid": idParam}, bson.M{"$set": updateData})
// 	if err != nil || result.MatchedCount == 0 {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update artist")
// 		return
// 	}

// 	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Artist updated"})
// }

// func CreateArtist(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form data")
// 		return
// 	}

// 	var socials map[string]string
// 	if data := r.FormValue("socials"); data != "" {
// 		if err := json.Unmarshal([]byte(data), &socials); err != nil {
// 			socials = map[string]string{"default": data}
// 		}
// 	}

// 	var members []models.BandMember
// 	if data := r.FormValue("members"); data != "" {
// 		if err := json.Unmarshal([]byte(data), &members); err != nil {
// 			members = []models.BandMember{}
// 		}
// 	}

// 	for i := range members {
// 		fileKey := fmt.Sprintf("memberImage_%d", i)
// 		file, header, err := r.FormFile(fileKey)
// 		if err == nil {
// 			defer file.Close()
// 			filename, err := filemgr.SaveFile(file, header, "static/artistpic/members")
// 			if err != nil {
// 				utils.RespondWithError(w, http.StatusInternalServerError, "Error saving member image")
// 				return
// 			}
// 			members[i].Image = filename
// 		}
// 	}

// 	var genres []string
// 	if raw := strings.TrimSpace(r.FormValue("genres")); raw != "" {
// 		for _, g := range strings.Split(raw, ",") {
// 			g = strings.TrimSpace(g)
// 			if g != "" {
// 				genres = append(genres, g)
// 			}
// 		}
// 	}

// 	artist := models.Artist{
// 		ArtistID: utils.GenerateID(12),
// 		Name:     r.FormValue("name"),
// 		Bio:      r.FormValue("bio"),
// 		Category: r.FormValue("category"),
// 		DOB:      r.FormValue("dob"),
// 		Place:    r.FormValue("place"),
// 		Country:  r.FormValue("country"),
// 		Genres:   genres,
// 		Socials:  socials,
// 		Members:  members,
// 		EventIDs: []string{},
// 	}

// 	if banner, err := filemgr.SaveFormFile(r, "banner", "static/artistpic/banner", false); err == nil && banner != "" {
// 		artist.Banner = banner
// 	}
// 	if photo, err := filemgr.SaveFormFile(r, "photo", "static/artistpic/photo", false); err == nil && photo != "" {
// 		artist.Photo = photo
// 	}

// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	if _, err := db.ArtistsCollection.InsertOne(ctx, artist); err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create artist")
// 		return
// 	}

// 	utils.RespondWithJSON(w, http.StatusCreated, artist)
// }

// func UpdateArtist(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	idParam := ps.ByName("id")

// 	if err := r.ParseMultipartForm(20 << 20); err != nil {
// 		utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form data")
// 		return
// 	}

// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	var existing models.Artist
// 	if err := db.ArtistsCollection.FindOne(ctx, bson.M{"artistid": idParam}).Decode(&existing); err != nil {
// 		utils.RespondWithError(w, http.StatusNotFound, "Artist not found")
// 		return
// 	}

// 	updateData := bson.M{}
// 	formFields := map[string]string{
// 		"category": r.FormValue("category"),
// 		"name":     r.FormValue("name"),
// 		"bio":      r.FormValue("bio"),
// 		"dob":      r.FormValue("dob"),
// 		"place":    r.FormValue("place"),
// 		"country":  r.FormValue("country"),
// 		"genres":   r.FormValue("genres"),
// 		"socials":  r.FormValue("socials"),
// 	}

// 	for key, val := range formFields {
// 		if strings.TrimSpace(val) == "" {
// 			continue
// 		}
// 		switch key {
// 		case "genres":
// 			updateData["genres"] = strings.Split(val, ",")
// 		case "socials":
// 			var socials map[string]string
// 			if err := json.Unmarshal([]byte(val), &socials); err != nil {
// 				socials = map[string]string{"raw": val}
// 			}
// 			updateData["socials"] = socials
// 		default:
// 			updateData[key] = val
// 		}
// 	}

// 	if banner, err := filemgr.SaveFormFile(r, "banner", "static/artistpic/banner", false); err == nil && banner != "" {
// 		updateData["banner"] = banner
// 		if existing.Banner != "" && existing.Banner != banner {
// 			_ = os.Remove(filepath.Join("static/artistpic/banner", existing.Banner))
// 		}
// 	}

// 	if photo, err := filemgr.SaveFormFile(r, "photo", "static/artistpic/photo", false); err == nil && photo != "" {
// 		updateData["photo"] = photo
// 		if existing.Photo != "" && existing.Photo != photo {
// 			_ = os.Remove(filepath.Join("static/artistpic/photo", existing.Photo))
// 		}
// 	}

// 	if membersRaw := r.FormValue("members"); membersRaw != "" {
// 		var newMembers []models.BandMember
// 		if err := json.Unmarshal([]byte(membersRaw), &newMembers); err == nil {
// 			for i := range newMembers {
// 				fileKey := fmt.Sprintf("memberImage_%d", i)
// 				file, header, err := r.FormFile(fileKey)
// 				if err == nil {
// 					defer file.Close()
// 					filename, err := filemgr.SaveFile(file, header, "static/artistpic/members")
// 					if err != nil {
// 						utils.RespondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save image for member %d", i))
// 						return
// 					}
// 					if i < len(existing.Members) && existing.Members[i].Image != filename {
// 						_ = os.Remove(filepath.Join("static/artistpic/members", existing.Members[i].Image))
// 					}
// 					newMembers[i].Image = filename
// 				} else if i < len(existing.Members) {
// 					newMembers[i].Image = existing.Members[i].Image
// 				}
// 			}
// 			updateData["members"] = newMembers
// 		}
// 	}

// 	if len(updateData) == 0 {
// 		utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "No changes detected"})
// 		return
// 	}

// 	result, err := db.ArtistsCollection.UpdateOne(ctx, bson.M{"artistid": idParam}, bson.M{"$set": updateData})
// 	if err != nil || result.MatchedCount == 0 {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update artist")
// 		return
// 	}

// 	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Artist updated"})
// }

func DeleteArtistByID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	artistID := ps.ByName("id")

	if artistID == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "artistID is required")
		return
	}

	filter := bson.M{"artistid": artistID}
	update := bson.M{"deleted": true}

	_, err := db.ArtistsCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to delete artist")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, bson.M{"message": "Artist deleted successfully"})
}
