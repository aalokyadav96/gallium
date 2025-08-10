package artists

import (
	"context"
	"naevis/db"
	"naevis/media"
	"naevis/merch"
	"naevis/models"
	"naevis/utils"
	"net/http"
	_ "net/http/pprof"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func GetArtistsAlbums(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	albums := []models.ArtistAlbum{
		{Title: "Nightfall", ReleaseDate: "2023-10-01", Description: "A journey through dusk.", Published: true},
		{Title: "Unreleased Gems", ReleaseDate: "2025-01-01", Description: "Upcoming exclusives.", Published: false},
	}
	utils.RespondWithJSON(w, http.StatusOK, albums)
}

func GetArtistsPosts(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	pr := httprouter.Param{Key: "entityType", Value: "artist"}
	px := httprouter.Param{Key: "eventid", Value: ps.ByName("id")}
	ps = append(ps, pr, px)
	media.GetMedias(w, r, ps)
}

func GetArtistsMerch(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	pr := httprouter.Param{Key: "entityType", Value: "artist"}
	px := httprouter.Param{Key: "eventid", Value: ps.ByName("id")}
	ps = append(ps, pr, px)
	merch.GetMerchs(w, r, ps)
}

func GetArtistsevents(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	events := []models.ArtistEvent{
		{Title: "Summer Fest 2025", Date: "2025-06-15", Venue: "Sunset Arena", City: "Los Angeles", Country: "USA", TicketURL: "http://localhost:5173/event/4s89t5jt6754djt"},
		{Title: "Berlin Beats", Date: "2025-07-20", Venue: "Techno Temple", City: "Berlin", Country: "Germany"},
	}
	utils.RespondWithJSON(w, http.StatusOK, events)
}

func GetBTS(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Assuming artistID is passed as a URL parameter like /artists/:artistID/behindthescenes
	artistID := ps.ByName("artistID")

	// Fetch the behind-the-scenes content from the database
	var content []models.BehindTheScenes // This is your data model for behind-the-scenes content
	cursor, err := db.BehindTheScenesCollection.Find(context.TODO(), bson.M{"artistid": artistID})
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Error fetching behind-the-scenes content")
		return
	}
	defer cursor.Close(context.TODO())

	// Iterate over the cursor and append the content to the slice
	for cursor.Next(context.TODO()) {
		var btsItem models.BehindTheScenes
		if err := cursor.Decode(&btsItem); err != nil {
			utils.RespondWithError(w, http.StatusInternalServerError, "Error decoding behind-the-scenes content")
			return
		}
		content = append(content, btsItem)
	}

	if err := cursor.Err(); err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Error fetching behind-the-scenes content")
		return
	}

	// Respond with the fetched behind-the-scenes content
	utils.RespondWithJSON(w, http.StatusOK, content)
}
