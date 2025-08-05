package db

import (
	"context"
	"log"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	Client *mongo.Client
	// Your collections:
	AnalyticsCollection         *mongo.Collection
	MapsCollection              *mongo.Collection
	CartCollection              *mongo.Collection
	OrderCollection             *mongo.Collection
	CatalogueCollection         *mongo.Collection
	FarmsCollection             *mongo.Collection
	FarmOrdersCollection        *mongo.Collection
	CropsCollection             *mongo.Collection
	CommentsCollection          *mongo.Collection
	RoomsCollection             *mongo.Collection
	UserCollection              *mongo.Collection
	LikesCollection             *mongo.Collection
	ProductCollection           *mongo.Collection
	ItineraryCollection         *mongo.Collection
	UserDataCollection          *mongo.Collection
	TicketsCollection           *mongo.Collection
	BehindTheScenesCollection   *mongo.Collection
	PurchasedTicketsCollection  *mongo.Collection
	ReviewsCollection           *mongo.Collection
	SettingsCollection          *mongo.Collection
	FollowingsCollection        *mongo.Collection
	PlacesCollection            *mongo.Collection
	SlotCollection              *mongo.Collection
	ConfigsCollection           *mongo.Collection
	BookingsCollection          *mongo.Collection
	PostsCollection             *mongo.Collection
	BlogPostsCollection         *mongo.Collection
	FilesCollection             *mongo.Collection
	MerchCollection             *mongo.Collection
	MenuCollection              *mongo.Collection
	ActivitiesCollection        *mongo.Collection
	EventsCollection            *mongo.Collection
	ArtistEventsCollection      *mongo.Collection
	SongsCollection             *mongo.Collection
	MediaCollection             *mongo.Collection
	ArtistsCollection           *mongo.Collection
	ChatsCollection             *mongo.Collection
	MessagesCollection          *mongo.Collection
	QuestionCollection          *mongo.Collection
	AnswerCollection            *mongo.Collection
	ReportsCollection           *mongo.Collection
	RecipeCollection            *mongo.Collection
	BaitoCollection             *mongo.Collection
	BaitoApplicationsCollection *mongo.Collection
	BaitoWorkerCollection       *mongo.Collection
)

// limiter chan to cap concurrent Mongo ops
var mongoLimiter = make(chan struct{}, 100) // allow up to 100 concurrent ops

func init() {
	_ = godotenv.Load()

	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		log.Fatal("❌ MONGODB_URI environment variable not set")
	}

	clientOpts := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(100).
		SetMinPoolSize(10).
		SetRetryWrites(true)

	var err error
	Client, err = mongo.Connect(context.Background(), clientOpts)
	if err != nil {
		log.Fatalf("❌ Failed to connect to MongoDB: %v", err)
	}
	if err := Client.Ping(context.Background(), nil); err != nil {
		log.Fatalf("❌ Mongo ping failed: %v", err)
	}

	log.Printf("✅ MongoDB connected (%s) maxPool=%d minPool=%d; Goroutines at start: %d",
		uri, *clientOpts.MaxPoolSize, *clientOpts.MinPoolSize, runtime.NumGoroutine(),
	)

	// Graceful shutdown hook
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		log.Println("🛑 Disconnecting from MongoDB...")
		_ = Client.Disconnect(context.Background())
		os.Exit(0)
	}()

	// Optional: log connection stats periodically
	go logPoolStats()

	// Initialize your collections
	db := Client.Database("eventdb")
	ActivitiesCollection = db.Collection("activities")
	AnalyticsCollection = db.Collection("analytics")
	ArtistEventsCollection = db.Collection("artistevents")
	ArtistsCollection = db.Collection("artists")
	BaitoCollection = db.Collection("baito")
	BaitoApplicationsCollection = db.Collection("baitoapply")
	BaitoWorkerCollection = db.Collection("baitoworkers")
	BlogPostsCollection = db.Collection("bposts")
	BookingsCollection = db.Collection("bookings")
	BehindTheScenesCollection = db.Collection("bts")
	CartCollection = db.Collection("cart")
	CatalogueCollection = db.Collection("catalogue")
	ChatsCollection = db.Collection("chats")
	CommentsCollection = db.Collection("comments")
	ConfigsCollection = db.Collection("configs")
	CropsCollection = db.Collection("crops")
	EventsCollection = db.Collection("events")
	FarmsCollection = db.Collection("farms")
	FilesCollection = db.Collection("files")
	FollowingsCollection = db.Collection("followings")
	FarmOrdersCollection = db.Collection("forders")
	ItineraryCollection = db.Collection("itinerary")
	LikesCollection = db.Collection("likes")
	MapsCollection = db.Collection("maps")
	MediaCollection = db.Collection("media")
	MenuCollection = db.Collection("menu")
	MerchCollection = db.Collection("merch")
	MessagesCollection = db.Collection("messages")
	OrderCollection = db.Collection("orders")
	PlacesCollection = db.Collection("places")
	PostsCollection = db.Collection("posts")
	ProductCollection = db.Collection("products")
	PurchasedTicketsCollection = db.Collection("purticks")
	RecipeCollection = db.Collection("recipes")
	QuestionCollection = db.Collection("questions")
	AnswerCollection = db.Collection("answers")
	ReportsCollection = db.Collection("reports")
	ReviewsCollection = db.Collection("reviews")
	RoomsCollection = db.Collection("rooms")
	SettingsCollection = db.Collection("settings")
	SlotCollection = db.Collection("slots")
	SongsCollection = db.Collection("songs")
	TicketsCollection = db.Collection("ticks")
	UserDataCollection = db.Collection("userdata")
	UserCollection = db.Collection("users")
}

// logPoolStats logs basic goroutine and pool stats every 60s (optional)
func logPoolStats() {
	for {
		time.Sleep(60 * time.Second)
		log.Printf("📊 Mongo Stats: Goroutines=%d | MongoOpsRunning=%d", runtime.NumGoroutine(), len(mongoLimiter))
	}
}

// PingMongo can be used in your /health endpoint
func PingMongo() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return Client.Ping(ctx, nil)
}

// WithMongo wraps any Mongo operation with concurrency and timeout + minimal retry
func WithMongo(op func(ctx context.Context) error) error {
	mongoLimiter <- struct{}{}        // acquire slot
	defer func() { <-mongoLimiter }() // release slot

	var err error
	for i := 0; i < 2; i++ { // 1 retry max
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = op(ctx)
		if err == nil {
			return nil
		}
		log.Printf("⚠️ Mongo op failed: %v (retry %d)", err, i+1)
		time.Sleep(200 * time.Millisecond)
	}
	return err
}

// OptionsFindLatest provides a find option with latest sort
func OptionsFindLatest(limit int64) *options.FindOptions {
	opts := options.Find()
	opts.SetSort(map[string]interface{}{"createdAt": -1})
	opts.SetLimit(limit)
	return opts
}
