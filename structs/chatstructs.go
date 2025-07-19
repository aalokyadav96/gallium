package structs

import (
	"context"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	// "github.com/redis/go-redis/v9"
)

// Dummy contact definition.
type Contact struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Dummy contacts list.
var DummyContacts = []Contact{
	{ID: "4", Name: "Very"},
	{ID: "5", Name: "Vague"},
	{ID: "6", Name: "Discussion"},
}

func GetUserContacts(userID string) []Contact {
	_ = userID
	return DummyContacts
}

// Simple thread ID generator (for demo purposes).
// var threadIDCounter int = 100

// Data structures for Thread and Message.
// Added ContactID to uniquely identify a thread per contact.
//
//	type Thread struct {
//		ThreadID    string `json:"thread_id" bson:"thread_id"`
//		ContactID string `json:"contact_id" bson:"contact_id"`
//		Name      string `json:"name" bson:"name"`
//		Preview   string `json:"preview" bson:"preview"`
//		Deleted   bool   `json:"deleted" bson:"deleted"`
//	}

type Thread struct {
	ThreadID string    `json:"thread_id" bson:"thread_id"`
	OwnerID  string    `json:"contact_id" bson:"contact_id"`
	Title    string    `json:"title" bson:"title"`
	Preview  string    `json:"preview" bson:"preview"`
	Created  time.Time `json:"created" bson:"created"`
	Deleted  bool      `json:"deleted" bson:"deleted"`
}

type Message struct {
	MessageID   string    `json:"message_id" bson:"message_id,omitempty"` // MongoDB can auto-generate an _id if needed.
	ThreadID    string    `json:"thread_id" bson:"thread_id"`
	SenderID    string    `json:"sender" bson:"sender"`
	Receiver    string    `json:"receiver" bson:"receiver"`
	Content     string    `json:"content,omitempty" bson:"content,omitempty"`
	Caption     string    `json:"caption,omitempty" bson:"caption,omitempty"`
	File        []string  `json:"filename,omitempty" bson:"filename,omitempty"`
	FileType    []string  `json:"filetype,omitempty" bson:"filetype,omitempty"`
	EditHistory []string  `json:"edithistory,omitempty" bson:"edithistory,omitempty"`
	EditedAt    time.Time `json:"editedat" bson:"editedat"`
	CreatedAt   time.Time `json:"createdat" bson:"createdat"`
	Deleted     bool      `json:"deleted" bson:"deleted"`
}

// Global Redis client.
var RedisClient *redis.Client
var Ctx = context.Background()

// WebSocket upgrader configuration.
var Upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Update as necessary to check origins
		return true
	},
}
