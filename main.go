package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"naevis/middleware"
	"naevis/mq"
	"naevis/newchat"
	"naevis/ratelim"
	"naevis/routes"

	"github.com/joho/godotenv"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
)

// Index is a simple health check handler.
func Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	fmt.Fprint(w, "200")
}

// setupRouter builds the router with all API routes except chat.
func setupRouter(rateLimiter *ratelim.RateLimiter) *httprouter.Router {
	router := httprouter.New()
	router.GET("/health", Index)
	routes.RoutesWrapper(router, rateLimiter)
	return router
}

func parseAllowedOrigins(env string) []string {
	if env == "" {
		return []string{"http://localhost:5173", "https://indium.netlify.app"}
	}
	parts := strings.Split(env, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// startStaticServer runs a separate HTTP server for public static files.
func startStaticServer() {
	staticRouter := httprouter.New()
	routes.AddStaticRoutes(staticRouter)

	staticServer := &http.Server{
		Addr:              ":4001",
		Handler:           staticRouter,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}

	go func() {
		log.Println("?? Static server running on http://localhost:4001")
		if err := staticServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Static server error: %v", err)
		}
	}()

	// graceful shutdown for static server
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		staticServer.Shutdown(ctx)
		log.Println("? Static server stopped")
	}()
}

func main() {
	// load .env if present
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found; using system environment")
	}

	// read port
	port := os.Getenv("PORT")
	if port == "" {
		port = ":4000"
	} else if port[0] != ':' {
		port = ":" + port
	}

	// parse allowed origins
	allowedOrigins := parseAllowedOrigins(os.Getenv("ALLOWED_ORIGINS"))

	// initialize rate limiter
	rateLimiter := ratelim.NewRateLimiter(1, 6, 10*time.Minute, 10000)

	// initialize chat hub
	hub := newchat.NewHub()
	go hub.Run()

	// build API router and add chat routes
	router := setupRouter(rateLimiter)
	routes.AddNewChatRoutes(router, hub, rateLimiter)

	// Middleware chain: Logging + SecurityHeaders -> router
	innerHandler := middleware.LoggingMiddleware(middleware.SecurityHeaders(router))

	// CORS must be applied outermost when using credentials
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"HEAD", "GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "Idempotency-Key", "X-Requested-With"},
		AllowCredentials: true,
	}).Handler(innerHandler)

	// create API HTTP server
	server := &http.Server{
		Addr:              port,
		Handler:           corsHandler,
		ReadTimeout:       7 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}

	// start workers
	go mq.StartIndexingWorker()
	go mq.StartHashtagWorker()

	// start static server
	startStaticServer()

	// graceful shutdown for API server
	server.RegisterOnShutdown(func() {
		log.Println("Shutting down chat hub...")
		hub.Stop()
	})

	// start API server
	go func() {
		log.Printf("API server listening on %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("? API ListenAndServe error: %v", err)
		}
	}()

	// wait for shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutdown signal received; shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Graceful shutdown failed: %v", err)
	}

	log.Println("API server stopped cleanly")
}
