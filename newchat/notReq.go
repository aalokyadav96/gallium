package newchat

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"naevis/db"
	"naevis/middleware"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// func InsertMessage(msg Message) error {
// 	_, err := db.MessagesCollection.InsertOne(context.Background(), msg)
// 	return err
// }

func UpdateMessage(userID string, id string, newContent string) error {
	filter := bson.M{"messageid": id, "senderId": userID}
	update := bson.M{"$set": bson.M{"content": newContent}}
	res, err := db.MessagesCollection.UpdateOne(context.Background(), filter, update)
	if err != nil || res.MatchedCount == 0 {
		return errors.New("update failed or unauthorized")
	}
	return nil
}

func DeleteMessage(userID string, id string) error {
	filter := bson.M{"messageid": id, "senderId": userID}
	res, err := db.MessagesCollection.DeleteOne(context.Background(), filter)
	if err != nil || res.DeletedCount == 0 {
		return errors.New("delete failed or unauthorized")
	}
	return nil
}

type editPayload struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

func EditMessageHandler(hub *Hub) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		var payload editPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		// userID := r.Context().Value(globals.UserIDKey).(string)
		// msgID, err := primitive.ObjectIDFromHex(payload.ID)
		// if err != nil {
		// 	http.Error(w, "invalid id", http.StatusBadRequest)
		// 	return
		// }

		var token = "Bearer " + r.URL.Query().Get("token")
		claims, err := middleware.ValidateJWT(token)
		if err != nil {
			log.Println(err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userID := claims.UserID
		msgID := payload.ID
		if err := UpdateMessage(userID, msgID, payload.Content); err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
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
		// userID := r.Context().Value(globals.UserIDKey).(string)
		// msgID, err := primitive.ObjectIDFromHex(payload.ID)
		// if err != nil {
		// 	http.Error(w, "invalid id", http.StatusBadRequest)
		// 	return
		// }

		var token = "Bearer " + r.URL.Query().Get("token")
		claims, err := middleware.ValidateJWT(token)
		if err != nil {
			log.Println(err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userID := claims.UserID
		msgID := payload.ID
		if err := DeleteMessage(userID, msgID); err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
