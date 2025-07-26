package discord

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ctx = context.Background()

type Chat struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"    json:"id"`
	Participants []string           `bson:"participants"     json:"participants"`
	CreatedAt    time.Time          `bson:"createdAt"        json:"createdAt"`
	UpdatedAt    time.Time          `bson:"updatedAt"        json:"updatedAt"`
	EntityType   string             `bson:"entitytype" json:"entitytype"`
	EntityId     string             `bson:"entityid" json:"entityid"`
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
