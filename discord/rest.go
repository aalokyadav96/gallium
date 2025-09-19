// discord/rest.go
package discord

import (
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"naevis/db"
	"naevis/filemgr"
	"naevis/utils"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// UploadAttachment handles media/file upload into a chat
func UploadAttachment(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	user := utils.GetUserIDFromRequest(r)

	chatIDHex := ps.ByName("chatid")
	chatID, err := primitive.ObjectIDFromHex(chatIDHex)
	if err != nil {
		writeErr(w, "invalid chatid", http.StatusBadRequest)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeErr(w, "invalid form", http.StatusBadRequest)
		return
	}

	var header *multipart.FileHeader
	if r.MultipartForm != nil && r.MultipartForm.File != nil {
		files := r.MultipartForm.File["file"]
		if len(files) > 0 {
			header = files[0]
		}
	}
	if header == nil {
		writeErr(w, "no file provided", http.StatusBadRequest)
		return
	}

	contentType := header.Header.Get("Content-Type")

	// Map content type → PictureType
	var picType filemgr.PictureType
	switch {
	case strings.HasPrefix(contentType, "image/"):
		picType = filemgr.PicPhoto
	case strings.HasPrefix(contentType, "video/"):
		picType = filemgr.PicVideo
	case strings.HasPrefix(contentType, "application/"):
		picType = filemgr.PicFile
	default:
		writeErr(w, "unsupported file type", http.StatusBadRequest)
		return
	}

	// Save file via filemgr
	savedName, err := filemgr.SaveFormFile(r.MultipartForm, "file", filemgr.EntityChat, picType, false)
	if err != nil {
		writeErr(w, "cannot save file", http.StatusInternalServerError)
		return
	}

	// Persist media message
	msg, err := persistMediaMessage(ctx, chatID, user, savedName, contentType)
	if err != nil {
		writeErr(w, "failed to persist message", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}

func GetUserChats(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
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
	ctx := r.Context()
	user := utils.GetUserIDFromRequest(r)

	var body struct {
		Participants []string `json:"participants"`
		EntityType   string   `json:"entityType"`
		EntityId     string   `json:"entityId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", 400)
		return
	}

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

	filter := bson.M{
		"participants": bson.M{"$all": body.Participants},
		"entityType":   body.EntityType,
		"entityId":     body.EntityId,
	}

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

	now := time.Now()
	chat := Chat{
		Participants: body.Participants,
		CreatedAt:    now,
		UpdatedAt:    now,
		EntityType:   body.EntityType,
		EntityId:     body.EntityId,
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
	ctx := r.Context()
	chatID, err := primitive.ObjectIDFromHex(ps.ByName("chatid"))
	if err != nil {
		http.Error(w, "invalid chatid", 400)
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
	ctx := r.Context()
	chatID, err := primitive.ObjectIDFromHex(ps.ByName("chatid"))
	if err != nil {
		http.Error(w, "invalid chatid", 400)
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
	cursor, err := db.MessagesCollection.Find(ctx, bson.M{"chatid": chatID}, opts)
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

// SendMessageREST (updated)

// SendMessageREST handles plain text messages via HTTP
func SendMessageREST(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	chatID, err := primitive.ObjectIDFromHex(ps.ByName("chatid"))
	if err != nil {
		writeErr(w, "invalid chatid", http.StatusBadRequest)
		return
	}

	var body struct {
		Content  string `json:"content"`
		ClientID string `json:"clientId,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, "invalid body", http.StatusBadRequest)
		return
	}

	sender := utils.GetUserIDFromRequest(r)
	msg, err := persistMessage(ctx, chatID, sender, body.Content, "", "")
	if err != nil {
		writeErr(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build response payload (echo back clientId if provided)
	resp := map[string]interface{}{
		"id":        msg.ID.Hex(),
		"sender":    msg.Sender,
		"content":   msg.Content,
		"createdAt": msg.CreatedAt,
		"media":     msg.Media,
	}
	if body.ClientID != "" {
		resp["clientId"] = body.ClientID
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func EditMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
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
	ctx := r.Context()
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
	ctx := r.Context()
	chatID, err := primitive.ObjectIDFromHex(ps.ByName("chatid"))
	if err != nil {
		http.Error(w, "invalid chatid", http.StatusBadRequest)
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

	filter := bson.M{"chatid": chatID}
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
	ctx := r.Context()

	// First, find all chats the user participates in
	cursor, err := db.ChatsCollection.Find(ctx, bson.M{"participants": user})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	type Unread struct {
		ChatID string `json:"chatid"`
		Count  int64  `json:"count"`
	}
	var result []Unread

	for cursor.Next(ctx) {
		var chat Chat
		if err := cursor.Decode(&chat); err != nil {
			continue
		}
		count, err := db.MessagesCollection.CountDocuments(ctx, bson.M{
			"chatid": chat.ID,
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
	ctx := r.Context()
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
