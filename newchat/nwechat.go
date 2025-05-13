package newchat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"naevis/db"
	"naevis/utils"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// --- TYPES ------------------------------------------------

// type Message struct {
// 	Room      string `json:"room" bson:"room"`
// 	SenderID  string `json:"senderId" bson:"senderId"`
// 	Content   string `json:"content,omitempty" bson:"content,omitempty"`
// 	Filename  string `json:"filename,omitempty" bson:"filename,omitempty"`
// 	Path      string `json:"path,omitempty" bson:"path,omitempty"`
// 	Timestamp int64  `json:"timestamp" bson:"timestamp"`
// }

// type Client struct {
// 	Conn   *websocket.Conn
// 	Send   chan []byte
// 	Room   string
// 	UserID string
// }

// type broadcastMsg struct {
// 	Room string
// 	Data []byte
// }

// type Hub struct {
// 	rooms      map[string]map[*Client]bool
// 	register   chan *Client
// 	unregister chan *Client
// 	broadcast  chan broadcastMsg
// 	mu         sync.Mutex
// }

// func NewHub() *Hub {
// 	return &Hub{
// 		rooms:      make(map[string]map[*Client]bool),
// 		register:   make(chan *Client),
// 		unregister: make(chan *Client),
// 		broadcast:  make(chan broadcastMsg),
// 	}
// }

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
// 			for c := range h.rooms[m.Room] {
// 				select {
// 				case c.Send <- m.Data:
// 				default:
// 					close(c.Send)
// 					delete(h.rooms[m.Room], c)
// 				}
// 			}
// 			h.mu.Unlock()
// 		}
// 	}
// }

// // --- GLOBALS & HELPERS ------------------------------------

// var upgrader = websocket.Upgrader{
// 	CheckOrigin: func(r *http.Request) bool { return true },
// }

// func MustEnv(key string) string {
// 	v := os.Getenv(key)
// 	if v == "" {
// 		log.Fatalf("missing env %s", key)
// 	}
// 	return v
// }

// func ValidateToken(token string) (string, error) {
// 	if token == "" {
// 		return "", errors.New("no token")
// 	}
// 	return token, nil // Replace with real JWT handling
// }

// func AuthMiddleware(next httprouter.Handle) httprouter.Handle {
// 	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 		tok := r.Header.Get("Authorization")
// 		userID, err := ValidateToken(strings.TrimPrefix(tok, "Bearer "))
// 		if err != nil {
// 			http.Error(w, "unauthorized", http.StatusUnauthorized)
// 			return
// 		}
// 		ctx := context.WithValue(r.Context(), globals.UserIDKey, userID)
// 		next(w, r.WithContext(ctx), ps)
// 	}
// }

// func authorizeMembership(userID, room string) bool {
// 	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
// 	defer cancel()
// 	cnt, _ := db.ChatsCollection.CountDocuments(ctx, bson.M{"room": room, "userId": userID})
// 	return cnt > 0
// }

// SanitizeFilename removes potentially dangerous characters
func SanitizeFilename(name string) string {
	// Remove any path traversal, non-alphanumeric (except dash/underscore/dot)
	re := regexp.MustCompile(`[^\w.\-]`)
	clean := re.ReplaceAllString(filepath.Base(name), "_")
	if clean == "" {
		return "file"
	}
	return clean
}

// // --- WEB SOCKET HANDLER -----------------------------------

// func WebSocketHandler(hub *Hub) httprouter.Handle {
// 	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 		room := ps.ByName("room")
// 		log.Println("-----", room)
// 		// ctxVal := r.Context().Value(globals.UserIDKey)
// 		// userID, ok := ctxVal.(string)
// 		// if !ok || userID == "" {
// 		// 	http.Error(w, "unauthorized", http.StatusUnauthorized)
// 		// 	return
// 		// }

// 		// if !authorizeMembership(userID, room) {
// 		// 	http.Error(w, "forbidden", http.StatusForbidden)
// 		// 	return
// 		// }

// 		var userID = "ter"

// 		conn, err := upgrader.Upgrade(w, r, nil)
// 		if err != nil {
// 			log.Println("upgrade error:", err)
// 			return
// 		}

// 		client := &Client{
// 			Conn:   conn,
// 			Send:   make(chan []byte, 256),
// 			Room:   room,
// 			UserID: userID,
// 		}

// 		// Fetch last 30 messages
// 		go func() {
// 			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 			defer cancel()

// 			opts := options.Find().
// 				SetSort(bson.D{{Key: "timestamp", Value: -1}}).
// 				SetLimit(30)

// 			cur, err := db.MessagesCollection.Find(ctx, bson.M{"room": room}, opts)
// 			if err == nil {
// 				defer cur.Close(ctx)
// 				var msgs []Message
// 				if err = cur.All(ctx, &msgs); err == nil {
// 					for i := len(msgs) - 1; i >= 0; i-- {
// 						if data, err := json.Marshal(msgs[i]); err == nil {
// 							client.Send <- data
// 						}
// 					}
// 				}
// 			}
// 		}()

