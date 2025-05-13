package chats

import (
	"context"
	"encoding/json"
	"log"
	"naevis/db"
	"naevis/models"
	"naevis/profile"
	"net/http"
	"sort"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetChat(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	chatIDHex := ps.ByName("chatid")

	tokenString := r.Header.Get("Authorization")
	claims, err := profile.ValidateJWT(tokenString)
	if err != nil {
		log.Printf("JWT validation error: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID := claims.UserID
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	chatID, err := primitive.ObjectIDFromHex(chatIDHex)
	if err != nil {
		http.Error(w, "Invalid chat ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Validate user is a participant of the chat
	chatColl := db.ChatsCollection
	var chat models.Chat
	err = chatColl.FindOne(ctx, bson.M{
		"_id":   chatID,
		"users": bson.M{"$in": []string{userID}},
	}).Decode(&chat)

	if err != nil {
		http.Error(w, "Chat not found", http.StatusNotFound)
		return
	}

	// Fetch messages
	msgColl := db.MessagesCollection
	cursor, err := msgColl.Find(ctx, bson.M{"chatId": chatIDHex})
	if err != nil {
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var messages []models.Message
	if err := cursor.All(ctx, &messages); err != nil {
		http.Error(w, "Decode error", http.StatusInternalServerError)
		return
	}

	if len(messages) == 0 {
		messages = []models.Message{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bson.M{
		"chatId":   chatIDHex,
		"messages": messages,
	})
}

func CreateMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := profile.ValidateJWT(tokenString)
	if err != nil {
		log.Printf("JWT validation error: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	chatID := ps.ByName("chatid")
	var input struct {
		Text string `json:"text"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Text == "" {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		http.Error(w, "Invalid chat ID", http.StatusBadRequest)
		return
	}

	var chat models.Chat
	chatColl := db.ChatsCollection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = chatColl.FindOne(ctx, bson.M{"_id": chatObjID}).Decode(&chat)
	if err != nil {
		http.Error(w, "Chat not found", http.StatusNotFound)
		return
	}

	isParticipant := false
	for _, uid := range chat.Users {
		if uid == claims.UserID {
			isParticipant = true
			break
		}
	}
	if !isParticipant {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	msg := models.Message{
		ChatID:    chatID,
		UserID:    claims.UserID,
		Text:      input.Text,
		CreatedAt: time.Now(),
	}

	msgColl := db.MessagesCollection
	res, err := msgColl.InsertOne(ctx, msg)
	if err != nil {
		http.Error(w, "Insert failed", http.StatusInternalServerError)
		return
	}

	// // Update chat's updatedAt field
	// _, err = chatColl.UpdateOne(ctx, bson.M{"_id": chatObjID}, bson.M{"$set": bson.M{"updatedAt": time.Now()}})
	// if err != nil {
	// 	log.Println("Warning: Failed to update chat timestamp")
	// }
	chatColl.UpdateOne(ctx, bson.M{"_id": chatObjID}, bson.M{
		"$set": bson.M{
			"lastMessage": models.MessagePreview{
				Text:      input.Text,
				SenderID:  claims.UserID,
				Timestamp: time.Now(),
			},
			"readStatus": bson.M{
				claims.UserID:     true,
				"otherUserIDHere": false, // Set to false for other participant
			},
			"updatedAt": time.Now(),
		},
	})

	msg.ID = res.InsertedID.(primitive.ObjectID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}

func UpdateMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := profile.ValidateJWT(tokenString)
	if err != nil {
		log.Printf("JWT validation error: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	msgID, err := primitive.ObjectIDFromHex(ps.ByName("msgid"))
	if err != nil {
		http.Error(w, "Invalid message ID", http.StatusBadRequest)
		return
	}

	coll := db.MessagesCollection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var message models.Message
	err = coll.FindOne(ctx, bson.M{"_id": msgID}).Decode(&message)
	if err != nil {
		http.Error(w, "Message not found", http.StatusNotFound)
		return
	}

	if message.UserID != claims.UserID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var input struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Text == "" {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	_, err = coll.UpdateOne(ctx, bson.M{"_id": msgID}, bson.M{"$set": bson.M{"text": input.Text}})
	if err != nil {
		http.Error(w, "Update failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func DeleteMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := profile.ValidateJWT(tokenString)
	if err != nil {
		log.Printf("JWT validation error: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	msgID, err := primitive.ObjectIDFromHex(ps.ByName("msgid"))
	if err != nil {
		http.Error(w, "Invalid message ID", http.StatusBadRequest)
		return
	}

	msgColl := db.MessagesCollection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var message models.Message
	err = msgColl.FindOne(ctx, bson.M{"_id": msgID}).Decode(&message)
	if err != nil {
		http.Error(w, "Message not found", http.StatusNotFound)
		return
	}

	if message.UserID != claims.UserID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	_, err = msgColl.DeleteOne(ctx, bson.M{"_id": msgID})
	if err != nil {
		http.Error(w, "Delete failed", http.StatusInternalServerError)
		return
	}

	// Update chat's updatedAt timestamp
	chatObjID, err := primitive.ObjectIDFromHex(message.ChatID)
	if err == nil {
		chatColl := db.ChatsCollection
		_, _ = chatColl.UpdateOne(ctx, bson.M{"_id": chatObjID}, bson.M{"$set": bson.M{"updatedAt": time.Now()}})
		// If update fails, we ignore it silently.
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// func InitChat(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	var body struct {
// 		UserA string `json:"userA"`
// 		UserB string `json:"userB"`
// 	}
// 	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
// 		http.Error(w, "Invalid input", http.StatusBadRequest)
// 		return
// 	}

// 	users := []string{body.UserA, body.UserB}
// 	sort.Strings(users)

// 	chatCol := db.ChatsCollection
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	var existingChat models.Chat
// 	err := chatCol.FindOne(ctx, bson.M{"users": users}).Decode(&existingChat)
// 	if err != nil && err != mongo.ErrNoDocuments {
// 		http.Error(w, "DB error", http.StatusInternalServerError)
// 		return
// 	}

// 	if err == nil {
// 		json.NewEncoder(w).Encode(bson.M{"chatId": existingChat.ID.Hex()})
// 		return
// 	}

// 	newChat := models.Chat{Users: users}
// 	res, err := chatCol.InsertOne(ctx, newChat)
// 	if err != nil {
// 		http.Error(w, "Failed to create chat", http.StatusInternalServerError)
// 		return
// 	}

//		chatId := res.InsertedID.(primitive.ObjectID)
//		w.Header().Set("Content-Type", "application/json")
//		json.NewEncoder(w).Encode(bson.M{"chatId": chatId.Hex()})
//	}

func InitChat(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var body struct {
		UserA string `json:"userA"`
		UserB string `json:"userB"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	users := []string{body.UserA, body.UserB}
	sort.Strings(users)

	chatCol := db.ChatsCollection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var existingChat models.Chat
	err := chatCol.FindOne(ctx, bson.M{"users": users}).Decode(&existingChat)
	if err == nil {
		// Chat already exists, return existing chat ID
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(bson.M{"chatId": existingChat.ID.Hex()})
		return
	} else if err != mongo.ErrNoDocuments {
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}

	// Create new chat
	newChat := models.Chat{Users: users}
	res, err := chatCol.InsertOne(ctx, newChat)
	if err != nil {
		http.Error(w, "Failed to create chat", http.StatusInternalServerError)
		return
	}

	chatId := res.InsertedID.(primitive.ObjectID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bson.M{"chatId": chatId.Hex()})
}

// func GetUserChats(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	tokenString := r.Header.Get("Authorization")
// 	claims, err := profile.ValidateJWT(tokenString)
// 	if err != nil {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}

// 	userID := claims.UserID
// 	if userID == "" {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}

// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	coll := db.ChatsCollection
// 	cursor, err := coll.Find(ctx, bson.M{
// 		"users": bson.M{"$in": []string{userID}},
// 	}, options.Find().
// 		SetSort(bson.M{"updatedAt": -1}).
// 		SetLimit(15),
// 	)

// 	if err != nil {
// 		http.Error(w, "Database error", http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(ctx)

// 	var chats []models.Chat
// 	if err := cursor.All(ctx, &chats); err != nil {
// 		http.Error(w, "Decode error", http.StatusInternalServerError)
// 		return
// 	}

//		w.Header().Set("Content-Type", "application/json")
//		json.NewEncoder(w).Encode(chats)
//	}
func GetUserChats(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := profile.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID := claims.UserID
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	coll := db.ChatsCollection
	cursor, err := coll.Find(ctx, bson.M{
		"users": bson.M{"$in": []string{userID}},
	}, options.Find().
		SetSort(bson.M{"updatedAt": -1}).
		SetLimit(15),
	)

	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var chats []models.Chat
	if err := cursor.All(ctx, &chats); err != nil {
		http.Error(w, "Decode error", http.StatusInternalServerError)
		return
	}

	// Optional: Filter out sensitive fields or normalize if needed

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chats)
}
