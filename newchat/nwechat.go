package newchat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"naevis/db"
	"naevis/middleware"
	"naevis/utils"

	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ——————————————————————————————————————————————————————————
// SanitizeFilename: exactly as before
func SanitizeFilename(name string) string {
	re := regexp.MustCompile(`[^\w.\-]`)
	clean := re.ReplaceAllString(filepath.Base(name), "_")
	if clean == "" {
		return "file"
	}
	return clean
}

// ——————————————————————————————————————————————————————————
// Hub, Client, broadcastMsg, outboundPayload, Message
type Client struct {
	Conn   *websocket.Conn
	Send   chan []byte
	Room   string
	UserID string
}

type broadcastMsg struct {
	Room string
	Data []byte
}

type Hub struct {
	rooms      map[string]map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan broadcastMsg
	mu         sync.Mutex
	stopped    bool
}

// func (h *Hub) Stop() {
// 	panic("unimplemented")
// }

func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan broadcastMsg),
	}
}

// func (h *Hub) Run() {
// 	for {
// 		select {
// 		case c := <-h.register:
// 			h.mu.Lock()
// 			if h.rooms[c.Room] == nil {
// 				h.rooms[c.Room] = make(map[*Client]bool)
// 			}
// 			h.rooms[c.Room][c] = true
// 			h.mu.Unlock()

// 		case c := <-h.unregister:
// 			h.mu.Lock()
// 			if conns := h.rooms[c.Room]; conns != nil {
// 				delete(conns, c)
// 				close(c.Send)
// 			}
// 			h.mu.Unlock()

// 		case m := <-h.broadcast:
// 			h.mu.Lock()
// 			if conns := h.rooms[m.Room]; conns != nil {
// 				for client := range conns {
// 					select {
// 					case client.Send <- m.Data:
// 					default:
// 						close(client.Send)
// 						delete(conns, client)
// 					}
// 				}
// 			}
// 			h.mu.Unlock()
// 		}
// 	}
// }

// Run keeps the Hub running to process client activity
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			if h.stopped {
				h.mu.Unlock()
				c.Conn.Close()
				continue
			}
			if h.rooms[c.Room] == nil {
				h.rooms[c.Room] = make(map[*Client]bool)
			}
			h.rooms[c.Room][c] = true
			h.mu.Unlock()

		case c := <-h.unregister:
			h.mu.Lock()
			if clients := h.rooms[c.Room]; clients != nil {
				if _, ok := clients[c]; ok {
					delete(clients, c)
					closeChanSafe(c.Send)
				}
			}
			h.mu.Unlock()

		case m := <-h.broadcast:
			h.mu.Lock()
			if clients := h.rooms[m.Room]; clients != nil {
				for client := range clients {
					select {
					case client.Send <- m.Data:
					default:
						closeChanSafe(client.Send)
						delete(clients, client)
					}
				}
			}
			h.mu.Unlock()
		}
	}
}

// Stop gracefully shuts down the Hub, closing all client connections
func (h *Hub) Stop() {
	h.mu.Lock()
	if h.stopped {
		h.mu.Unlock()
		return
	}
	h.stopped = true

	for room, clients := range h.rooms {
		for client := range clients {
			// Best effort to notify clients
			_ = client.Conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "server shutting down"))

			client.Conn.Close()
			closeChanSafe(client.Send)
		}
		delete(h.rooms, room)
	}
	h.mu.Unlock()

	log.Println("✅ Hub stopped cleanly")
}

