package rdx

import (
	"encoding/json"
	"log"
	"naevis/db"
	"naevis/globals"
	"naevis/models"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Flush messages from Redis to MongoDB in bulk.
func FlushRedisMessages() {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		// Get all keys matching chat:*:messages.
		keys, err := Conn.Keys(globals.Ctx, "chat:*:messages").Result()
		if err != nil {
			log.Println("Redis scan error:", err)
			continue
		}
		for _, key := range keys {
			// Retrieve all messages from Redis.
			msgs, err := Conn.LRange(globals.Ctx, key, 0, -1).Result()
			if err != nil {
				log.Println("Redis LRange error:", err)
				continue
			}
			if len(msgs) == 0 {
				continue
			}
			var messagesBulk []interface{}
			for _, mStr := range msgs {
				var m models.Message
				if err := json.Unmarshal([]byte(mStr), &m); err != nil {
					log.Println("JSON unmarshal error:", err)
					continue
				}
				messagesBulk = append(messagesBulk, m)
			}
			if len(messagesBulk) > 0 {
				_, err := db.MessagesCollection.InsertMany(globals.Ctx, messagesBulk)
				if err != nil {
					log.Println("MongoDB InsertMany error:", err)
					continue
				}
				// Remove the key from Redis after successful insertion.
				Conn.Del(globals.Ctx, key)
			}
		}
	}
}

func FlushRedisLikes() {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		keys, err := Conn.Keys(globals.Ctx, "like:count:*:*").Result()
		if err != nil {
			log.Println("Redis scan error:", err)
			continue
		}

		for _, key := range keys {
			parts := strings.Split(key, ":")
			if len(parts) != 4 {
				log.Println("Invalid Redis like key format:", key)
				continue
			}
			entityType := parts[2]
			entityID := parts[3]

			// Check TTL to determine if the key is stale
			ttl, err := Conn.TTL(globals.Ctx, key).Result()
			if err != nil {
				log.Println("Redis TTL error for key", key, ":", err)
				continue
			}

			// Only flush if TTL is less than threshold (e.g. < 10 seconds)
			if ttl > 0 && ttl > 10*time.Second {
				continue // skip fresh keys
			}

			// Get the like count
			countStr, err := Conn.Get(globals.Ctx, key).Result()
			if err != nil {
				log.Println("Redis Get error for key", key, ":", err)
				continue
			}

			count, err := strconv.ParseInt(countStr, 10, 64)
			if err != nil {
				log.Println("Failed to parse like count:", countStr)
				continue
			}

			// Update MongoDB
			filter := bson.M{"_id": entityID}
			update := bson.M{"$set": bson.M{"likes": count}}

			var targetCollection *mongo.Collection
			switch entityType {
			case "post":
				targetCollection = db.PostsCollection
			case "comment":
				targetCollection = db.CommentsCollection
			case "media":
				targetCollection = db.MediaCollection
			default:
				log.Println("Unknown entity type:", entityType)
				continue
			}

			_, err = targetCollection.UpdateOne(globals.Ctx, filter, update)
			if err != nil {
				log.Println("MongoDB update error for", entityType, entityID, ":", err)
				continue
			}

			// Optionally delete or reset TTL
			if err := Conn.Del(globals.Ctx, key).Err(); err != nil {
				log.Println("Failed to delete Redis key:", key)
			}
		}
	}
}
