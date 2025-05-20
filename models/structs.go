package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Event struct for MongoDB documents
type Event struct {
	EventID     string    `json:"eventid"`
	Title       string    `json:"title"`
	Location    string    `json:"location"`
	Category    string    `json:"category"`
	Date        time.Time `json:"date"`
	Description string    `json:"description"`
	Image       string    `json:"banner_image"`
}

// Place struct for MongoDB documents
type Place struct {
	PlaceID     string `json:"placeid"`
	Name        string `json:"name"`
	Address     string `json:"address"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Image       string `json:"banner"`
	CreatedAt   string `json:"created_at"`
}

// Index represents the incoming JSON event structure.
type Index struct {
	EntityType string `json:"entity_type"`
	Method     string `json:"method"`
	EntityId   string `json:"entity_id"`
	ItemId     string `json:"item_id"`
	ItemType   string `json:"item_type"`
}

// Result represents a single search result.
type Result struct {
	Placeid     string    `json:"placeid" bson:"placeid"`
	Eventid     string    `json:"eventid" bson:"eventid"`
	Businessid  string    `json:"businessid" bson:"businessid"`
	Userid      string    `json:"userid" bson:"userid"`
	Type        string    `json:"type" bson:"type"`
	Location    string    `json:"location" bson:"location"`
	Address     string    `json:"address" bson:"address"`
	Category    string    `json:"category" bson:"category"`
	Date        time.Time `json:"date" bson:"date"`
	Price       string    `json:"price" bson:"price"`
	Description string    `json:"description" bson:"description"`
	ID          string    `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	Name        string    `json:"name"`
	Title       string    `json:"title"`
	Contact     string    `json:"contact,omitempty"`
	Image       string    `json:"image,omitempty"`
	Link        string    `json:"link,omitempty"`
}

// type Report struct {
// 	ID         primitive.ObjectID `bson:"_id,omitempty"`
// 	ReportedBy string             `bson:"reportedBy"`
// 	TargetID   string             `bson:"targetId"`   // postId, commentId, userId etc.
// 	TargetType string             `bson:"targetType"` // "post", "comment", "user", "media", etc.
// 	Reason     string             `bson:"reason"`
// 	Notes      string             `bson:"notes,omitempty"`
// 	Status     string             `bson:"status"` // "pending", "resolved", "ignored"
// 	CreatedAt  time.Time          `bson:"createdAt"`
// 	UpdatedAt  time.Time          `bson:"updatedAt"`
// }

type Report struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"       json:"id"`
	ReportedBy  string             `bson:"reportedBy"          json:"reportedBy"`
	TargetID    string             `bson:"targetId"            json:"targetId"`
	TargetType  string             `bson:"targetType"          json:"targetType"`
	Reason      string             `bson:"reason"              json:"reason"`
	Notes       string             `bson:"notes,omitempty"     json:"notes,omitempty"`         // user’s optional notes
	Status      string             `bson:"status"              json:"status"`                  // e.g. "pending", "reviewed", "resolved"
	ReviewedBy  string             `bson:"reviewedBy,omitempty" json:"reviewedBy,omitempty"`   // moderator ID
	ReviewNotes string             `bson:"reviewNotes,omitempty" json:"reviewNotes,omitempty"` // moderator’s notes
	CreatedAt   time.Time          `bson:"createdAt"           json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt"           json:"updatedAt"`
}

type Message struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	ChatID    string             `bson:"chatID" json:"chatID"`
	UserID    string             `bson:"userID" json:"userID"`
	Text      string             `bson:"text,omitempty" json:"text,omitempty"`
	FileURL   string             `bson:"fileURL,omitempty" json:"fileURL,omitempty"`
	FileType  string             `bson:"fileType,omitempty" json:"fileType,omitempty"` // "image" or "video"
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	ReplyTo   *ReplyRef          `bson:"replyTo,omitempty" json:"replyTo,omitempty"`
}

type Chat struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Users       []string           `bson:"users" json:"users"`
	LastMessage MessagePreview     `bson:"lastMessage" json:"lastMessage"`
	ReadStatus  map[string]bool    `bson:"readStatus,omitempty" json:"readStatus,omitempty"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type MessagePreview struct {
	Text      string    `bson:"text" json:"text"`
	SenderID  string    `bson:"senderId" json:"senderId"`
	Timestamp time.Time `bson:"timestamp" json:"timestamp"`
}

// ReplyRef represents the client‐side “replyTo” payload.
type ReplyRef struct {
	ID   string `json:"id"`
	User string `json:"user"`
	Text string `json:"text"`
}

type Like struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	UserID     string             `bson:"user_id"`
	EntityType string             `bson:"entity_type"` // e.g. "post"
	EntityID   string             `bson:"entity_id"`   // e.g. post ID
	CreatedAt  time.Time          `bson:"created_at"`
}
