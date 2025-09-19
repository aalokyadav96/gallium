package discord

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"naevis/db"
	"naevis/middleware"

	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	clients = struct {
		sync.RWMutex
		m map[string]*websocket.Conn
	}{m: make(map[string]*websocket.Conn)}

	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

// HandleWebSocket manages connections & messages
func HandleWebSocket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	// userID := utils.GetUserIDFromRequest(r)

	var token = "Bearer " + r.URL.Query().Get("token")
	claims, err := middleware.ValidateJWT(token)
	if err != nil {
		log.Println(err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := claims.UserID
	log.Println("-------------------------", userID)

	log.Println("-_-_-__--__-____--_------_--______-_-_ : ", userID)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}

	clients.Lock()
	clients.m[userID] = conn
	clients.Unlock()
	log.Println("WS connected:", userID)

	defer func() {
		clients.Lock()
		delete(clients.m, userID)
		clients.Unlock()
		conn.Close()
		log.Println("WS disconnected:", userID)
	}()

	// ping ticker
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			<-ticker.C
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				return
			}
		}
	}()

	for {
		var in IncomingWSMessage
		if err := conn.ReadJSON(&in); err != nil {
			log.Printf("Read error from %s: %v", userID, err)
			break
		}

		switch in.Type {
		case "message":
			cid, err := primitive.ObjectIDFromHex(in.ChatID)
			if err != nil {
				log.Printf("Invalid chatId from %s: %v", userID, err)
				break
			}

			msg, err := persistMessage(ctx, cid, userID, in.Content, in.MediaURL, in.MediaType)
			if err != nil {
				log.Printf("Failed to persist message from %s: %v", userID, err)
				break
			}

			payload := map[string]interface{}{
				"type":      "message",
				"id":        msg.ID.Hex(),
				"sender":    msg.Sender,
				"content":   msg.Content,
				"createdAt": msg.CreatedAt,
				"media":     msg.Media,
			}
			if in.ClientID != "" {
				payload["clientId"] = in.ClientID
			}

			broadcastToChat(ctx, in.ChatID, payload)

		case "typing":
			broadcastToChat(ctx, in.ChatID, map[string]interface{}{
				"type":   "typing",
				"sender": userID,
				"chatId": in.ChatID,
			})

		case "presence":
			broadcastGlobal(map[string]interface{}{
				"type":   "presence",
				"from":   userID,
				"online": in.Online,
			})

		default:
			log.Printf("Unknown WebSocket type from %s: %s", userID, in.Type)
		}
	}
}

//
// ==== Broadcasting ====
//

// Broadcast to all chat participants
func broadcastToChat(ctx context.Context, chatHex string, payload interface{}) {
	cid, err := primitive.ObjectIDFromHex(chatHex)
	if err != nil {
		log.Printf("Invalid chatHex in broadcast: %v", chatHex)
		return
	}

	var chat Chat
	if err := db.ChatsCollection.FindOne(ctx, bson.M{"_id": cid}).Decode(&chat); err != nil {
		log.Printf("Chat not found for broadcast: %v", cid)
		return
	}

	clients.RLock()
	peers := make([]*websocket.Conn, 0, len(chat.Participants))
	for _, p := range chat.Participants {
		if conn, ok := clients.m[p]; ok {
			peers = append(peers, conn)
		}
	}
	clients.RUnlock()

	for _, conn := range peers {
		if err := conn.WriteJSON(payload); err != nil {
			conn.Close()
			clients.Lock()
			for uid, c := range clients.m {
				if c == conn {
					delete(clients.m, uid)
					break
				}
			}
			clients.Unlock()
		}
	}
}

// Broadcast to all connected users
func broadcastGlobal(payload interface{}) {
	clients.RLock()
	conns := make(map[string]*websocket.Conn, len(clients.m))
	for id, conn := range clients.m {
		conns[id] = conn
	}
	clients.RUnlock()

	for id, conn := range conns {
		if err := conn.WriteJSON(payload); err != nil {
			conn.Close()
			clients.Lock()
			delete(clients.m, id)
			clients.Unlock()
		}
	}
}

//
// ==== Persistence ====
//

func persistMediaMessage(ctx context.Context, chatID primitive.ObjectID, sender, mediaURL, mediaType string) (*Message, error) {
	return persistMessage(ctx, chatID, sender, "", mediaURL, mediaType)
}

func persistMessage(ctx context.Context, chatID primitive.ObjectID, sender, content, mediaURL, mediaType string) (*Message, error) {
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

	db.ChatsCollection.UpdateOne(ctx,
		bson.M{"_id": chatID},
		bson.M{"$set": bson.M{"updatedAt": time.Now()}},
	)

	return msg, nil
}

//
// ==== Helpers ====
//

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
