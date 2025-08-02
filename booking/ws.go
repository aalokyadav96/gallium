package booking

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins — adjust for production if needed
		return true
	},
}

var (
	subscribers = make(map[string][]*websocket.Conn)
	mu          sync.Mutex
)

func HandleWS(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entityType")
	entityId := ps.ByName("entityId")
	key := entityType + "_" + entityId

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "WebSocket upgrade failed", http.StatusBadRequest)
		return
	}

	mu.Lock()
	subscribers[key] = append(subscribers[key], conn)
	mu.Unlock()

	for {
		// This keeps the connection alive until the client disconnects
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}

	// Clean up on disconnect
	mu.Lock()
	conns := subscribers[key]
	newList := make([]*websocket.Conn, 0, len(conns))
	for _, c := range conns {
		if c != conn {
			newList = append(newList, c)
		}
	}
	subscribers[key] = newList
	mu.Unlock()

	conn.Close()
}

func broadcast(key string, val []byte) {
	mu.Lock()
	defer mu.Unlock()

	conns := subscribers[key]
	newList := conns[:0]

	for _, conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, val); err == nil {
			newList = append(newList, conn)
		} else {
			conn.Close()
		}
	}

	subscribers[key] = newList
}
