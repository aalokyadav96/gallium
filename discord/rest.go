// discord/rest.go
package discord

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"naevis/db"
	"naevis/utils"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func UploadAttachment(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := utils.GetUserIDFromRequest(r)
	chatIDHex := ps.ByName("chatId")
	chatID, err := primitive.ObjectIDFromHex(chatIDHex)
	if err != nil {
		writeErr(w, "invalid chatId", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, "failed to read file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") && !strings.HasPrefix(contentType, "application/") {
		writeErr(w, "unsupported file type", http.StatusBadRequest)
		return
	}

	uploadDir := "static/uploads/farmchat"
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		writeErr(w, "cannot create upload dir", http.StatusInternalServerError)
		return
	}

	ext := filepath.Ext(header.Filename)
	fname := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	destPath := filepath.Join(uploadDir, fname)

	out, err := os.Create(destPath)
	if err != nil {
		writeErr(w, "cannot save file", http.StatusInternalServerError)
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		writeErr(w, "error writing file", http.StatusInternalServerError)
		return
	}

	// url := fmt.Sprintf("/static/uploads/farmchat/%s", fname)

	// Save as media message (empty text, media attached)
	// msg, err := persistMessage(chatID, user, "", fname, contentType)
	msg, err := persistMediaMessage(chatID, user, fname, contentType)
	if err != nil {
		writeErr(w, "failed to persist message", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}

// func UploadAttachment(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	user := utils.GetUserIDFromRequest(r)
// 	chatIDHex := ps.ByName("chatId")
// 	chatID, err := primitive.ObjectIDFromHex(chatIDHex)
// 	if err != nil {
// 		writeErr(w, "invalid chatId", http.StatusBadRequest)
// 		return
// 	}

// 	file, header, err := r.FormFile("file")
// 	if err != nil {
// 		writeErr(w, "failed to read file", http.StatusBadRequest)
// 		return
// 	}
// 	defer file.Close()

// 	contentType := header.Header.Get("Content-Type")
// 	if !strings.HasPrefix(contentType, "image/") && !strings.HasPrefix(contentType, "application/") {
// 		writeErr(w, "unsupported file type", http.StatusBadRequest)
// 		return
// 	}

// 	uploadDir := "static/uploads/farmchat"
// 	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
// 		writeErr(w, "cannot create upload dir", http.StatusInternalServerError)
// 		return
// 	}

// 	ext := filepath.Ext(header.Filename)
// 	fname := fmt.Sprintf("%s%s", uuid.New().String(), ext)
// 	destPath := filepath.Join(uploadDir, fname)

// 	out, err := os.Create(destPath)
// 	if err != nil {
// 		writeErr(w, "cannot save file", http.StatusInternalServerError)
// 		return
// 	}
// 	defer out.Close()

// 	if _, err := io.Copy(out, file); err != nil {
// 		writeErr(w, "error writing file", http.StatusInternalServerError)
// 		return
// 	}

// 	url := fmt.Sprintf("/static/uploads/farmchat/%s", fname)
// 	msg, err := persistMessage(chatID, user, url)
// 	if err != nil {
// 		writeErr(w, "failed to persist message", http.StatusInternalServerError)
// 		return
// 	}

// 	resp := map[string]interface{}{
// 		"id":      msg.ID.Hex(),
// 		"url":     url,
// 		"type":    contentType,
// 		"created": msg.CreatedAt,
// 		"sender":  msg.Sender,
// 	}
// 	json.NewEncoder(w).Encode(resp)
// }

// (continue with other REST functions like GetUserChats, GetChatMessages, etc.)

func GetUserChats(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	user := utils.GetUserIDFromRequest(r)
	cursor, err := db.ChatsCollection.Find(ctx, bson.M{"participants": user})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer cursor.Close(ctx)

	var chats []Chat
	if err := cursor.All(ctx, &chats); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// ensure non-nil slice
	if chats == nil {
		chats = make([]Chat, 0)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chats)
}

func StartNewChat(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	user := utils.GetUserIDFromRequest(r)
	var body struct{ Participants []string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", 400)
		return
	}
	// ensure the requesting user is in the participants list
	found := false
	for _, p := range body.Participants {
		if p == user {
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "must include yourself", 400)
		return
	}
	// check existing chat
	filter := bson.M{"participants": bson.M{"$all": body.Participants}}
	var existing Chat
	err := db.ChatsCollection.FindOne(ctx, filter).Decode(&existing)
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(existing)
		return
	}
	if err != mongo.ErrNoDocuments {
		http.Error(w, err.Error(), 500)
		return
	}
	// create new chat
	now := time.Now()
	chat := Chat{
		Participants: body.Participants,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	res, err := db.ChatsCollection.InsertOne(ctx, chat)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	chat.ID = res.InsertedID.(primitive.ObjectID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chat)
}

func GetChatByID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	chatID, err := primitive.ObjectIDFromHex(ps.ByName("chatId"))
	if err != nil {
		http.Error(w, "invalid chatId", 400)
		return
	}
	var chat Chat
	if err := db.ChatsCollection.FindOne(ctx, bson.M{"_id": chatID}).Decode(&chat); err != nil {
		http.Error(w, "not found", 404)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chat)
}

func GetChatMessages(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	chatID, err := primitive.ObjectIDFromHex(ps.ByName("chatId"))
	if err != nil {
		http.Error(w, "invalid chatId", 400)
		return
	}
	// pagination
	limit := int64(50)
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := parseInt64(l); err == nil {
			limit = v
		}
	}
	skip := int64(0)
	if s := r.URL.Query().Get("skip"); s != "" {
		if v, err := parseInt64(s); err == nil {
			skip = v
		}
	}

	opts := options.Find().SetSort(bson.M{"createdAt": 1}).SetLimit(limit).SetSkip(skip)
	cursor, err := db.MessagesCollection.Find(ctx, bson.M{"chatId": chatID}, opts)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer cursor.Close(ctx)

	var msgs []Message
	if err := cursor.All(ctx, &msgs); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// ensure non-nil slice
	if msgs == nil {
		msgs = make([]Message, 0)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msgs)
}

