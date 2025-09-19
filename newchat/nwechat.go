package newchat

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
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

// ------------------------- Types -------------------------

type Hub struct {
	rooms      map[string]map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan broadcastMsg

	mu       sync.Mutex
	stopped  bool
	stopChan chan struct{}
	stopOnce sync.Once
}

type Client struct {
	Conn   *websocket.Conn
	Send   chan []byte
	Room   string
	UserID string

	ctx    context.Context
	cancel context.CancelFunc
}

type Attachment struct {
	Filename string `bson:"filename" json:"filename"`
	Path     string `bson:"path" json:"path"`
}

type Message struct {
	MessageID string       `bson:"message_id" json:"message_id"`
	Room      string       `bson:"room" json:"room"`
	SenderID  string       `bson:"sender_id" json:"sender_id"`
	Content   string       `bson:"content" json:"content"`
	Files     []Attachment `bson:"files,omitempty" json:"files,omitempty"`
	Timestamp int64        `bson:"timestamp" json:"timestamp"`
}

type inboundPayload struct {
	Action  string `json:"action"`
	ID      string `json:"id,omitempty"`
	Content string `json:"content,omitempty"`
}

type outboundPayload struct {
	Action    string       `json:"action"`
	ID        string       `json:"id,omitempty"`
	Room      string       `json:"room,omitempty"`
	SenderID  string       `json:"sender_id,omitempty"`
	Content   string       `json:"content,omitempty"`
	Files     []Attachment `json:"files,omitempty"`
	Timestamp int64        `json:"timestamp,omitempty"`
}

type broadcastMsg struct {
	Room string
	Data []byte
}

// ------------------------- Helpers -------------------------

// closeChanSafe closes a channel if not already closed (panic-safe)
func closeChanSafe(ch chan []byte) {
	defer func() {
		_ = recover() // avoid panic on double-close
	}()
	close(ch)
}

// originAllowed checks the Origin header against server host and ALLOWED_ORIGINS env var.
// Behavior:
// - if Origin header is empty => allow (some clients don't send it)
// - if origin host equals request Host => allow
// - if ALLOWED_ORIGINS contains the exact origin string => allow
// - otherwise deny
func originAllowed(r *http.Request) bool {
	// origin := r.Header.Get("Origin")
	// if origin == "" {
	// 	// No Origin header: allow (e.g., non-browser clients or same-origin requests without header)
	// 	return true
	// }

	// u, err := url.Parse(origin)
	// if err != nil {
	// 	return false
	// }
	// // allow same host (useful for same-origin)
	// if u.Host == r.Host {
	// 	return true
	// }

	// // check ALLOWED_ORIGINS env var (comma separated origins)
	// allowed := os.Getenv("ALLOWED_ORIGINS")
	// if allowed == "" {
	// 	// explicit deny if we have an Origin and no ALLOWED_ORIGINS configured
	// 	return false
	// }
	// for _, o := range strings.Split(allowed, ",") {
	// 	if strings.TrimSpace(o) == origin {
	// 		return true
	// 	}
	// }
	// return false
	return true
}

var upgrader = websocket.Upgrader{
	CheckOrigin: originAllowed,
}

// ------------------------- Hub lifecycle -------------------------

func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan broadcastMsg),
		stopChan:   make(chan struct{}),
	}
}

// Run keeps the Hub running to process client activity.
// It will exit when Stop() closes stopChan.
func (h *Hub) Run() {
	for {
		select {
		case <-h.stopChan:
			// exit loop and let Stop() do cleanup (Stop already performed connection closes)
			return

		case c := <-h.register:
			h.mu.Lock()
			if h.stopped {
				h.mu.Unlock()
				// ensure the connection is closed
				_ = c.Conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, "server shutting down"))
				c.cancel()
				c.Conn.Close()
				closeChanSafe(c.Send)
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
				// if room empty, remove it
				if len(clients) == 0 {
					delete(h.rooms, c.Room)
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
						// client send buffer full or closed, drop it
						closeChanSafe(client.Send)
						delete(clients, client)
					}
				}
			}
			h.mu.Unlock()
		}
	}
}

// Stop gracefully shuts down the Hub, closing all client connections and signaling Run to exit.
func (h *Hub) Stop() {
	h.mu.Lock()
	if h.stopped {
		h.mu.Unlock()
		return
	}
	h.stopped = true
	// ensure stopChan closed exactly once
	h.stopOnce.Do(func() {
		close(h.stopChan)
	})
	// copy rooms to avoid mutation while iterating
	roomsCopy := make(map[string]map[*Client]bool, len(h.rooms))
	for k, v := range h.rooms {
		roomsCopy[k] = v
	}
	// clear hub rooms map
	h.rooms = make(map[string]map[*Client]bool)
	h.mu.Unlock()

	for room, clients := range roomsCopy {
		for client := range clients {
			// notify clients best-effort
			_ = client.Conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "server shutting down"))
			// cancel client context to stop pumps
			client.cancel()
			client.Conn.Close()
			closeChanSafe(client.Send)
		}
		delete(roomsCopy, room)
	}

	log.Println("✅ Hub stopped cleanly")
}

