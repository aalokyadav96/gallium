package artists

import (
	"context"
	"fmt"
	"naevis/db"
	"naevis/models"
	"naevis/utils"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// Artist Events
func GetArtistEvents(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	filter := bson.M{"artistid": ps.ByName("id")}
	artistevents, err := utils.FindAndDecode[models.ArtistEvent](ctx, db.ArtistEventsCollection, filter)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch artist events")
		return
	}

	if len(artistevents) == 0 {
		artistevents = []models.ArtistEvent{}
	}

	utils.RespondWithJSON(w, http.StatusOK, artistevents)
}

// All Artists
func GetAllArtists(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var artists []models.Artist
	cursor, err := db.ArtistsCollection.Find(ctx, bson.M{})
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Error fetching artists")
		return
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &artists); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Error decoding artists")
		return
	}

	if len(artists) == 0 {
		artists = []models.Artist{}
	}

	utils.RespondWithJSON(w, http.StatusOK, artists)
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