// 		hub.register <- client
// 		go writePump(client)
// 		go readPump(client, hub)
// 	}
// }

// func writePump(c *Client) {
// 	defer c.Conn.Close()
// 	for msg := range c.Send {
// 		if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
// 			break
// 		}
// 	}
// }

// // func readPump(c *Client, hub *Hub) {
// // 	defer func() {
// // 		hub.unregister <- c
// // 		c.Conn.Close()
// // 	}()

// // 	for {
// // 		_, data, err := c.Conn.ReadMessage()
// // 		if err != nil {
// // 			break
// // 		}

// // 		msg := Message{
// // 			MessageID: utils.GenerateIntID(16),
// // 			Room:      c.Room,
// // 			SenderID:  c.UserID,
// // 			Content:   string(data),
// // 			Timestamp: time.Now().Unix(),
// // 		}

// // 		_, err = db.MessagesCollection.InsertOne(context.Background(), msg)
// // 		if err != nil {
// // 			log.Printf("insert error: %v", err)
// // 			continue
// // 		}

// //			out, err := json.Marshal(msg)
// //			if err == nil {
// //				hub.broadcast <- broadcastMsg{Room: c.Room, Data: out}
// //			}
// //		}
// //	}

// type inboundPayload struct {
// 	Action  string `json:"action"`            // "chat", "edit", "delete"
// 	ID      string `json:"id,omitempty"`      // for edit/delete
// 	Content string `json:"content,omitempty"` // for chat/edit
// }

// func readPump(c *Client, hub *Hub) {
// 	defer func() {
// 		hub.unregister <- c
// 		c.Conn.Close()
// 	}()

// 	for {
// 		_, data, err := c.Conn.ReadMessage()
// 		if err != nil {
// 			break
// 		}

// 		// 1) Parse control vs chat:
// 		var in inboundPayload
// 		if err := json.Unmarshal(data, &in); err != nil {
// 			log.Printf("invalid payload: %v", err)
// 			continue
// 		}

// 		switch in.Action {
// 		case "chat":
// 			// build a normal Message
// 			msg := Message{
// 				MessageID: utils.GenerateIntID(16),
// 				Room:      c.Room,
// 				SenderID:  c.UserID,
// 				Content:   in.Content,
// 				Timestamp: time.Now().Unix(),
// 			}
// 			// store
// 			if _, err := db.MessagesCollection.InsertOne(context.Background(), msg); err != nil {
// 				log.Printf("insert error: %v", err)
// 				continue
// 			}
// 			// broadcast
// 			out, _ := json.Marshal(msg)
// 			hub.broadcast <- broadcastMsg{Room: c.Room, Data: out}

// 		case "edit":
// 			if err := UpdateMessage(c.UserID, in.ID, in.Content); err != nil {
// 				log.Printf("edit failed: %v", err)
// 				continue
// 			}
// 			// optionally broadcast an “edited” event
// 			evt := struct {
// 				Action  string `json:"action"`
// 				ID      string `json:"id"`
// 				Content string `json:"content"`
// 			}{"edit", in.ID, in.Content}
// 			if out, err := json.Marshal(evt); err == nil {
// 				hub.broadcast <- broadcastMsg{Room: c.Room, Data: out}
// 			}

// 		case "delete":
// 			if err := DeleteMessage(c.UserID, in.ID); err != nil {
// 				log.Printf("delete failed: %v", err)
// 				continue
// 			}
// 			// broadcast a “delete” event so clients can remove it from their UI
// 			evt := struct {
// 				Action string `json:"action"`
// 				ID     string `json:"id"`
// 			}{"delete", in.ID}
// 			if out, err := json.Marshal(evt); err == nil {
// 				hub.broadcast <- broadcastMsg{Room: c.Room, Data: out}
// 			}

// 		default:
// 			log.Printf("unknown action: %q", in.Action)
// 		}
// 	}
// }

// type Message struct {
// 	// ID        primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
// 	MessageID string `json:"messageid,omitempty" bson:"messageid,omitempty"`
// 	Room      string `json:"room" bson:"room"`
// 	SenderID  string `json:"senderId" bson:"senderId"`
// 	Content   string `json:"content,omitempty" bson:"content,omitempty"`
// 	Filename  string `json:"filename,omitempty" bson:"filename,omitempty"`
// 	Path      string `json:"path,omitempty" bson:"path,omitempty"`
// 	Timestamp int64  `json:"timestamp" bson:"timestamp"`
// }

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
}

func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan broadcastMsg),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			if h.rooms[c.Room] == nil {
				h.rooms[c.Room] = make(map[*Client]bool)
			}
			h.rooms[c.Room][c] = true
			h.mu.Unlock()

		case c := <-h.unregister:
			h.mu.Lock()
			if conns := h.rooms[c.Room]; conns != nil {
				delete(conns, c)
				close(c.Send)
			}
			h.mu.Unlock()

		case m := <-h.broadcast:
			h.mu.Lock()
			for c := range h.rooms[m.Room] {
				select {
				case c.Send <- m.Data:
				default:
					close(c.Send)
					delete(h.rooms[m.Room], c)
				}
			}
			h.mu.Unlock()
		}
	}
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

