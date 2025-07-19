package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Artist struct {
	// ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ArtistID string            `bson:"artistid,omitempty" json:"artistid"`
	Category string            `bson:"category" json:"category"`
	Name     string            `bson:"name" json:"name"`
	Place    string            `bson:"place" json:"place"`
	Country  string            `bson:"country" json:"country"`
	Bio      string            `bson:"bio" json:"bio"`
	DOB      string            `bson:"dob" json:"dob"`
	Photo    string            `bson:"photo" json:"photo"`
	Banner   string            `bson:"banner" json:"banner"`
	Genres   []string          `bson:"genres" json:"genres"`
	Socials  map[string]string `bson:"socials" json:"socials"`
	EventIDs []string          `bson:"events" json:"events"`
	Members  []BandMember      `bson:"members,omitempty" json:"members,omitempty"` // ✅ ADD THIS
}

type BandMember struct {
	Name  string `bson:"name" json:"name"`
	Role  string `bson:"role,omitempty" json:"role,omitempty"`
	DOB   string `bson:"dob,omitempty" json:"dob,omitempty"`
	Image string `bson:"image,omitempty" json:"image,omitempty"` // ✅ fixed bson tag
}

type Cartoon struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Category string             `bson:"category" json:"category"`
	Name     string             `bson:"name" json:"name"`
	Place    string             `bson:"place" json:"place"`
	Country  string             `bson:"country" json:"country"`
	Bio      string             `bson:"bio" json:"bio"`
	DOB      string             `bson:"dob" json:"dob"`
	Photo    string             `bson:"photo" json:"photo"`
	Banner   string             `bson:"banner" json:"banner"`
	Genres   []string           `bson:"genres" json:"genres"`
	Socials  map[string]string  `bson:"socials" json:"socials"`
	// Socials  []string `bson:"socials" json:"socials"`
	EventIDs []string `bson:"events" json:"events"`
}

// ArtistEvent Struct
type ArtistEvent struct {
	EventID   string `bson:"eventid,omitempty" json:"eventid"`
	ArtistID  string `bson:"artistid" json:"artistid"`
	Title     string `bson:"title" json:"title"`
	Date      string `bson:"date" json:"date"`
	Venue     string `bson:"venue" json:"venue"`
	City      string `bson:"city" json:"city"`
	Country   string `bson:"country" json:"country"`
	CreatorID string `bson:"creatorid" json:"creatorid"`
	TicketURL string `bson:"ticket_url,omitempty" json:"ticketUrl,omitempty"`
}

// BehindTheScenes model for the "behind the scenes" content
type BehindTheScenes struct {
	ArtistID    string    `json:"artistid" bson:"artistid"`
	Title       string    `json:"title" bson:"title"`
	Description string    `json:"description" bson:"description"`
	MediaURL    string    `json:"mediaUrl" bson:"mediaUrl"`
	MediaType   string    `json:"mediaType" bson:"mediaType"` // "image" or "video"
	Published   bool      `json:"published" bson:"published"`
	CreatedAt   time.Time `json:"createdAt" bson:"createdAt"`
}
