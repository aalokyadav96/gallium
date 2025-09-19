package newchat

import (
	"encoding/json"
	"testing"
	"time"
)

// // fake client that just records messages
// type fakeClient struct {
// 	received [][]byte
// }

// func (f *fakeClient) push(msg []byte) {
// 	f.received = append(f.received, msg)
// }

func TestHubRegisterBroadcastUnregister(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	// create fake client
	client := &Client{
		Send: make(chan []byte, 10),
		Room: "room1",
	}

	// register client
	hub.register <- client

	// broadcast a test message
	msg := outboundPayload{Action: "chat", Content: "hello test"}
	data, _ := json.Marshal(msg)
	hub.broadcast <- broadcastMsg{Room: "room1", Data: data}

	select {
	case got := <-client.Send:
		if string(got) != string(data) {
			t.Fatalf("expected %s, got %s", data, got)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for message")
	}

	// unregister client
	hub.unregister <- client
}
