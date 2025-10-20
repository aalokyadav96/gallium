package artists

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"naevis/db"
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

	var payload struct {
		Title       string `json:"title"`
		Genre       string `json:"genre"`
		Duration    string `json:"duration"`
		Description string `json:"description"`
		Audio       string `json:"audio"`
		Poster      string `json:"poster"`
		AudioExtn   string `json:"audioextn"`
		PosterExtn  string `json:"posterextn"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if payload.Title == "" || payload.Genre == "" || payload.Duration == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Missing required fields: title, genre, duration")
		return
	}

	newSong := models.ArtistSong{
		SongID:      utils.GenerateRandomString(12),
		ArtistID:    artistID,
		Title:       payload.Title,
		Genre:       payload.Genre,
		Duration:    payload.Duration,
		Description: payload.Description,
		AudioURL:    payload.Audio,
		Poster:      payload.Poster,
		Published:   true,
		Plays:       0,
		UploadedAt:  time.Now(),
		AudioExtn:   payload.AudioExtn,
		PosterExtn:  payload.PosterExtn,
	}

	filter := bson.M{"artistid": artistID}
	update := bson.M{
		"$push":        bson.M{"songs": newSong},
		"$setOnInsert": bson.M{"artistid": artistID},
	}
	opts := options.Update().SetUpsert(true)

	if _, err := db.SongsCollection.UpdateOne(ctx, filter, update, opts); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to add song to artist")
		return
	}

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

	var payload struct {
		Title       string `json:"title"`
		Genre       string `json:"genre"`
		Duration    string `json:"duration"`
		Description string `json:"description"`
		Audio       string `json:"audio"`
		Poster      string `json:"poster"`
		AudioExtn   string `json:"audioextn"`
		PosterExtn  string `json:"posterextn"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	updateFields := bson.M{}
	if payload.Title != "" {
		updateFields["songs.$.title"] = payload.Title
	}
	if payload.Genre != "" {
		updateFields["songs.$.genre"] = payload.Genre
	}
	if payload.Duration != "" {
		updateFields["songs.$.duration"] = payload.Duration
	}
	if payload.Description != "" {
		updateFields["songs.$.description"] = payload.Description
	}
	if payload.Audio != "" {
		updateFields["songs.$.audioUrl"] = payload.Audio
	}
	if payload.AudioExtn != "" {
		updateFields["songs.$.audioextn"] = payload.AudioExtn
	}
	if payload.Poster != "" {
		updateFields["songs.$.poster"] = payload.Poster
	}
	if payload.PosterExtn != "" {
		updateFields["songs.$.posterextn"] = payload.PosterExtn
	}

	if len(updateFields) == 0 {
		utils.RespondWithError(w, http.StatusBadRequest, "No fields to update")
		return
	}

	filter := bson.M{"artistid": artistID, "songs.songid": songID}
	update := bson.M{"$set": updateFields}

	res, err := db.SongsCollection.UpdateOne(ctx, filter, update)
	if err != nil || res.MatchedCount == 0 {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update song")
		return
	}

	go mq.Emit(ctx, "song-updated", models.Index{
		EntityType: "song", EntityId: songID, Method: "PUT",
	})
	utils.RespondWithJSON(w, http.StatusOK, bson.M{"message": "Song updated successfully"})
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