// closeChanSafe closes a channel if not already closed (panic-safe)
func closeChanSafe(ch chan []byte) {
	defer func() {
		_ = recover() // avoid panic on double-close
	}()
	close(ch)
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

// inboundPayload: same as yours
type inboundPayload struct {
	Action  string `json:"action"`
	ID      string `json:"id,omitempty"`
	Content string `json:"content,omitempty"`
}

// outboundPayload: always include Room even on file uploads,
// so client knows which room it’s for.
type outboundPayload struct {
	Action    string `json:"action"`
	ID        string `json:"id"`
	Room      string `json:"room,omitempty"`
	SenderID  string `json:"senderId,omitempty"`
	Content   string `json:"content,omitempty"`
	Filename  string `json:"filename,omitempty"`
	Path      string `json:"path,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// Message: same schema; notice the BSON tags match your collection.
type Message struct {
	MessageID string `bson:"messageid,omitempty" json:"messageid,omitempty"`
	Room      string `bson:"room" json:"room"`
	SenderID  string `bson:"senderId" json:"senderId"`
	Content   string `bson:"content,omitempty" json:"content,omitempty"`
	Filename  string `bson:"filename,omitempty" json:"filename,omitempty"`
	Path      string `bson:"path,omitempty" json:"path,omitempty"`
	Timestamp int64  `bson:"timestamp" json:"timestamp"`
}

// ——————————————————————————————————————————————————————————
// WebSocketHandler: ensure route is /ws/:room (client must connect to /ws/room123)
func WebSocketHandler(hub *Hub) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		room := strings.TrimSpace(ps.ByName("room"))
		if room == "" {
			http.Error(w, "room required", http.StatusBadRequest)
			return
		}
		var token = "Bearer " + r.URL.Query().Get("token")
		claims, err := middleware.ValidateJWT(token)
		if err != nil {
			log.Println(err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userID := claims.UserID
		log.Println("-------------------------", userID)
		// userID := "ter" // replace with real user ID from ctx

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("upgrade:", err)
			return
		}
		client := &Client{
			Conn:   conn,
			Send:   make(chan []byte, 256),
			Room:   room,
			UserID: userID,
		}

		// Send last 20 messages to this client, oldest→newest:
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			opts := options.Find().
				SetSort(bson.D{{Key: "timestamp", Value: -1}}).
				SetLimit(20)

			cur, err := db.MessagesCollection.Find(ctx, bson.M{"room": room}, opts)
			if err != nil {
				log.Println("history find:", err)
				return
			}
			defer cur.Close(ctx)

			var history []Message
			if err := cur.All(ctx, &history); err != nil {
				log.Println("history decode:", err)
				return
			}
			for i := len(history) - 1; i >= 0; i-- {
				m := history[i]
				out := outboundPayload{
					Action:    "chat",
					ID:        m.MessageID,
					Room:      m.Room,
					SenderID:  m.SenderID,
					Content:   m.Content,
					Filename:  m.Filename,
					Path:      m.Path,
					Timestamp: m.Timestamp,
				}
				if data, err := json.Marshal(out); err == nil {
					client.Send <- data
				}
			}
		}()

		hub.register <- client
		go writePump(client)
		go readPump(client, hub)
	}
}

func writePump(c *Client) {
	defer c.Conn.Close()
	for msg := range c.Send {
		if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			break
		}
	}
}

func readPump(c *Client, hub *Hub) {
	defer func() {
		hub.unregister <- c
		c.Conn.Close()
	}()

	for {
		_, raw, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		var in inboundPayload
		if err := json.Unmarshal(raw, &in); err != nil {
			log.Println("invalid payload:", err)
			continue
		}

		switch in.Action {
		case "chat":
			msg := Message{
				MessageID: utils.GenerateIntID(16),
				Room:      c.Room,
				SenderID:  c.UserID,
				Content:   in.Content,
				Timestamp: time.Now().Unix(),
			}
			if _, err := db.MessagesCollection.InsertOne(context.Background(), msg); err != nil {
				log.Println("insert:", err)
				continue
			}
			out := outboundPayload{
				Action:    "chat",
				ID:        msg.MessageID,
				Room:      msg.Room,
				SenderID:  msg.SenderID,
				Content:   msg.Content,
				Filename:  msg.Filename,
				Path:      msg.Path,
				Timestamp: msg.Timestamp,
			}
			data, _ := json.Marshal(out)
			hub.broadcast <- broadcastMsg{Room: c.Room, Data: data}

		case "edit":
			if err := UpdateMessage(c.UserID, in.ID, in.Content); err != nil {
				log.Println("edit failed:", err)
				continue
			}
			out := outboundPayload{
				Action:    "edit",
				ID:        in.ID,
				Content:   in.Content,
				Timestamp: time.Now().Unix(),
			}
			data, _ := json.Marshal(out)
			hub.broadcast <- broadcastMsg{Room: c.Room, Data: data}

		case "delete":
			if err := DeleteMessage(c.UserID, in.ID); err != nil {
				log.Println("delete failed:", err)
				continue
			}
			out := outboundPayload{
				Action: "delete",
				ID:     in.ID,
			}
			data, _ := json.Marshal(out)
			hub.broadcast <- broadcastMsg{Room: c.Room, Data: data}

		default:
			log.Println("unknown action:", in.Action)
		}
	}
}

// ——————————————————————————————————————————————————————————
// UploadHandler: wrap the file message in outboundPayload so clients ignore
// anything not matching “action”:“chat” and filter by .Room on the client.
func UploadHandler(hub *Hub) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		// 1) Validate JWT
		token := r.Header.Get("Authorization")

		// 1) Validate JWT
		claims, err := middleware.ValidateJWT(token)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		room := strings.TrimSpace(r.FormValue("chat"))
		if room == "" {
			http.Error(w, "room missing", http.StatusBadRequest)
			return
		}

		if err := r.ParseMultipartForm(12 << 20); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		file, hdr, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "file error", http.StatusBadRequest)
			return
		}
		defer file.Close()

		mediaType, _, _ := mime.ParseMediaType(hdr.Header.Get("Content-Type"))
		if !(strings.HasPrefix(mediaType, "image/") ||
			strings.HasPrefix(mediaType, "video/") ||
			mediaType == "audio/mpeg") ||
			hdr.Size > 10<<20 {

			http.Error(w, "invalid file", http.StatusBadRequest)
			return
		}

		ts := time.Now().Unix()
		safeName := SanitizeFilename(hdr.Filename)
		fn := fmt.Sprintf("%d_%s", ts, safeName)
		path := filepath.Join("static", "newchatpic", fn)
		os.MkdirAll("static/newchatpic", 0755)

		dst, err := os.Create(path)
		if err != nil {
			http.Error(w, "save error", http.StatusInternalServerError)
			return
		}
		defer dst.Close()
		io.Copy(dst, file)

		msg := Message{
			MessageID: utils.GenerateIntID(16),
			Room:      room,
			SenderID:  claims.UserID,
			Filename:  hdr.Filename,
			Path:      fn,
			Timestamp: ts,
		}
		if _, err := db.MessagesCollection.InsertOne(context.Background(), msg); err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}

		out := outboundPayload{
			Action:    "chat",
			ID:        msg.MessageID,
			Room:      msg.Room,
			SenderID:  msg.SenderID,
			Filename:  msg.Filename,
			Path:      msg.Path,
			Timestamp: msg.Timestamp,
		}
		data, _ := json.Marshal(out)
		hub.broadcast <- broadcastMsg{Room: msg.Room, Data: data}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}
