package feed

import (
	"context"
	"encoding/json"
	"errors"
	"naevis/db"
	"naevis/rdx"
	"naevis/structs"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetPosts(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var posts []structs.Post

	filter := bson.M{}
	sortOrder := bson.D{{Key: "timestamp", Value: -1}}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := db.PostsCollection.Find(ctx, filter, &options.FindOptions{
		Sort: sortOrder,
	})
	if err != nil {
		http.Error(w, "Failed to fetch posts", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	userIDsSet := make(map[string]struct{})
	for cursor.Next(ctx) {
		var post structs.Post
		if err := cursor.Decode(&post); err != nil {
			http.Error(w, "Failed to decode post", http.StatusInternalServerError)
			return
		}
		posts = append(posts, post)
		userIDsSet[post.UserID] = struct{}{}
	}

	if err := cursor.Err(); err != nil {
		http.Error(w, "Cursor error", http.StatusInternalServerError)
		return
	}

	if len(posts) == 0 {
		posts = []structs.Post{}
	}

	var userIDs []string
	for id := range userIDsSet {
		userIDs = append(userIDs, id)
	}

	// Fetch usernames from Redis using HGET
	usernameMap := make(map[string]string)
	if len(userIDs) > 0 {
		pipe := rdx.Conn.Pipeline()
		cmds := make([]*redis.StringCmd, len(userIDs))
		for i, id := range userIDs {
			cmds[i] = pipe.HGet(ctx, "users", id) // assumes "users" hash contains userid -> username
		}
		_, err := pipe.Exec(ctx)
		if err != nil && !errors.Is(err, redis.Nil) {
			http.Error(w, "Failed to fetch usernames", http.StatusInternalServerError)
			return
		}
		for i, cmd := range cmds {
			if username, err := cmd.Result(); err == nil {
				usernameMap[userIDs[i]] = username
			} else {
				usernameMap[userIDs[i]] = "unknown"
			}
		}
	}

	// Hybrid strategy: prefer Redis username, fallback to stored
	for i := range posts {
		redisUsername := usernameMap[posts[i].UserID]
		if redisUsername != "" && redisUsername != "unknown" {
			posts[i].Username = redisUsername
		} else if posts[i].Username == "" {
			posts[i].Username = "unknown"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"ok":   true,
		"data": posts,
	})
}

func GetPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.ByName("postid")

	// Aggregation pipeline to fetch post along with related tickets, media, and merch
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "postid", Value: id}}}},
	}

	// Execute the aggregation query
	cursor, err := db.PostsCollection.Aggregate(context.TODO(), pipeline)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	var post structs.Post
	if cursor.Next(context.TODO()) {
		if err := cursor.Decode(&post); err != nil {
			http.Error(w, "Failed to decode post data", http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	// Encode the post as JSON and write to response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(post); err != nil {
		http.Error(w, "Failed to encode post data", http.StatusInternalServerError)
	}
}
