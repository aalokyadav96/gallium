package discord

import (
	"log"
	"net/http"
	"sync"
	"time"

	"naevis/db"
	"naevis/utils"

	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	// ctx = context.Background()

	clients = struct {
		sync.RWMutex
		m map[string]*websocket.Conn
	}{m: make(map[string]*websocket.Conn)}

	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

// HandleWebSocket (updated parts)
// Replace the message-handling switch (or integrate these changes into your existing function)
func HandleWebSocket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userID := utils.GetUserIDFromRequest(r)
	log.Println("=================================", userID)
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

	// ping ticker (unchanged)
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
		var in struct {
			Type      string `json:"type"`
			ChatID    string `json:"chatId"`
			Content   string `json:"content"`
			MediaURL  string `json:"mediaUrl"`
			MediaType string `json:"mediaType"`
			Online    bool   `json:"online"`
			ClientID  string `json:"clientId,omitempty"`
		}

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

			msg, err := persistMessage(cid, userID, in.Content, in.MediaURL, in.MediaType)
			if err != nil {
				log.Printf("Failed to persist message from %s: %v", userID, err)
				break
			}

			// Broadcast a flattened payload that the frontend expects.
			// Include clientId if provided so the client can reconcile pending messages.
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

			broadcastToChat(in.ChatID, payload)

		case "typing":
			// broadcast with 'sender' key to match frontend expectations
			broadcastToChat(in.ChatID, map[string]interface{}{
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

// Broadcast to all chat participants (safe)
func broadcastToChat(chatHex string, payload interface{}) {
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

	// Gather recipients under read lock
	clients.RLock()
	peers := make([]*websocket.Conn, 0, len(chat.Participants))
	for _, p := range chat.Participants {
		if conn, ok := clients.m[p]; ok {
			peers = append(peers, conn)
		}
	}
	clients.RUnlock()

	// Write to each safely
	for _, conn := range peers {
		if err := conn.WriteJSON(payload); err != nil {
			conn.Close()
			// Remove closed connection
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

// Broadcast to all connected users (presence updates, etc.)
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
