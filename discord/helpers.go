// discord/helpers.go
package discord

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"naevis/db"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ctx = context.Background()

type Chat struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"    json:"id"`
	Participants []string           `bson:"participants"     json:"participants"`
	CreatedAt    time.Time          `bson:"createdAt"        json:"createdAt"`
	UpdatedAt    time.Time          `bson:"updatedAt"        json:"updatedAt"`
}

type Media struct {
	URL  string `bson:"url"  json:"url"`
	Type string `bson:"type" json:"type"`
}

type Message struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"     json:"id"`
	ChatID     primitive.ObjectID `bson:"chatId"            json:"chatId"`
	Sender     string             `bson:"sender"            json:"sender"`
	SenderName string             `bson:"senderName,omitempty" json:"senderName,omitempty"`
	AvatarURL  string             `bson:"avatarUrl,omitempty" json:"avatarUrl,omitempty"`

	Content string              `bson:"content"           json:"content"`
	Media   *Media              `bson:"media,omitempty"   json:"media,omitempty"`
	ReplyTo *primitive.ObjectID `bson:"replyTo,omitempty" json:"replyTo,omitempty"`

	CreatedAt time.Time  `bson:"createdAt"         json:"createdAt"`
	EditedAt  *time.Time `bson:"editedAt,omitempty" json:"editedAt,omitempty"`
	Deleted   bool       `bson:"deleted"           json:"deleted"`
	ReadBy    []string   `bson:"readBy,omitempty"  json:"readBy,omitempty"`
	Status    string     `bson:"status,omitempty"  json:"status,omitempty"` // e.g. "sent", "read"
}

func persistMediaMessage(chatID primitive.ObjectID, sender, mediaURL, mediaType string) (*Message, error) {
	return persistMessage(chatID, sender, "", mediaURL, mediaType)
}

func persistMessage(chatID primitive.ObjectID, sender, content string, mediaURL string, mediaType string) (*Message, error) {
	if content == "" && mediaURL == "" {
		return nil, errors.New("empty content and media")
	}

	var media *Media
	if mediaURL != "" && mediaType != "" {
		media = &Media{
			URL:  mediaURL,
			Type: mediaType,
		}
	}

	msg := &Message{
		ChatID:    chatID,
		Sender:    sender,
		Content:   content,
		Media:     media,
		CreatedAt: time.Now(),
	}

	res, err := db.MessagesCollection.InsertOne(ctx, msg)
	if err != nil {
		return nil, err
	}
	msg.ID = res.InsertedID.(primitive.ObjectID)

	// Update chat timestamp
	db.ChatsCollection.UpdateOne(ctx,
		bson.M{"_id": chatID},
		bson.M{"$set": bson.M{"updatedAt": time.Now()}},
	)

	return msg, nil
}

func parseInt64(s string) (int64, error) {
	var v int64
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return 0, err
	}
	return v, nil
}

func writeErr(w http.ResponseWriter, msg string, code int) {
	http.Error(w, msg, code)
}
