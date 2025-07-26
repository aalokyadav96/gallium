// discord/helpers.go
package discord

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"naevis/db"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

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
