package artists

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"strings"
	"time"

	"naevis/db"
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

func CreateArtist(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form data")
		return
	}

	// Parse socials
	var socials map[string]string
	socialsData := r.FormValue("socials")
	if socialsData != "" {
		if err := json.Unmarshal([]byte(socialsData), &socials); err != nil {
			socials = map[string]string{"default": socialsData}
		}
	}

	// Parse members
	var members []models.BandMember
	membersData := r.FormValue("members")
	if membersData != "" {
		if err := json.Unmarshal([]byte(membersData), &members); err != nil {
			members = []models.BandMember{}
		}
	}

	// Handle member images
	for i := range members {
		fileKey := fmt.Sprintf("memberImage_%d", i)
		file, header, err := r.FormFile(fileKey)
		if err == nil && header != nil {
			defer file.Close()
			path, err := saveFile(file, header, "./static/artistpic/members")
			if err != nil {
				utils.RespondWithError(w, http.StatusInternalServerError, "Error saving member image")
				return
			}
			members[i].Image = path
		}
	}

	// Parse genres safely
	rawGenres := strings.TrimSpace(r.FormValue("genres"))
	var genres []string
	if rawGenres != "" {
		for _, genre := range strings.Split(rawGenres, ",") {
			g := strings.TrimSpace(genre)
			if g != "" {
				genres = append(genres, g)
			}
		}
	}

	artist := models.Artist{
		ArtistID: utils.GenerateID(12),
		Name:     r.FormValue("name"),
		Bio:      r.FormValue("bio"),
		Category: r.FormValue("category"),
		DOB:      r.FormValue("dob"),
		Place:    r.FormValue("place"),
		Country:  r.FormValue("country"),
		Genres:   genres,
		Socials:  socials,
		Members:  members,
		EventIDs: []string{}, // initialize to empty slice
	}

	// Handle banner
	if bannerFile, bannerHeader, err := r.FormFile("banner"); err == nil && bannerHeader != nil {
		defer bannerFile.Close()
		bannerPath, err := saveFile(bannerFile, bannerHeader, "./static/artistpic/banner")
		if err != nil {
			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving banner")
			return
		}
		artist.Banner = bannerPath
	}

	// Handle photo
	if photoFile, photoHeader, err := r.FormFile("photo"); err == nil && photoHeader != nil {
		defer photoFile.Close()
		photoPath, err := saveFile(photoFile, photoHeader, "./static/artistpic/photo")
		if err != nil {
			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving photo")
			return
		}
		artist.Photo = photoPath
	}

	// Insert into DB
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.ArtistsCollection.InsertOne(ctx, artist)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create artist")
		return
	}

	utils.RespondWithJSON(w, http.StatusCreated, artist)
}

// func CreateArtist(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form data")
// 		return
// 	}

// 	var socials map[string]string
// 	socialsData := r.FormValue("socials")
// 	if socialsData != "" {
// 		if err := json.Unmarshal([]byte(socialsData), &socials); err != nil {
// 			socials = map[string]string{"default": socialsData}
// 		}
// 	}

// 	var members []models.BandMember
// 	membersData := r.FormValue("members")
// 	if membersData != "" {
// 		if err := json.Unmarshal([]byte(membersData), &members); err != nil {
// 			members = []models.BandMember{}
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
// 		Genres:   strings.Split(r.FormValue("genres"), ","),
// 		Socials:  socials,
// 		Members:  members,
// 	}

// 	if bannerFile, bannerHeader, err := r.FormFile("banner"); err == nil {
// 		defer bannerFile.Close()
// 		bannerPath, err := saveFile(bannerFile, bannerHeader, "./static/artistpic/banner")
// 		if err != nil {
// 			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving banner")
// 			return
// 		}
// 		artist.Banner = bannerPath
// 	}

// 	if photoFile, photoHeader, err := r.FormFile("photo"); err == nil {
// 		defer photoFile.Close()
// 		photoPath, err := saveFile(photoFile, photoHeader, "./static/artistpic/photo")
// 		if err != nil {
// 			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving photo")
// 			return
// 		}
// 		artist.Photo = photoPath
// 	}

// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	_, err := db.ArtistsCollection.InsertOne(ctx, artist)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create artist")
// 		return
// 	}

// 	// // 🔹 Insert initial empty song list for artist
// 	// _, err = db.ArtistSongsCollection.InsertOne(ctx, bson.M{
// 	// 	"artistid": artist.ArtistID,
// 	// 	"songs":    []interface{}{},
// 	// })
// 	// if err != nil {
// 	// 	utils.RespondWithError(w, http.StatusInternalServerError, "Failed to initialize artist song data")
// 	// 	return
// 	// }

//		utils.RespondWithJSON(w, http.StatusCreated, artist)
//	}

// func UpdateArtist(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	idParam := ps.ByName("id")

