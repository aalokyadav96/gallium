package main

import (
	"context"
	"fmt"
	"log"
	"naevis/db"
	"naevis/ratelim"
	"naevis/routes"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Middleware: Security headers
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "max-age=0, no-cache, no-store, must-revalidate, private")
		next.ServeHTTP(w, r)
	})
}

// Middleware: Simple request logging
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s %s", r.Method, r.RequestURI, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

// Health check endpoint
func Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	fmt.Fprint(w, "200")
}

// Set up all routes and middleware layers
func setupRouter(rateLimiter *ratelim.RateLimiter) http.Handler {
	router := httprouter.New()
	router.GET("/health", Index)

	routes.AddActivityRoutes(router)
	routes.AddAuthRoutes(router)
	routes.AddEventsRoutes(router)
	routes.AddMerchRoutes(router)
	routes.AddTicketRoutes(router)
	routes.AddSuggestionsRoutes(router)
	routes.AddReviewsRoutes(router)
	routes.AddMediaRoutes(router)
	routes.AddPlaceRoutes(router)
	routes.AddProfileRoutes(router)
	routes.AddArtistRoutes(router)
	routes.AddCartoonRoutes(router)
	routes.AddMapRoutes(router)
	routes.AddItineraryRoutes(router)
	routes.AddFeedRoutes(router, rateLimiter)
	routes.AddSettingsRoutes(router)
	routes.AddAdsRoutes(router)
	routes.AddHomeFeedRoutes(router)
	routes.AddSearchRoutes(router)
	routes.AddStaticRoutes(router)

	// CORS setup (adjust AllowedOrigins in production)
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"}, // Consider specific origins in production
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})

	// Wrap handlers with middleware: CORS -> Security -> Logging -> Router
	return loggingMiddleware(securityHeaders(c.Handler(router)))
}

func main() {
	// Load .env if present
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found. Continuing with system environment variables.")
	}

	rateLimiter := ratelim.NewRateLimiter()
	handler := setupRouter(rateLimiter)

	server := &http.Server{
		Addr:              ":4000",
		Handler:           handler,
		ReadTimeout:       7 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}

	setDBCollection()

	// Register cleanup tasks on shutdown
	server.RegisterOnShutdown(func() {
		log.Println("🛑 Cleaning up resources before shutdown...")
		// Add cleanup tasks like closing DB connection
	})

	// Start server in a goroutine
	go func() {
		log.Println("🚀 Server started on port 4000")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Could not listen on port 4000: %v", err)
		}
	}()

	// Graceful shutdown on interrupt
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)
	<-shutdownChan

	log.Println("🛑 Shutdown signal received. Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("❌ Server shutdown failed: %v", err)
	}

	log.Println("✅ Server stopped cleanly")
}

func setDBCollection() {

	// Initialize MongoDB connection
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	ClientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	db.Client, err = mongo.Connect(context.TODO(), ClientOptions)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	// CreateIndexes(Client)
	db.MapsCollection = db.Client.Database("eventdb").Collection("maps")
	db.SettingsCollection = db.Client.Database("eventdb").Collection("settings")
	db.ReviewsCollection = db.Client.Database("eventdb").Collection("reviews")
	db.FollowingsCollection = db.Client.Database("eventdb").Collection("followings")
	db.ItineraryCollection = db.Client.Database("eventdb").Collection("itinerary")
	db.UserCollection = db.Client.Database("eventdb").Collection("users")
	db.UserDataCollection = db.Client.Database("eventdb").Collection("userdata")
	db.TicketsCollection = db.Client.Database("eventdb").Collection("ticks")
	db.PurchasedTicketsCollection = db.Client.Database("eventdb").Collection("purticks")
	db.PlacesCollection = db.Client.Database("eventdb").Collection("places")
	db.BookingsCollection = db.Client.Database("eventdb").Collection("bookings")
	db.SlotCollection = db.Client.Database("eventdb").Collection("slots")
	db.PostsCollection = db.Client.Database("eventdb").Collection("posts")
	db.FilesCollection = db.Client.Database("eventdb").Collection("files")
	db.MerchCollection = db.Client.Database("eventdb").Collection("merch")
	db.MenuCollection = db.Client.Database("eventdb").Collection("menu")
	db.ActivitiesCollection = db.Client.Database("eventdb").Collection("activities")
	db.EventsCollection = db.Client.Database("eventdb").Collection("events")
	db.ArtistEventsCollection = db.Client.Database("eventdb").Collection("artistevents")
	db.SongsCollection = db.Client.Database("eventdb").Collection("songs")
	db.MediaCollection = db.Client.Database("eventdb").Collection("media")
	db.ArtistsCollection = db.Client.Database("eventdb").Collection("artists")
	db.CartoonsCollection = db.Client.Database("eventdb").Collection("cartoons")
	db.ChatsCollection = db.Client.Database("eventdb").Collection("chats")
	db.MessagesCollection = db.Client.Database("eventdb").Collection("messages")
}

