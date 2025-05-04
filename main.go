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
	routes.AddAdsRoutes(router)
	routes.AddArtistRoutes(router)
	routes.AddAuthRoutes(router)
	routes.AddCartoonRoutes(router)
	routes.AddChatRoutes(router)
	routes.AddEventsRoutes(router)
	routes.AddFeedRoutes(router, rateLimiter)
	routes.AddHomeFeedRoutes(router)
	routes.AddItineraryRoutes(router)
	routes.AddMapRoutes(router)
	routes.AddMediaRoutes(router)
	routes.AddMerchRoutes(router)
	routes.AddPlaceRoutes(router)
	routes.AddProfileRoutes(router)
	routes.AddReviewsRoutes(router)
	routes.AddSearchRoutes(router)
	routes.AddSettingsRoutes(router)
	routes.AddStaticRoutes(router)
	routes.AddSuggestionsRoutes(router)
	routes.AddTicketRoutes(router)
	routes.AddUtilityRoutes(router, rateLimiter)
	routes.AddWebsockRoutes(router)

	// CORS setup (adjust AllowedOrigins in production)
	allowedOrigins := []string{"https://zincate.netlify.app"}
	if os.Getenv("ENV") == "development" {
		allowedOrigins = []string{"*"}
	}

	c := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
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
		log.Println("Warning: No .env file found, proceeding with defaults.")
	}

	dbURL := os.Getenv("MONGODB_URI")
	if dbURL == "" {
		log.Fatal("❌ MONGODB_URI is missing in environment variables!")
	}

	err := db.InitMongoDB()
	if err != nil {
		log.Fatalf("❌ MongoDB initialization failed: %v", err)
	}

	// rateLimiter, err := ratelim.NewRateLimiter()
	// if err != nil {
	// 	log.Fatalf("❌ Rate limiter setup failed: %v", err)
	// }

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

	// Register cleanup tasks on shutdown
	server.RegisterOnShutdown(func() {
		log.Println("🛑 Cleaning up resources before shutdown...")
		// Add cleanup tasks like closing DB connection
		db.CloseMongoDB()
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