// inboundPayload represents what clients send us:
type inboundPayload struct {
	Action  string `json:"action"`            // "chat", "edit", "delete"
	ID      string `json:"id,omitempty"`      // for edit/delete
	Content string `json:"content,omitempty"` // for chat/edit
}

// outboundPayload is what we broadcast to every client:
type outboundPayload struct {
	Action    string `json:"action"`             // "chat", "edit", "delete"
	ID        string `json:"id"`                 // message ID
	Room      string `json:"room,omitempty"`     // only on chat
	SenderID  string `json:"senderId,omitempty"` // only on chat
	Content   string `json:"content,omitempty"`  // chat or edit text
	Filename  string `json:"filename,omitempty"` // file uploads
	Path      string `json:"path,omitempty"`     // file uploads
	Timestamp int64  `json:"timestamp"`          // unix seconds
}

func WebSocketHandler(hub *Hub) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		room := ps.ByName("room")
		// (auth/membership checks here...)
		userID := "ter" // replace with real ctx user

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

		// send last 30 messages as chat actions
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			opts := options.Find().
				SetSort(bson.D{{Key: "timestamp", Value: -1}}).
				SetLimit(30)

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
			// send oldest → newest
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
			// persist
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
			// broadcast
			out := outboundPayload{
				Action:    "chat",
				ID:        msg.MessageID,
				Room:      msg.Room,
				SenderID:  msg.SenderID,
				Content:   msg.Content,
				Timestamp: msg.Timestamp,
			}
			if data, _ := json.Marshal(out); data != nil {
				hub.broadcast <- broadcastMsg{Room: c.Room, Data: data}
			}

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
			if data, _ := json.Marshal(out); data != nil {
				hub.broadcast <- broadcastMsg{Room: c.Room, Data: data}
			}

		case "delete":
			if err := DeleteMessage(c.UserID, in.ID); err != nil {
				log.Println("delete failed:", err)
				continue
			}
			out := outboundPayload{
				Action: "delete",
				ID:     in.ID,
			}
			if data, _ := json.Marshal(out); data != nil {
				hub.broadcast <- broadcastMsg{Room: c.Room, Data: data}
			}

		default:
			log.Println("unknown action:", in.Action)
		}
	}
}

// Message schema stays the same:
type Message struct {
	MessageID string `json:"messageid,omitempty" bson:"messageid,omitempty"`
	Room      string `json:"room" bson:"room"`
	SenderID  string `json:"senderId" bson:"senderId"`
	Content   string `json:"content,omitempty" bson:"content,omitempty"`
	Filename  string `json:"filename,omitempty" bson:"filename,omitempty"`
	Path      string `json:"path,omitempty" bson:"path,omitempty"`
	Timestamp int64  `json:"timestamp" bson:"timestamp"`
}

// --- FILE UPLOAD HANDLER ----------------------------------

func UploadHandler(hub *Hub) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		// ctxVal := r.Context().Value(globals.UserIDKey)
		// userID, ok := ctxVal.(string)
		// if !ok || userID == "" {
		// 	http.Error(w, "unauthorized", http.StatusUnauthorized)
		// 	return
		// }

		var userID = "ter"

		room := r.FormValue("room")
		// if room == "" || !authorizeMembership(userID, room) {
		// 	http.Error(w, "forbidden", http.StatusForbidden)
		// 	return
		// }

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
		// path := fn
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
			SenderID:  userID,
			Filename:  hdr.Filename,
			Path:      fn,
			Timestamp: ts,
		}

		_, err = db.MessagesCollection.InsertOne(context.Background(), msg)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}

		out, err := json.Marshal(msg)
		if err != nil {
			http.Error(w, "marshal error", http.StatusInternalServerError)
			return
		}

		hub.broadcast <- broadcastMsg{Room: room, Data: out}

		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
	}
}

// --- ORPHANED FILE CLEANUP --------------------------------

func CleanupOrphans() {
	for {
		time.Sleep(24 * time.Hour)
		cleanOrphanedFiles()
	}
}

func cleanOrphanedFiles() {
	files := map[string]bool{}
	filepath.Walk("newchatpic", func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			files[p] = false
		}
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cur, err := db.MessagesCollection.Find(ctx, bson.M{"path": bson.M{"$exists": true}})
	if err != nil {
		log.Println("cleanup: db find error:", err)
		return
	}
	defer cur.Close(ctx)

	var m Message
	for cur.Next(ctx) {
		if err := cur.Decode(&m); err == nil {
			files[m.Path] = true
		}
	}
	for p, used := range files {
		if !used {
			if err := os.Remove(p); err != nil {
				log.Printf("cleanup: failed to remove %s: %v", p, err)
			}
		}
	}
}
