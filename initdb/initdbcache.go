package initdb

import (
	"context"
	"naevis/db"
	"naevis/rdx"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
)

// Package-level context.
var CTX = context.Background()

var RedisClient *redis.Client = rdx.Conn
var MongoClient *mongo.Client = db.Client

// // init initializes Redis and MongoDB clients.
// func init() {
// 	errx := godotenv.Load()
// 	if errx != nil {
// 		log.Fatal("Error loading .env file")
// 	}

// 	var redis_url = os.Getenv("REDIS_URL")
// 	var mongo_url = os.Getenv("MONGODB_URI")
// 	// Initialize Redis client.
// 	RedisClient = redis.NewClient(&redis.Options{
// 		Addr:     redis_url,
// 		Password: os.Getenv("REDIS_PASSWORD"), // no password set
// 		DB:       0,                           // use default DB
// 	})
// 	if _, err := RedisClient.Ping(CTX).Result(); err != nil {
// 		log.Fatalf("Failed to connect to Redis: %v", err)
// 	}

// 	// Initialize MongoDB client.
// 	var err error
// 	MongoClient, err = mongo.Connect(CTX, options.Client().ApplyURI(mongo_url))
// 	if err != nil {
// 		log.Fatalf("Failed to connect to MongoDB: %v", err)
// 	}
// 	if err = MongoClient.Ping(CTX, nil); err != nil {
// 		log.Fatalf("MongoDB ping failed: %v", err)
// 	}
// }
