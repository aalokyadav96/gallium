package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"naevis/globals"
	"naevis/models"
	"naevis/search"
)

// Index represents an indexing-related message to be emitted.
type Index struct {
	EntityType string `json:"entity_type"`
	Method     string `json:"method"`
	EntityId   string `json:"entity_id"`
	ItemId     string `json:"item_id"`
	ItemType   string `json:"item_type"`
}

// Notify is a placeholder for broadcasting events without indexing.
func Notify(eventName string, content models.Index) error {
	fmt.Println(eventName, "Notified", content)
	return nil
}

// Emit now publishes indexing events to Redis instead of running immediately
func Emit(ctx context.Context, eventName string, content models.Index) {
	log.Printf("[Emit] START eventName=%s content=%+v", eventName, content)

	data, err := json.Marshal(content)
	if err != nil {
		log.Printf("[Emit] Failed to marshal event content: %v", err)
		return
	}

	if err := globals.RedisClient.Publish(context.Background(), "indexing-events", data).Err(); err != nil {
		log.Printf("[Emit] Failed to publish event to Redis: %v", err)
		return
	}

	log.Printf("[Emit] Event published to channel 'indexing-events'")
	log.Printf("[Emit] END")
}

// // Emit sends the event to the indexing system.
// // eventName is for logging/tracking purposes only.
// func Emit(ctx context.Context, eventName string, content models.Index) {
// 	fmt.Println(eventName, "emitted", content)
// 	search.IndexDatainRedis(ctx, content)
// }

func StartIndexingWorker() {
	ctx := context.Background()
	sub := globals.RedisClient.Subscribe(ctx, "indexing-events")
	ch := sub.Channel()

	log.Println("[IndexingWorker] Listening for indexing events...")

	for msg := range ch {
		var event models.Index
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			log.Printf("[IndexingWorker] Failed to parse event: %v", err)
			continue
		}
		log.Printf("[IndexingWorker] Processing event=%+v", event)

		if err := search.IndexDatainRedis(ctx, event); err != nil {
			log.Printf("[IndexingWorker] IndexDatainRedis error: %v", err)
		} else {
			log.Println("[IndexingWorker] Indexing complete")
		}
	}
}

// Printer sends raw JSON to an external endpoint (placeholder).
// Replace with your actual QUIC or HTTP implementation when ready.
/*
func Printer(jsonData []byte) error {
	start := time.Now()
	resp, err := http.Post(SERP_URL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	elapsed := time.Since(start)
	fmt.Printf("Server Response: %s\n", string(body))
	fmt.Printf("Execution Time: %v\n", elapsed)

	return nil
}
*/
