package chats

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"naevis/db"
	"naevis/middleware"
	"naevis/models"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"sync"

	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // Allow all origins, restrict in production
}

var clients = make(map[string]*websocket.Conn) // key: userID
var clientsMu sync.Mutex

func ChatWebSocket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	token := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(token)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := claims.UserID

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	clientsMu.Lock()
	clients[userID] = conn
	clientsMu.Unlock()

	for {
		var msg models.Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println("WebSocket read error:", err)
			break
		}

		msg.UserID = userID
		msg.CreatedAt = time.Now()

		// Insert to DB
		_, err = db.MessagesCollection.InsertOne(context.TODO(), msg)
		if err != nil {
			log.Println("Mongo insert error:", err)
			continue
		}

		// Broadcast to all chat participants
		for _, uid := range getChatUserIDs(msg.ChatID) {
			clientsMu.Lock()
			if c, ok := clients[uid]; ok && c != conn {
				_ = c.WriteJSON(msg)
			}
			clientsMu.Unlock()
		}
	}
}

func getChatUserIDs(chatID string) []string {
	var chat models.Chat
	objID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		return nil
	}
	err = db.ChatsCollection.FindOne(context.TODO(), bson.M{"_id": objID}).Decode(&chat)
	if err != nil {
		return nil
	}
	return chat.Users
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
		"chatID": chatIDHex,
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
		"chatId":   chatIDHex,
		"messages": messages,
	})
}

func saveUploadedFile(r *http.Request) (fileURL string, fileType string, err error) {
	file, handler, err := r.FormFile("file")
	if err != nil {
		if err == http.ErrMissingFile {
			return "", "", nil
		}
		return "", "", fmt.Errorf("failed to read file: %w", err)
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(handler.Filename))
	allowed := map[string]string{
		".jpg": "image", ".jpeg": "image", ".png": "image", ".gif": "image",
		".mp4": "video", ".webm": "video", ".mov": "video",
	}

	ftype, ok := allowed[ext]
	if !ok {
		return "", "", fmt.Errorf("unsupported file type: %s", ext)
	}

	filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), handler.Filename)
	savePath := filepath.Join("static", "uploads", filename)

	out, err := os.Create(savePath)
	if err != nil {
		return "", "", fmt.Errorf("unable to create file on disk: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		return "", "", fmt.Errorf("failed to write file: %w", err)
	}

	publicURL := "/uploads/" + filename
	return publicURL, ftype, nil
}

func CreateMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// 1) Validate JWT
	tokenString := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2) Parse chatID and convert to ObjectID
	chatID := ps.ByName("chatid")
	chatObjID, err := primitive.ObjectIDFromHex(chatID)
	if err != nil {
		http.Error(w, "Invalid chat ID", http.StatusBadRequest)
		return
	}

	// 3) Verify participation
	var chat models.Chat
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
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

	// 4) Parse multipart form (max 10 MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// 5) Extract “text”
	text := r.FormValue("text")

	// 6) Extract and unmarshal “replyTo” if present
	var replyRef *models.ReplyRef
	if v := r.FormValue("replyTo"); v != "" {
		var rr models.ReplyRef
		if err := json.Unmarshal([]byte(v), &rr); err != nil {
			http.Error(w, "Invalid replyTo payload", http.StatusBadRequest)
			return
		}
		replyRef = &rr
	}

	// 7) Handle file upload separately
	fileURL, fileType, err := saveUploadedFile(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 8) Ensure there is either text, file, or a reply reference
	if text == "" && fileURL == "" && replyRef == nil {
		http.Error(w, "No content provided", http.StatusBadRequest)
		return
	}

	// 9) Build the Message object (including optional ReplyTo)
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
			ID:   replyRef.ID,   // raw hex string from client
			User: replyRef.User, // username or userID from client
			Text: replyRef.Text,
		}
	}

	// 10) Insert into Messages collection
	res, err := db.MessagesCollection.InsertOne(ctx, msg)
	if err != nil {
		http.Error(w, "Insert failed", http.StatusInternalServerError)
		return
	}

	// 11) Update chat’s lastMessage and updatedAt
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

	// 12) Return the created message as JSON
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

func DeleteMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

// func SearchChat(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	tokenString := r.Header.Get("Authorization")
// 	claims, err := middleware.ValidateJWT(tokenString)
// 	if err != nil {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}

// 	userID := claims.UserID
// 	if userID == "" {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}

// 	res := models.Message{
// 		ChatID: ps.ByName("chatid"),
// 		UserID: userID,
// 		Text:   r.URL.Query().Get("q"),
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(res)
// }

type ChatMessage struct {
	ID        string `json:"id"`
	Sender    string `json:"sender"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
}

type SearchResult struct {
	Matches []ChatMessage `json:"matches"`
}

func SearchChat(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

	dummyMessages := []ChatMessage{
		{
			ID:        "msg1",
			Sender:    "user123",
			Text:      "Hello, how are you?",
			Timestamp: time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
		},
		{
			ID:        "msg2",
			Sender:    "user456",
			Text:      "I'm fine, thanks!",
			Timestamp: time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
		},
	}

	result := SearchResult{
		Matches: dummyMessages,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
