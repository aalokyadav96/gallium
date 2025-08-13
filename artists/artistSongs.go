package artists

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"naevis/db"
	"naevis/filemgr"
	"naevis/models"
	"naevis/mq"
	"naevis/utils"
	_ "net/http/pprof"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func PostNewSong(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	artistID := ps.ByName("id")

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid multipart form")
		return
	}

	audioFile, err := filemgr.SaveFormFile(r.MultipartForm, "audio", filemgr.EntitySong, filemgr.PicAudio, true)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	posterFile, err := filemgr.SaveFormFile(r.MultipartForm, "poster", filemgr.EntitySong, filemgr.PicPhoto, false)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	formFields, _ := collectSongFieldsFromForm(r)

	newSong := models.ArtistSong{
		SongID:      utils.GenerateRandomString(12),
		ArtistID:    artistID,
		Title:       formFields["title"],
		Genre:       formFields["genre"],
		Duration:    formFields["duration"],
		Description: formFields["description"],
		AudioURL:    audioFile,
		Poster:      posterFile,
		Published:   true,
		Plays:       0,
		UploadedAt:  time.Now(),
	}

	filter := bson.M{"artistid": artistID}
	update := bson.M{
		"$push":        bson.M{"songs": newSong},
		"$setOnInsert": bson.M{"artistid": artistID},
	}
	opts := options.Update().SetUpsert(true)

	if _, err := db.SongsCollection.UpdateOne(context.TODO(), filter, update, opts); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to add song to artist")
		return
	}

	// ✅ Emit event for messaging queue (if needed)
	go mq.Emit(ctx, "song-created", models.Index{
		EntityType: "song", EntityId: newSong.SongID, Method: "POST",
	})
	utils.RespondWithJSON(w, http.StatusCreated, newSong)
}

func EditSong(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	artistID := ps.ByName("id")
	songID := ps.ByName("songId")

	if songID == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "songId is required")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid form data")
		return
	}

	formFields, _ := collectSongFieldsFromForm(r)
	updateFields := bson.M{}
	for key, value := range formFields {
		if value != "" {
			updateFields["songs.$."+key] = value
		}
	}

	audioFile, err := filemgr.SaveFormFile(r.MultipartForm, "audio", filemgr.EntitySong, filemgr.PicAudio, false)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if audioFile != "" {
		updateFields["songs.$.audioUrl"] = audioFile
	}

	posterFile, err := filemgr.SaveFormFile(r.MultipartForm, "poster", filemgr.EntitySong, filemgr.PicPhoto, false)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if posterFile != "" {
		updateFields["songs.$.poster"] = posterFile
	}

	if len(updateFields) == 0 {
		utils.RespondWithError(w, http.StatusBadRequest, "No fields to update")
		return
	}

	filter := bson.M{"artistid": artistID, "songs.songid": songID}
	update := bson.M{"$set": updateFields}

	res, err := db.SongsCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil || res.MatchedCount == 0 {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update song")
		return
	}

	// ✅ Emit event for messaging queue (if needed)
	go mq.Emit(ctx, "song-updated", models.Index{
		EntityType: "song", EntityId: songID, Method: "PUT",
	})
	utils.RespondWithJSON(w, http.StatusOK, bson.M{"message": "Song updated successfully"})
}

// collectSongFieldsFromForm collects form fields for a song.
func collectSongFieldsFromForm(r *http.Request) (map[string]string, error) {
	fields := map[string]string{
		"title":       r.FormValue("title"),
		"genre":       r.FormValue("genre"),
		"duration":    r.FormValue("duration"),
		"description": r.FormValue("description"),
	}
	return fields, nil
}

// DeleteSong removes a song by its songid.
func DeleteSong(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	artistID := ps.ByName("id")
	// songID := r.URL.Query().Get("songId")
	songID := ps.ByName("songId")

	if songID == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "songId is required")
		return
	}

	filter := bson.M{"artistid": artistID}
	update := bson.M{"$pull": bson.M{"songs": bson.M{"songid": songID}}}

	_, err := db.SongsCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to delete song")
		return
	}

	// ✅ Emit event for messaging queue (if needed)
	go mq.Emit(ctx, "song-deleted", models.Index{
		EntityType: "song", EntityId: songID, Method: "DELETE",
	})
	utils.RespondWithJSON(w, http.StatusOK, bson.M{"message": "Song deleted successfully"})
}

// GetArtistsSongs returns all published songs for an artist.
// If no songs exist, returns an empty array.
func GetArtistsSongs(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	artistID := ps.ByName("id")

	var result struct {
		Songs []models.ArtistSong `bson:"songs"`
	}
	fmt.Println("---------------------", artistID)
	// ignore errors; result.Songs will be nil if no document found
	err := db.SongsCollection.FindOne(context.TODO(), bson.M{"artistid": artistID}).Decode(&result)

	if err != nil {
		fmt.Println(err)
	}

	filtered := make([]models.ArtistSong, 0, len(result.Songs))
	for _, s := range result.Songs {
		if s.Published {
			filtered = append(filtered, s)
		}
	}

	utils.RespondWithJSON(w, http.StatusOK, filtered)
}