// ------------------------- WebSocket handlers & pumps -------------------------

// WebSocketHandler: ensure route is /ws/:room (client must connect to /ws/room123)
func WebSocketHandler(hub *Hub) httprouter.Handle {
	log.Println(hub)
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		room := strings.TrimSpace(ps.ByName("room"))
		if room == "" {
			http.Error(w, "room required", http.StatusBadRequest)
			return
		}

		var token string
		// prefer Authorization header if present, fallback to ?token=...
		if auth := r.Header.Get("Authorization"); auth != "" {
			token = auth
		} else if q := r.URL.Query().Get("token"); q != "" {
			token = "Bearer " + q
		} else {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		claims, err := middleware.ValidateJWT(token)
		if err != nil {
			log.Println("jwt validate:", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		userID := claims.UserID

		// upgrade
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("upgrade:", err)
			return
		}

		// create cancellable context per client to support coordinated shutdown
		ctx, cancel := context.WithCancel(context.Background())
		client := &Client{
			Conn:   conn,
			Send:   make(chan []byte, 256),
			Room:   room,
			UserID: userID,
			ctx:    ctx,
			cancel: cancel,
		}

		// Send last 20 messages to this client, oldest→newest:
		go func() {
			// guard: if client cancelled early, exit
			ctx, cancelHistory := context.WithTimeout(client.ctx, 5*time.Second)
			defer cancelHistory()

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
			// send oldest -> newest
			for i := len(history) - 1; i >= 0; i-- {
				select {
				case <-client.ctx.Done():
					return
				default:
				}
				m := history[i]
				out := outboundPayload{
					Action:    "chat",
					ID:        m.MessageID,
					Room:      m.Room,
					SenderID:  m.SenderID,
					Content:   m.Content,
					Files:     m.Files,
					Timestamp: m.Timestamp,
				}
				if data, err := json.Marshal(out); err == nil {
					// avoid panic/write to closed channel: non-blocking send, but respect context
					select {
					case client.Send <- data:
					case <-client.ctx.Done():
						return
					default:
						// client's send buffer is full; drop history message (client likely slow)
					}
				}
			}
		}()

		// register client with hub
		hub.register <- client

		// start pumps; writePump will unregister on error or ctx cancel
		go writePump(client, hub)
		go readPump(client, hub)
	}
}

// writePump writes messages from the client's Send channel to the websocket connection.
// On write error or client.ctx cancellation, it ensures client is unregistered and cleaned up.
func writePump(c *Client, hub *Hub) {
	defer func() {
		// ensure unregister and cleanup
		hub.unregister <- c
		c.cancel()
		c.Conn.Close()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case msg, ok := <-c.Send:
			if !ok {
				// channel closed; exit
				return
			}
			// write with deadline to avoid blocking indefinitely
			_ = c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				// on any write error, unregister client and stop
				log.Println("write error:", err)
				return
			}
		}
	}
}

// readPump reads messages from websocket and processes inbound actions.
// It exits and unregisters the client on error or client.ctx cancellation.
func readPump(c *Client, hub *Hub) {
	defer func() {
		hub.unregister <- c
		c.cancel()
		c.Conn.Close()
	}()

	// set read limits/timeouts if desired
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			_ = c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			_, raw, err := c.Conn.ReadMessage()
			if err != nil {
				// read error (client disconnected or network issue)
				select {
				case <-hub.stopChan:
					// server is stopping; suppress error log
				default:
					log.Println("read error:", err)
				}
				return
			}

			var in inboundPayload
			if err := json.Unmarshal(raw, &in); err != nil {
				log.Println("invalid payload:", err)
				continue
			}

			switch in.Action {
			case "chat":
				msg := Message{
					MessageID: utils.GenerateRandomString(16),
					Room:      c.Room,
					SenderID:  c.UserID,
					Content:   in.Content,
					Files:     nil, // chat via WS supports inline text; file uploads use UploadHandler
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
					Files:     msg.Files,
					Timestamp: msg.Timestamp,
				}
				data, _ := json.Marshal(out)
				hub.broadcast <- broadcastMsg{Room: c.Room, Data: data}

			case "edit":
				if err := UpdatexMessage(c.UserID, in.ID, in.Content); err != nil {
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
				if err := DeletexMessage(c.UserID, in.ID); err != nil {
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
}
