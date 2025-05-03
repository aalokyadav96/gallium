package db

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	MapsCollection             *mongo.Collection
	UserCollection             *mongo.Collection
	ItineraryCollection        *mongo.Collection
	UserDataCollection         *mongo.Collection
	TicketsCollection          *mongo.Collection
	PurchasedTicketsCollection *mongo.Collection
	ReviewsCollection          *mongo.Collection
	SettingsCollection         *mongo.Collection
	FollowingsCollection       *mongo.Collection
	PlacesCollection           *mongo.Collection
	SlotCollection             *mongo.Collection
	BookingsCollection         *mongo.Collection
	PostsCollection            *mongo.Collection
	FilesCollection            *mongo.Collection
	MerchCollection            *mongo.Collection
	MenuCollection             *mongo.Collection
	ActivitiesCollection       *mongo.Collection
	EventsCollection           *mongo.Collection
	ArtistEventsCollection     *mongo.Collection
	SongsCollection            *mongo.Collection
	MediaCollection            *mongo.Collection
	ArtistsCollection          *mongo.Collection
	CartoonsCollection         *mongo.Collection
	ChatsCollection            *mongo.Collection
	MessagesCollection         *mongo.Collection
	Client                     *mongo.Client
)

// Initialize MongoDB connection

// Initialize MongoDB connection with proper error handling
func InitMongoDB() error {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		return fmt.Errorf("❌ MONGODB_URI is not set in environment variables")
	}

	ClientOptions := options.Client().ApplyURI(mongoURI)
	var err error

	Client, err = mongo.Connect(context.TODO(), ClientOptions)
	if err != nil {
		return fmt.Errorf("❌ Failed to connect to MongoDB: %v", err)
	}

	// Test connection
	err = Client.Ping(context.TODO(), nil)
	if err != nil {
		return fmt.Errorf("❌ MongoDB ping failed: %v", err)
	}

	// CreateIndexes(Client)
	MapsCollection = Client.Database("eventdb").Collection("maps")
	SettingsCollection = Client.Database("eventdb").Collection("settings")
	ReviewsCollection = Client.Database("eventdb").Collection("reviews")
	FollowingsCollection = Client.Database("eventdb").Collection("followings")
	ItineraryCollection = Client.Database("eventdb").Collection("itinerary")
	UserCollection = Client.Database("eventdb").Collection("users")
	UserDataCollection = Client.Database("eventdb").Collection("userdata")
	TicketsCollection = Client.Database("eventdb").Collection("ticks")
	PurchasedTicketsCollection = Client.Database("eventdb").Collection("purticks")
	PlacesCollection = Client.Database("eventdb").Collection("places")
	BookingsCollection = Client.Database("eventdb").Collection("bookings")
	SlotCollection = Client.Database("eventdb").Collection("slots")
	PostsCollection = Client.Database("eventdb").Collection("posts")
	FilesCollection = Client.Database("eventdb").Collection("files")
	MerchCollection = Client.Database("eventdb").Collection("merch")
	MenuCollection = Client.Database("eventdb").Collection("menu")
	ActivitiesCollection = Client.Database("eventdb").Collection("activities")
	EventsCollection = Client.Database("eventdb").Collection("events")
	ArtistEventsCollection = Client.Database("eventdb").Collection("artistevents")
	SongsCollection = Client.Database("eventdb").Collection("songs")
	MediaCollection = Client.Database("eventdb").Collection("media")
	ArtistsCollection = Client.Database("eventdb").Collection("artists")
	CartoonsCollection = Client.Database("eventdb").Collection("cartoons")
	ChatsCollection = Client.Database("eventdb").Collection("chats")
	MessagesCollection = Client.Database("eventdb").Collection("messages")

	log.Println("✅ Successfully connected to MongoDB")
	return nil
}

// Gracefully close MongoDB connection
func CloseMongoDB() {
	if Client != nil {
		err := Client.Disconnect(context.TODO())
		if err != nil {
			log.Printf("❌ Error closing MongoDB connection: %v", err)
		} else {
			log.Println("✅ MongoDB connection closed successfully")
		}
	}
}