// 	if err := r.ParseMultipartForm(20 << 20); err != nil { // 20 MB max
// 		utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form data")
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

// 	// ✅ Handle standard fields
// 	for key, value := range formFields {
// 		if strings.TrimSpace(value) == "" {
// 			continue
// 		}
// 		switch key {
// 		case "genres":
// 			updateData["genres"] = strings.Split(value, ",")
// 		case "socials":
// 			var socials map[string]string
// 			if err := json.Unmarshal([]byte(value), &socials); err != nil {
// 				socials = map[string]string{"raw": value}
// 			}
// 			updateData["socials"] = socials
// 		default:
// 			updateData[key] = value
// 		}
// 	}

// 	// ✅ Handle banner image
// 	if bannerFile, bannerHeader, err := r.FormFile("banner"); err == nil {
// 		defer bannerFile.Close()
// 		bannerPath, err := saveFile(bannerFile, bannerHeader, "./static/artistpic/banner")
// 		if err != nil {
// 			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving banner")
// 			return
// 		}
// 		updateData["banner"] = bannerPath
// 	}

// 	// ✅ Handle photo image
// 	if photoFile, photoHeader, err := r.FormFile("photo"); err == nil {
// 		defer photoFile.Close()
// 		photoPath, err := saveFile(photoFile, photoHeader, "./static/artistpic/photo")
// 		if err != nil {
// 			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving photo")
// 			return
// 		}
// 		updateData["photo"] = photoPath
// 	}

// 	// ✅ Handle band members and their images
// 	membersRaw := r.FormValue("members")
// 	if membersRaw != "" {
// 		var members []models.BandMember
// 		if err := json.Unmarshal([]byte(membersRaw), &members); err == nil {
// 			for i := range members {
// 				fileKey := fmt.Sprintf("memberImage_%d", i)
// 				if memberFile, memberHeader, err := r.FormFile(fileKey); err == nil {
// 					defer memberFile.Close()
// 					imgPath, err := saveFile(memberFile, memberHeader, "./static/artistpic/members")
// 					if err != nil {
// 						utils.RespondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Error saving image for member %d", i))
// 						return
// 					}
// 					members[i].Image = imgPath
// 				}
// 			}
// 			updateData["members"] = members
// 		}
// 	}

// 	// ✅ Perform update
// 	update := bson.M{"$set": updateData}
// 	filter := bson.M{"artistid": idParam}

// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	result, err := db.ArtistsCollection.UpdateOne(ctx, filter, update)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update artist")
// 		return
// 	}

// 	if result.MatchedCount == 0 {
// 		utils.RespondWithError(w, http.StatusNotFound, "Artist not found")
// 		return
// 	}

// 	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Artist updated"})
// }

// // func UpdateArtist(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// // 	idParam := ps.ByName("id")

// // 	// Parse the multipart form data (max 10MB)
// // 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// // 		utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form data")
// // 		return
// // 	}

// // 	updateData := bson.M{}
// // 	formFields := map[string]string{
// // 		"category": r.FormValue("category"),
// // 		"name":     r.FormValue("name"),
// // 		"bio":      r.FormValue("bio"),
// // 		"dob":      r.FormValue("dob"),
// // 		"place":    r.FormValue("place"),
// // 		"country":  r.FormValue("country"),
// // 		"genres":   r.FormValue("genres"),
// // 		"socials":  r.FormValue("socials"),
// // 	}

// // 	// Handle each form field
// // 	for key, value := range formFields {
// // 		if value != "" {
// // 			switch key {
// // 			case "genres":
// // 				updateData["genres"] = strings.Split(value, ",")
// // 			case "socials":
// // 				var socials map[string]string
// // 				if err := json.Unmarshal([]byte(value), &socials); err != nil {
// // 					socials = map[string]string{"default": value}
// // 				}
// // 				updateData["socials"] = socials
// // 			default:
// // 				updateData[key] = value
// // 			}
// // 		}
// // 	}

// // 	// Handle file uploads

// // 	if bannerFile, bannerHeader, err := r.FormFile("banner"); err == nil {
// // 		defer bannerFile.Close()
// // 		bannerPath, err := saveFile(bannerFile, bannerHeader, "./static/artistpic/banner")
// // 		if err != nil {
// // 			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving banner")
// // 			return
// // 		}
// // 		updateData["banner"] = bannerPath
// // 	}

// // 	if photoFile, photoHeader, err := r.FormFile("photo"); err == nil {
// // 		defer photoFile.Close()
// // 		photoPath, err := saveFile(photoFile, photoHeader, "./static/artistpic/photo")
// // 		if err != nil {
// // 			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving photo")
// // 			return
// // 		}
// // 		updateData["photo"] = photoPath
// // 	}

// // 	membersData := r.FormValue("members")
// // 	if membersData != "" {
// // 		var members []models.BandMember
// // 		if err := json.Unmarshal([]byte(membersData), &members); err == nil {
// // 			updateData["members"] = members
// // 		}
// // 	}