func SendMessageREST(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	chatID, err := primitive.ObjectIDFromHex(ps.ByName("chatId"))
	if err != nil {
		http.Error(w, "invalid chatId", http.StatusBadRequest)
		return
	}

	var body struct{ Content string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	sender := utils.GetUserIDFromRequest(r)
	msg, err := persistMessage(chatID, sender, body.Content, "", "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}

func EditMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	msgID, err := primitive.ObjectIDFromHex(ps.ByName("messageId"))
	if err != nil {
		http.Error(w, "invalid messageId", 400)
		return
	}
	var body struct{ Content string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", 400)
		return
	}
	now := time.Now()
	res, err := db.MessagesCollection.UpdateOne(ctx,
		bson.M{"_id": msgID},
		bson.M{"$set": bson.M{"content": body.Content, "editedAt": now}},
	)
	if err != nil || res.MatchedCount == 0 {
		http.Error(w, "not found or no permission", 404)
		return
	}
	w.WriteHeader(204)
}

func DeleteMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	msgID, err := primitive.ObjectIDFromHex(ps.ByName("messageId"))
	if err != nil {
		http.Error(w, "invalid messageId", 400)
		return
	}
	res, err := db.MessagesCollection.UpdateOne(ctx,
		bson.M{"_id": msgID},
		bson.M{"$set": bson.M{"deleted": true}},
	)
	if err != nil || res.MatchedCount == 0 {
		http.Error(w, "not found or no permission", 404)
		return
	}
	w.WriteHeader(204)
}

func SearchMessages(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	chatID, err := primitive.ObjectIDFromHex(ps.ByName("chatId"))
	if err != nil {
		http.Error(w, "invalid chatId", http.StatusBadRequest)
		return
	}
	term := r.URL.Query().Get("term")

	// pagination
	limit := int64(50)
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := parseInt64(l); err == nil {
			limit = v
		}
	}
	skip := int64(0)
	if s := r.URL.Query().Get("skip"); s != "" {
		if v, err := parseInt64(s); err == nil {
			skip = v
		}
	}

	filter := bson.M{"chatId": chatID}
	if term != "" {
		filter["content"] = bson.M{"$regex": primitive.Regex{Pattern: term, Options: "i"}}
	}

	opts := options.Find().
		SetSort(bson.M{"createdAt": 1}).
		SetLimit(limit).
		SetSkip(skip)

	cursor, err := db.MessagesCollection.Find(ctx, filter, opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var msgs []Message
	if err := cursor.All(ctx, &msgs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if msgs == nil {
		msgs = make([]Message, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msgs)
}

// —— Misc ————————————————————————————————————————

func GetUnreadCount(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	user := utils.GetUserIDFromRequest(r)

	// First, find all chats the user participates in
	cursor, err := db.ChatsCollection.Find(ctx, bson.M{"participants": user})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	type Unread struct {
		ChatID string `json:"chatId"`
		Count  int64  `json:"count"`
	}
	var result []Unread

	for cursor.Next(ctx) {
		var chat Chat
		if err := cursor.Decode(&chat); err != nil {
			continue
		}
		count, err := db.MessagesCollection.CountDocuments(ctx, bson.M{
			"chatId": chat.ID,
			"readBy": bson.M{"$ne": user},
		})
		if err != nil {
			continue
		}
		result = append(result, Unread{
			ChatID: chat.ID.Hex(),
			Count:  count,
		})
	}
	if result == nil {
		result = make([]Unread, 0)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func MarkAsRead(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	msgID, err := primitive.ObjectIDFromHex(ps.ByName("messageId"))
	if err != nil {
		http.Error(w, "invalid messageId", http.StatusBadRequest)
		return
	}
	user := utils.GetUserIDFromRequest(r)

	res, err := db.MessagesCollection.UpdateOne(ctx,
		bson.M{"_id": msgID},
		bson.M{"$addToSet": bson.M{"readBy": user}},
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if res.MatchedCount == 0 {
		http.Error(w, "message not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
