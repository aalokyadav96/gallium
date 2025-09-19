package newchat

import (
	"context"
	"encoding/json"
	"log"
	"naevis/db"
	"naevis/filemgr"
	"naevis/middleware"
	"naevis/models"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type SearchResult struct {
	Matches []ChatMessage `json:"matches"`
}

type ChatMessage struct {
	ID        string `json:"id"`
	Sender    string `json:"sender"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
}

func GetChat(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	chatIDHex := ps.ByName("chatid")

	tokenString := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(tokenString)
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
	var chat models.Chat
	err = db.ChatsCollection.FindOne(ctx, bson.M{
		"_id":   chatID,
		"users": bson.M{"$in": []string{userID}},
	}).Decode(&chat)

	if err != nil {
		http.Error(w, "Chat not found", http.StatusNotFound)
		return
	}

	// Fetch messages (sorted by CreatedAt ascending)
	cursor, err := db.MessagesCollection.Find(ctx, bson.M{
		"chatid": chatIDHex,
	}, options.Find().SetSort(bson.M{"createdAt": 1}))
	if err != nil {
		http.Error(w, "DB error", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var messages []models.Message
	if err := cursor.All(ctx, &messages); err != nil {
		http.Error(w, "Failed to decode messages", http.StatusInternalServerError)
		return
	}

	// Return chat ID and messages (text + file info will be auto included)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bson.M{
		"chatid":   chatIDHex,
		"messages": messages,
	})
}
func CreateMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	chatID := ps.ByName("chatid")
	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		http.Error(w, "Invalid chat ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var chat models.Chat
	if err := db.ChatsCollection.FindOne(ctx, bson.M{"_id": chatObjID}).Decode(&chat); err != nil {
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

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	text := r.FormValue("text")

	var replyRef *models.ReplyRef
	if v := r.FormValue("replyTo"); v != "" {
		var rr models.ReplyRef
		if err := json.Unmarshal([]byte(v), &rr); err != nil {
			http.Error(w, "Invalid replyTo payload", http.StatusBadRequest)
			return
		}
		replyRef = &rr
	}

	// ⬇️ Optional file handling
	var fileURL, fileType string
	if r.MultipartForm != nil && r.MultipartForm.File != nil {
		files := r.MultipartForm.File["file"]
		if len(files) > 0 {
			header := files[0]
			contentType := header.Header.Get("Content-Type")

			switch {
			case strings.HasPrefix(contentType, "image/"):
				fileType = "image"
			case strings.HasPrefix(contentType, "video/"):
				fileType = "video"
			case strings.HasPrefix(contentType, "application/"):
				fileType = "document"
			default:
				http.Error(w, "Unsupported file type", http.StatusBadRequest)
				return
			}

			// ✅ Save file using filemgr with PicPhoto (instead of PicFile)
			savedName, err := filemgr.SaveFormFile(r.MultipartForm, "file", filemgr.EntityChat, filemgr.PicPhoto, false)
			if err != nil {
				http.Error(w, "Failed to save file", http.StatusInternalServerError)
				return
			}
			fileURL = savedName
		}
	}

	if text == "" && fileURL == "" && replyRef == nil {
		http.Error(w, "No content provided", http.StatusBadRequest)
		return
	}

	now := time.Now()
	msg := models.Message{
		ChatID:    chatID,
		UserID:    claims.UserID,
		Text:      text,
		FileURL:   fileURL,
		FileType:  fileType,
		CreatedAt: now,
	}
	if replyRef != nil {
		msg.ReplyTo = &models.ReplyRef{
			ID:   replyRef.ID,
			User: replyRef.User,
			Text: replyRef.Text,
		}
	}

	res, err := db.MessagesCollection.InsertOne(ctx, msg)
	if err != nil {
		http.Error(w, "Insert failed", http.StatusInternalServerError)
		return
	}

	db.ChatsCollection.UpdateOne(
		ctx,
		bson.M{"_id": chatObjID},
		bson.M{
			"$set": bson.M{
				"lastMessage": models.MessagePreview{
					Text:      text,
					SenderID:  claims.UserID,
					Timestamp: now,
				},
				"updatedAt": now,
			},
		},
	)

	msg.ID = res.InsertedID.(primitive.ObjectID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}

func UpdateMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(tokenString)
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

func DeletesMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(tokenString)
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
		json.NewEncoder(w).Encode(bson.M{"chatid": existingChat.ID.Hex()})
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
	json.NewEncoder(w).Encode(bson.M{"chatid": chatId.Hex()})
}

func GetUserChats(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tokenString := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(tokenString)
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

	if len(chats) == 0 {
		chats = []models.Chat{}
	}

	// Optional: Filter out sensitive fields or normalize if needed

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chats)
}
