package beats

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"naevis/db"
	"naevis/models"
	"naevis/rdx"
	"naevis/utils"
)

// redisLikeKey builds the Redis key for a given entityType/entityID.
func redisLikeKey(entityType, entityID string) string {
	return "like:count:" + entityType + ":" + entityID
}

// ToggleLike handles POST /likes/:entitytype/like/:entityid
func ToggleLike(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	userid := utils.GetUserIDFromRequest(r)
	if userid == "" {
		http.Error(w, "Unauthorized: user not found", http.StatusUnauthorized)
		return
	}

	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")
	filter := bson.M{"user_id": userid, "entity_type": entityType, "entity_id": entityID}
	redisKey := redisLikeKey(entityType, entityID)

	var existing models.Like
	err := db.LikesCollection.FindOne(ctx, filter).Decode(&existing)

	if err == nil {
		// Already liked → remove
		_, _ = db.LikesCollection.DeleteOne(ctx, filter)
		count := decrementRedisOrMongo(ctx, redisKey, entityType, entityID)
		utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{"liked": false, "count": count})
		return
	}

	// Not yet liked → insert
	like := models.Like{UserID: userid, EntityType: entityType, EntityID: entityID, CreatedAt: time.Now()}
	_, err = db.LikesCollection.InsertOne(ctx, like)
	if err != nil {
		http.Error(w, "Failed to like", http.StatusInternalServerError)
		return
	}
	count := incrementRedisOrMongo(ctx, redisKey, entityType, entityID)
	utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{"liked": true, "count": count})
}

// Helper functions to handle Redis fallback
func decrementRedisOrMongo(ctx context.Context, redisKey, entityType, entityID string) int64 {
	if val, err := rdx.Conn.Decr(ctx, redisKey).Result(); err == nil {
		return val
	}
	mongoCount, _ := db.LikesCollection.CountDocuments(ctx, bson.M{"entity_type": entityType, "entity_id": entityID})
	return mongoCount
}

func incrementRedisOrMongo(ctx context.Context, redisKey, entityType, entityID string) int64 {
	if val, err := rdx.Conn.Incr(ctx, redisKey).Result(); err == nil {
		return val
	}
	mongoCount, _ := db.LikesCollection.CountDocuments(ctx, bson.M{"entity_type": entityType, "entity_id": entityID})
	return mongoCount
}

// BatchUserLikes handles POST /likes/:entitytype/batch/users
func BatchUserLikes(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userID := utils.GetUserIDFromRequest(r)
	if userID == "" {
		http.Error(w, "Unauthorized: user not found", http.StatusUnauthorized)
		return
	}

	var req struct {
		EntityIDs []string `json:"entity_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if len(req.EntityIDs) == 0 {
		utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{"data": map[string]bool{}})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result := make(map[string]bool, len(req.EntityIDs))

	cursor, err := db.LikesCollection.Find(ctx, bson.M{
		"user_id":   userID,
		"entity_id": bson.M{"$in": req.EntityIDs},
	})
	if err != nil {
		http.Error(w, "Failed to query likes", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	likedSet := make(map[string]struct{})
	for cursor.Next(ctx) {
		var like models.Like
		if err := cursor.Decode(&like); err != nil {
			continue
		}
		likedSet[like.EntityID] = struct{}{}
	}

	for _, eid := range req.EntityIDs {
		_, liked := likedSet[eid]
		result[eid] = liked
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{"data": result})
}

// GetLikers handles GET /likes/:entitytype/users/:entityid
func GetLikers(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userid := utils.GetUserIDFromRequest(r)
	if userid == "" {
		http.Error(w, "Unauthorized: user not found", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")
	if entityType == "" || entityID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	cursor, err := db.LikesCollection.Find(ctx, bson.M{
		"entity_type": entityType,
		"entity_id":   entityID,
	})
	if err != nil {
		http.Error(w, "Failed to fetch likers", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	likers := []map[string]string{}
	for cursor.Next(ctx) {
		var like models.Like
		if err := cursor.Decode(&like); err != nil {
			continue
		}

		var userMeta struct {
			UserID         string `bson:"userid"`
			Username       string `bson:"username"`
			ProfilePicture string `bson:"profile_picture,omitempty"`
		}

		err := db.UserCollection.FindOne(ctx, bson.M{"userid": like.UserID}).Decode(&userMeta)
		if err != nil && err != mongo.ErrNoDocuments {
			continue
		}

		likers = append(likers, map[string]string{
			"userid":          userMeta.UserID,
			"username":        userMeta.Username,
			"profile_picture": userMeta.ProfilePicture,
		})
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"likers": likers,
	})
}

// GetLikeCount handles GET /likes/:entitytype/count/:entityid
func GetLikeCount(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userid := utils.GetUserIDFromRequest(r)
	if userid == "" {
		http.Error(w, "Unauthorized: user not found", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")
	redisKey := redisLikeKey(entityType, entityID)

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

	filter := bson.M{"entity_type": entityType, "entity_id": entityID}
	count, err := db.LikesCollection.CountDocuments(ctx, filter)
	if err != nil {
		http.Error(w, "Count failed", http.StatusInternalServerError)
		return
	}

	if err := rdx.Conn.Set(ctx, redisKey, count, 10*time.Minute).Err(); err != nil {
		log.Printf("Redis Set error: %v", err)
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]int64{"count": count})
}