// // 	update := bson.M{"$set": updateData}

// // 	// Update in MongoDB using "id" field (string, not ObjectID)
// // 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// // 	defer cancel()

// // 	filter := bson.M{"artistid": idParam}
// // 	result, err := db.ArtistsCollection.UpdateOne(ctx, filter, update)
// // 	if err != nil {
// // 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update artist")
// // 		return
// // 	}

// // 	if result.MatchedCount == 0 {
// // 		utils.RespondWithError(w, http.StatusNotFound, "Artist not found")
// // 		return
// // 	}

// // 	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Artist updated"})
// // }
func UpdateArtist(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	idParam := ps.ByName("id")

	if err := r.ParseMultipartForm(20 << 20); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse form data")
		return
	}

	updateData := bson.M{}
	formFields := map[string]string{
		"category": r.FormValue("category"),
		"name":     r.FormValue("name"),
		"bio":      r.FormValue("bio"),
		"dob":      r.FormValue("dob"),
		"place":    r.FormValue("place"),
		"country":  r.FormValue("country"),
		"genres":   r.FormValue("genres"),
		"socials":  r.FormValue("socials"),
	}

	// 🧠 Fetch existing artist for comparison and cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var existing models.Artist
	err := db.ArtistsCollection.FindOne(ctx, bson.M{"artistid": idParam}).Decode(&existing)
	if err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "Artist not found")
		return
	}

	// ✅ Process text fields
	for key, value := range formFields {
		if strings.TrimSpace(value) == "" {
			continue
		}
		switch key {
		case "genres":
			updateData["genres"] = strings.Split(value, ",")
		case "socials":
			var socials map[string]string
			if err := json.Unmarshal([]byte(value), &socials); err != nil {
				socials = map[string]string{"raw": value}
			}
			updateData["socials"] = socials
		default:
			updateData[key] = value
		}
	}

	// ✅ Handle banner image replacement
	if bannerFile, bannerHeader, err := r.FormFile("banner"); err == nil {
		defer bannerFile.Close()
		bannerPath, err := saveFile(bannerFile, bannerHeader, "./static/artistpic/banner")
		if err != nil {
			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving banner")
			return
		}
		updateData["banner"] = bannerPath
		if existing.Banner != "" && existing.Banner != bannerPath {
			_ = os.Remove(existing.Banner)
		}
	}

	// ✅ Handle photo image replacement
	if photoFile, photoHeader, err := r.FormFile("photo"); err == nil {
		defer photoFile.Close()
		photoPath, err := saveFile(photoFile, photoHeader, "./static/artistpic/photo")
		if err != nil {
			utils.RespondWithError(w, http.StatusInternalServerError, "Error saving photo")
			return
		}
		updateData["photo"] = photoPath
		if existing.Photo != "" && existing.Photo != photoPath {
			_ = os.Remove(existing.Photo)
		}
	}

	// ✅ Handle band members and their image replacement
	membersRaw := r.FormValue("members")
	if membersRaw != "" {
		var newMembers []models.BandMember
		if err := json.Unmarshal([]byte(membersRaw), &newMembers); err == nil {
			for i := range newMembers {
				fileKey := fmt.Sprintf("memberImage_%d", i)
				if memberFile, memberHeader, err := r.FormFile(fileKey); err == nil {
					defer memberFile.Close()
					imgPath, err := saveFile(memberFile, memberHeader, "./static/artistpic/members")
					if err != nil {
						utils.RespondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Error saving image for member %d", i))
						return
					}
					// remove old image if exists and changed
					if i < len(existing.Members) && existing.Members[i].Image != "" && existing.Members[i].Image != imgPath {
						_ = os.Remove(existing.Members[i].Image)
					}
					newMembers[i].Image = imgPath
				} else if i < len(existing.Members) {
					// retain old image if not replaced
					newMembers[i].Image = existing.Members[i].Image
				}
			}
			updateData["members"] = newMembers
		}
	}

	// ✅ Return early if no changes
	if len(updateData) == 0 {
		utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "No changes detected"})
		return
	}

	// ✅ Perform MongoDB update
	update := bson.M{"$set": updateData}
	filter := bson.M{"artistid": idParam}

	result, err := db.ArtistsCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update artist")
		return
	}

	if result.MatchedCount == 0 {
		utils.RespondWithError(w, http.StatusNotFound, "Artist not found")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Artist updated"})
}

func saveFile(file multipart.File, header *multipart.FileHeader, folder string) (string, error) {
	filename := fmt.Sprintf("%s%s", utils.GenerateID(12), filepath.Ext(header.Filename))
	filePath := fmt.Sprintf("%s/%s", folder, filename)
	fmt.Println("_____________", filePath)
	out, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		return "", err
	}

	return filename, nil
}

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
