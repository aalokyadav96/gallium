package db

import (
	"context"
	"log"
	_ "net/http/pprof"

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
	// BusinessesCollection *mongo.Collection
	SlotCollection     *mongo.Collection
	BookingsCollection *mongo.Collection
	// MenusCollection      *mongo.Collection
	// PromotionsCollection *mongo.Collection
	// OwnersCollection     *mongo.Collection
	PostsCollection *mongo.Collection
	FilesCollection *mongo.Collection
	// SeatsCollection      *mongo.Collection
	MerchCollection        *mongo.Collection
	MenuCollection         *mongo.Collection
	ActivitiesCollection   *mongo.Collection
	EventsCollection       *mongo.Collection
	ArtistEventsCollection *mongo.Collection
	SongsCollection        *mongo.Collection
	// GigsCollection       *mongo.Collection
	MediaCollection    *mongo.Collection
	ArtistsCollection  *mongo.Collection
	CartoonsCollection *mongo.Collection
	// BlogCollection       *mongo.Collection
	ChatsCollection    *mongo.Collection
	MessagesCollection *mongo.Collection
	Client             *mongo.Client
)

// Initialize MongoDB connection
func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	ClientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	Client, err = mongo.Connect(context.TODO(), ClientOptions)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
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
	// BusinessesCollection = Client.Database("eventdb").Collection("businesses")
	BookingsCollection = Client.Database("eventdb").Collection("bookings")
	SlotCollection = Client.Database("eventdb").Collection("slots")
	// MenusCollection = Client.Database("eventdb").Collection("menus")
	// PromotionsCollection = Client.Database("eventdb").Collection("promotions")
	// OwnersCollection = Client.Database("eventdb").Collection("owners")
	PostsCollection = Client.Database("eventdb").Collection("posts")
	FilesCollection = Client.Database("eventdb").Collection("files")
	// SeatsCollection = Client.Database("eventdb").Collection("seats")
	MerchCollection = Client.Database("eventdb").Collection("merch")
	MenuCollection = Client.Database("eventdb").Collection("menu")
	ActivitiesCollection = Client.Database("eventdb").Collection("activities")
	EventsCollection = Client.Database("eventdb").Collection("events")
	ArtistEventsCollection = Client.Database("eventdb").Collection("artistevents")
	SongsCollection = Client.Database("eventdb").Collection("songs")
	// GigsCollection = Client.Database("eventdb").Collection("gigs")
	MediaCollection = Client.Database("eventdb").Collection("media")
	ArtistsCollection = Client.Database("eventdb").Collection("artists")
	CartoonsCollection = Client.Database("eventdb").Collection("cartoons")
	// BlogCollection = Client.Database("eventdb").Collection("blogs")
	ChatsCollection = Client.Database("eventdb").Collection("chats")
	MessagesCollection = Client.Database("eventdb").Collection("messages")
}
