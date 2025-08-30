package auth

import (
	"context"
	"naevis/rdx"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
)

// Logout handles POST /auth/logout
func Logout(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Extract token or session ID from request (e.g., header or cookie)
	token := r.Header.Get("Authorization")
	if token == "" {
		http.Error(w, "No token provided", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Remove token from Redis (or wherever you store active sessions)
	redisKey := "auth:token:" + token
	_, err := rdx.Conn.Del(ctx, redisKey).Result()
	if err != nil {
		http.Error(w, "Failed to invalidate session", http.StatusInternalServerError)
		return
	}

	// Optionally clear cookies if you set a session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1, // expires immediately
		HttpOnly: true,
		Secure:   true,
	})

	// Respond with success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true,"message":"Logged out successfully"}`))
}