// package main

// import (
// 	"context"
// 	"fmt"
// 	"log"
// 	"naevis/ratelim"
// 	"naevis/routes"
// 	"net/http"
// 	"os"
// 	"os/signal"
// 	"syscall"

// 	"github.com/joho/godotenv"
// 	"github.com/julienschmidt/httprouter"
// 	"github.com/rs/cors"
// )

// // Security headers middleware
// func securityHeaders(next http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		// Set HTTP headers for enhanced security
// 		w.Header().Set("X-XSS-Protection", "1; mode=block")
// 		w.Header().Set("X-Content-Type-Options", "nosniff")
// 		w.Header().Set("X-Frame-Options", "DENY")
// 		w.Header().Set("Cache-Control", "max-age=0, no-cache, no-store, must-revalidate, private")
// 		next.ServeHTTP(w, r) // Call the next handler
// 	})
// }

// func main() {
// 	err := godotenv.Load()
// 	if err != nil {
// 		log.Fatal("Error loading .env file")
// 	}

// 	router := httprouter.New()

// 	rateLimiter := ratelim.NewRateLimiter()

// 	router.GET("/health", Index)

// 	routes.AddActivityRoutes(router)
// 	routes.AddAuthRoutes(router)
// 	routes.AddEventsRoutes(router)
// 	routes.AddMerchRoutes(router)
// 	routes.AddTicketRoutes(router)
// 	routes.AddSuggestionsRoutes(router)
// 	routes.AddReviewsRoutes(router)
// 	routes.AddMediaRoutes(router)
// 	routes.AddPlaceRoutes(router)
// 	routes.AddProfileRoutes(router)
// 	routes.AddArtistRoutes(router)
// 	routes.AddMapRoutes(router)
// 	routes.AddItineraryRoutes(router)
// 	routes.AddFeedRoutes(router, rateLimiter)
// 	routes.AddSettingsRoutes(router)
// 	routes.AddAdsRoutes(router)
// 	routes.AddHomeFeedRoutes(router)
// 	routes.AddSearchRoutes(router)

// 	// CORS setup
// 	c := cors.New(cors.Options{
// 		AllowedOrigins:   []string{"*"},
// 		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
// 		AllowedHeaders:   []string{"Content-Type", "Authorization"},
// 		AllowCredentials: true,
// 	})

// 	handler := securityHeaders(c.Handler(router))
// 	routes.AddStaticRoutes(router)

// 	server := &http.Server{
// 		Addr:    ":4000",
// 		Handler: handler, // Use the middleware-wrapped handler
// 	}

// 	// Start server in a goroutine to handle graceful shutdown
// 	go func() {
// 		log.Println("Server started on port 4000")
// 		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
// 			log.Fatalf("Could not listen on port 4000: %v", err)
// 		}
// 	}()

// 	// Graceful shutdown listener
// 	shutdownChan := make(chan os.Signal, 1)
// 	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

// 	// Wait for termination signal
// 	<-shutdownChan
// 	log.Println("Shutting down gracefully...")

// 	// Attempt to gracefully shut down the server
// 	if err := server.Shutdown(context.Background()); err != nil {
// 		log.Fatalf("Server shutdown failed: %v", err)
// 	}
// 	log.Println("Server stopped")
// }

// func Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	fmt.Fprint(w, "200")
// }
