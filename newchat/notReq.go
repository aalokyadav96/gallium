package newchat

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"naevis/db"
	"naevis/middleware"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func UpdatexMessage(userID string, id string, newContent string) error {
	filter := bson.M{"message_id": id, "sender_id": userID}
	update := bson.M{"$set": bson.M{"content": newContent}}
	res, err := db.MessagesCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return errors.New("message not found or unauthorized")
	}
	return nil
}

func DeletexMessage(userID string, id string) error {
	filter := bson.M{"message_id": id, "sender_id": userID}
	res, err := db.MessagesCollection.DeleteOne(context.Background(), filter)
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return errors.New("message not found or unauthorized")
	}
	return nil
}

type editPayload struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

func extractToken(r *http.Request) (string, error) {
	if auth := r.Header.Get("Authorization"); auth != "" {
		return auth, nil
	}
	if q := r.URL.Query().Get("token"); q != "" {
		return "Bearer " + q, nil
	}
	return "", errors.New("missing token")
}

// find room of a message by ID
func findMessageRoom(id string) (string, error) {
	var msg Message
	err := db.MessagesCollection.FindOne(context.Background(), bson.M{"message_id": id}).Decode(&msg)
	if err != nil {
		return "", err
	}
	return msg.Room, nil
}

func EditMessageHandler(hub *Hub) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		var payload editPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		token, err := extractToken(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		claims, err := middleware.ValidateJWT(token)
		if err != nil {
			log.Println("jwt:", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if err := UpdatexMessage(claims.UserID, payload.ID, payload.Content); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, "not found", http.StatusNotFound)
			} else {
				http.Error(w, "forbidden", http.StatusForbidden)
			}
			return
		}

		room, err := findMessageRoom(payload.ID)
		if err != nil {
			http.Error(w, "room not found", http.StatusNotFound)
			return
		}

		// Broadcast edit to that room only
		out := outboundPayload{
			Action:    "edit",
			ID:        payload.ID,
			Content:   payload.Content,
			Timestamp: time.Now().Unix(),
		}
		if data, err := json.Marshal(out); err == nil {
			hub.broadcast <- broadcastMsg{Room: room, Data: data}
		}

		w.WriteHeader(http.StatusOK)
	}
}

func DeleteMessageHandler(hub *Hub) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		var payload struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		token, err := extractToken(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		claims, err := middleware.ValidateJWT(token)
		if err != nil {
			log.Println("jwt:", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		room, err := findMessageRoom(payload.ID)
		if err != nil {
			http.Error(w, "room not found", http.StatusNotFound)
			return
		}

		if err := DeletexMessage(claims.UserID, payload.ID); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, "not found", http.StatusNotFound)
			} else {
				http.Error(w, "forbidden", http.StatusForbidden)
			}
			return
		}

		// Broadcast delete to that room only
		out := outboundPayload{
			Action: "delete",
			ID:     payload.ID,
		}
		if data, err := json.Marshal(out); err == nil {
			hub.broadcast <- broadcastMsg{Room: room, Data: data}
		}

		w.WriteHeader(http.StatusOK)
	}
}
