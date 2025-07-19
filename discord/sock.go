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

// WebSocket handler
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

	// Start ping ticker for connection keep-alive
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

			broadcastToChat(in.ChatID, map[string]interface{}{
				"type":    "message",
				"message": msg,
			})

		case "typing":
			broadcastToChat(in.ChatID, map[string]interface{}{
				"type":   "typing",
				"from":   userID,
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

// // Optional: broadcast to one user
// func broadcastToUser(userID string, payload interface{}) {
// 	clients.RLock()
// 	conn, ok := clients.m[userID]
// 	clients.RUnlock()
// 	if ok {
// 		_ = conn.WriteJSON(payload) // silent fail if disconnected
// 	}
// }

// // discord/socket.go
// package discord

// import (
// 	"log"
// 	"naevis/db"
// 	"naevis/utils"
// 	"net/http"
// 	"sync"

// 	"github.com/gorilla/websocket"
// 	"github.com/julienschmidt/httprouter"
// 	"go.mongodb.org/mongo-driver/bson"
// 	"go.mongodb.org/mongo-driver/bson/primitive"
// )

// var (
// 	clients = struct {
// 		sync.RWMutex
// 		m map[string]*websocket.Conn
// 	}{m: make(map[string]*websocket.Conn)}

// 	upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
// )

// func HandleWebSocket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	userID := utils.GetUserIDFromRequest(r)
// 	conn, err := upgrader.Upgrade(w, r, nil)
// 	if err != nil {
// 		log.Println("ws upgrade:", err)
// 		return
// 	}

// 	clients.Lock()
// 	clients.m[userID] = conn
// 	clients.Unlock()
// 	log.Println("WS connected:", userID)

// 	defer func() {
// 		clients.Lock()
// 		delete(clients.m, userID)
// 		clients.Unlock()
// 		conn.Close()
// 		log.Println("WS disconnected:", userID)
// 	}()

// 	for {
// 		var in struct {
// 			Type      string `json:"type"`
// 			ChatID    string `json:"chatId"`
// 			Content   string `json:"content"`
// 			MediaURL  string `json:"mediaUrl"`
// 			MediaType string `json:"mediaType"`
// 			Online    bool   `json:"online"`
// 		}
// 		if err := conn.ReadJSON(&in); err != nil {
// 			break
// 		}

// 		switch in.Type {
// 		case "message":
// 			cid, err := primitive.ObjectIDFromHex(in.ChatID)
// 			if err != nil {
// 				break
// 			}
// 			// Persist message (can be text, media, or both)
// 			msg, err := persistMessage(cid, userID, in.Content, in.MediaURL, in.MediaType)
// 			if err != nil {
// 				break
// 			}
// 			broadcastToChat(in.ChatID, map[string]interface{}{
// 				"type":    "message",
// 				"message": msg,
// 			})

// 		case "typing":
// 			broadcastToChat(in.ChatID, map[string]interface{}{
// 				"type":   "typing",
// 				"from":   userID,
// 				"chatId": in.ChatID,
// 			})

// 		case "presence":
// 			broadcastGlobal(map[string]interface{}{
// 				"type":   "presence",
// 				"from":   userID,
// 				"online": in.Online,
// 			})
// 		}
// 	}
// }

// func broadcastToChat(chatHex string, payload interface{}) {
// 	cid, _ := primitive.ObjectIDFromHex(chatHex)
// 	var chat Chat
// 	if err := db.ChatsCollection.FindOne(ctx, bson.M{"_id": cid}).Decode(&chat); err != nil {
// 		return
// 	}
// 	for _, p := range chat.Participants {
// 		clients.RLock()
// 		if peer, ok := clients.m[p]; ok {
// 			if err := peer.WriteJSON(payload); err != nil {
// 				peer.Close()
// 				delete(clients.m, p)
// 			}
// 		}
// 		clients.RUnlock()
// 	}
// }

// func broadcastGlobal(payload interface{}) {
// 	clients.RLock()
// 	defer clients.RUnlock()
// 	for id, conn := range clients.m {
// 		if err := conn.WriteJSON(payload); err != nil {
// 			conn.Close()
// 			delete(clients.m, id)
// 		}
// 	}
// }

// // func HandleWebSocket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// // 	userID := utils.GetUserIDFromRequest(r)
// // 	conn, err := upgrader.Upgrade(w, r, nil)
// // 	if err != nil {
// // 		log.Println("ws upgrade:", err)
// // 		return
// // 	}

// // 	clients.Lock()
// // 	clients.m[userID] = conn
// // 	clients.Unlock()
// // 	log.Println("WS connected:", userID)

// // 	defer func() {
// // 		clients.Lock()
// // 		delete(clients.m, userID)
// // 		clients.Unlock()
// // 		conn.Close()
// // 		log.Println("WS disconnected:", userID)
// // 	}()

// // 	for {
// // 		var in struct {
// // 			Type    string `json:"type"`
// // 			ChatID  string `json:"chatId"`
// // 			Content string `json:"content"`
// // 			Online  bool   `json:"online"`
// // 		}
// // 		if err := conn.ReadJSON(&in); err != nil {
// // 			break
// // 		}

// // 		switch in.Type {
// // 		case "message":
// // 			cid, err := primitive.ObjectIDFromHex(in.ChatID)
// // 			if err != nil {
// // 				break
// // 			}
// // 			msg, err := persistMessage(cid, userID, in.Content)
// // 			if err != nil {
// // 				break
// // 			}
// // 			broadcastToChat(in.ChatID, map[string]interface{}{
// // 				"type":    "message",
// // 				"message": msg,
// // 			})
// // 		case "typing":
// // 			broadcastToChat(in.ChatID, map[string]interface{}{
// // 				"type":   "typing",
// // 				"from":   userID,
// // 				"chatId": in.ChatID,
// // 			})
// // 		case "presence":
// // 			broadcastGlobal(map[string]interface{}{
// // 				"type":   "presence",
// // 				"from":   userID,
// // 				"online": in.Online,
// // 			})
// // 		}
// // 	}
// // }
