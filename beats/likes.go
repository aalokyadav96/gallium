package beats

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"

	"naevis/db"
	"naevis/middleware"
	"naevis/models"
	"naevis/rdx"
	"naevis/utils"
)

// redisLikeKey builds the Redis key for a given entityType/entityID.
func redisLikeKey(entityType, entityID string) string {
	return "like:count:" + entityType + ":" + entityID
}

// ToggleLike handles POST /likes/:entitytype/:entityid
// It toggles (adds or removes) a “Like” and returns { liked: bool, count: <newCount> }.
func ToggleLike(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Use a short‐timeout context so we don’t hang on Redis/Mongo calls indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")
	token := r.Header.Get("Authorization")

	// 1) Validate JWT
	claims, err := middleware.ValidateJWT(token)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2) Build a filter by user + entity
	filter := bson.M{
		"user_id":     claims.UserID,
		"entity_type": entityType,
		"entity_id":   entityID,
	}

	// 3) Check if the user already liked this entity
	var existing models.Like
	err = db.LikesCollection.FindOne(ctx, filter).Decode(&existing)
	redisKey := redisLikeKey(entityType, entityID)

	if err == nil {
		// → Already liked: remove (unlike)
		_, _ = db.LikesCollection.DeleteOne(ctx, filter)

		// Try to decrement in Redis. If Redis is down or key does not exist, we'll catch that below.
		newCount, rErr := rdx.Conn.Decr(ctx, redisKey).Result()
		if rErr != nil {
			// On Redis error, fallback to counting in Mongo directly
			mongoCount, mErr := db.LikesCollection.CountDocuments(ctx, bson.M{
				"entity_type": entityType,
				"entity_id":   entityID,
			})
			if mErr != nil {
				http.Error(w, "Failed to calculate like count", http.StatusInternalServerError)
				return
			}
			utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
				"liked": false,
				"count": mongoCount,
			})
			return
		}

		// Redis succeeded
		utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
			"liked": false,
			"count": newCount,
		})
		return
	}

	// err != nil means “no document found” → user has not yet liked
	like := models.Like{
		UserID:     claims.UserID,
		EntityType: entityType,
		EntityID:   entityID,
		CreatedAt:  time.Now(),
	}

	_, err = db.LikesCollection.InsertOne(ctx, like)
	if err != nil {
		http.Error(w, "Failed to like", http.StatusInternalServerError)
		return
	}

	// Increment Redis. If Redis fails, we’ll fall back to Mongo below.
	newCount, rErr := rdx.Conn.Incr(ctx, redisKey).Result()
	if rErr != nil {
		mongoCount, mErr := db.LikesCollection.CountDocuments(ctx, bson.M{
			"entity_type": entityType,
			"entity_id":   entityID,
		})
		if mErr != nil {
			http.Error(w, "Failed to calculate like count", http.StatusInternalServerError)
			return
		}
		utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
			"liked": true,
			"count": mongoCount,
		})
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"liked": true,
		"count": newCount,
	})
}

func GetLikeCount(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")
	redisKey := redisLikeKey(entityType, entityID)

	// Try Redis first
	countStr, err := rdx.Conn.Get(ctx, redisKey).Result()
	if err == nil {
		if count, parseErr := strconv.ParseInt(countStr, 10, 64); parseErr == nil {
			utils.RespondWithJSON(w, http.StatusOK, map[string]int64{"count": count})
			return
		}
	}
	if err != nil && err != redis.Nil {
		log.Printf("Redis Get error: %v", err)
	}

	// Fallback to MongoDB
	filter := bson.M{"entity_type": entityType, "entity_id": entityID}
	count, err := db.LikesCollection.CountDocuments(ctx, filter)
	if err != nil {
		http.Error(w, "Count failed", http.StatusInternalServerError)
		return
	}

	// Cache in Redis for future requests
	if err := rdx.Conn.Set(ctx, redisKey, count, 10*time.Minute).Err(); err != nil {
		log.Printf("Redis Set error: %v", err)
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]int64{"count": count})
}
