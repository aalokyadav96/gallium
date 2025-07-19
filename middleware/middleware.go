package middleware

import (
	"context"
	"fmt"
	"naevis/globals"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
)

// JWT claims
type Claims struct {
	Username string   `json:"username"`
	UserID   string   `json:"userId"`
	Role     []string `json:"role"`
	jwt.RegisteredClaims
}

func Authenticate(next httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if websocket.IsWebSocketUpgrade(r) {
			// Allow WebSocket through without setting body/headers yet
			next(w, r, ps)
			return
		}

		tokenString := r.Header.Get("Authorization")
		if tokenString == "" {
			http.Error(w, "Missing token", http.StatusUnauthorized)
			return
		}

		if len(tokenString) < 8 || tokenString[:7] != "Bearer " {
			http.Error(w, "Invalid token format", http.StatusUnauthorized)
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString[7:], claims, func(token *jwt.Token) (any, error) {
			return globals.JwtSecret, nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Store UserID in context
		ctx := context.WithValue(r.Context(), globals.UserIDKey, claims.UserID)
		// Pass updated context to the next handler
		next(w, r.WithContext(ctx), ps)
	}
}

func OptionalAuth(next httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		tokenString := r.Header.Get("Authorization")
		if len(tokenString) >= 8 && tokenString[:7] == "Bearer " {
			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenString[7:], claims, func(token *jwt.Token) (any, error) {
				return globals.JwtSecret, nil
			})
			if err == nil && token.Valid {
				// Add user ID to context if token is valid
				r = r.WithContext(context.WithValue(r.Context(), globals.UserIDKey, claims.UserID))
			}
		}
		// Proceed regardless of token state
		next(w, r, ps)
	}
}

func ValidateJWT(tokenString string) (*Claims, error) {
	if tokenString == "" || len(tokenString) < 8 {
		return nil, fmt.Errorf("invalid token")
	}

	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString[7:], claims, func(token *jwt.Token) (any, error) {
		return globals.JwtSecret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("unauthorized: %w", err)
	}
	return claims, nil
}

// package middleware

// import (
// 	"context"
// 	"fmt"
// 	"naevis/globals"
// 	"net/http"

// 	"github.com/golang-jwt/jwt/v5"
// 	"github.com/julienschmidt/httprouter"
// )

// // JWT claims
// type Claims struct {
// 	Username string `json:"username"`
// 	UserID   string `json:"userId"`
// 	Role     string `json:"role"`
// 	jwt.RegisteredClaims
// }

// func Authenticate(next httprouter.Handle) httprouter.Handle {
// 	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 		tokenString := r.Header.Get("Authorization")
// 		if tokenString == "" {
// 			http.Error(w, "Missing token", http.StatusUnauthorized)
// 			return
// 		}

// 		if len(tokenString) < 7 || tokenString[:7] != "Bearer " {
// 			http.Error(w, "Invalid token format", http.StatusUnauthorized)
// 			return
// 		}

// 		claims := &Claims{}
// 		token, err := jwt.ParseWithClaims(tokenString[7:], claims, func(token *jwt.Token) (any, error) {
// 			return globals.JwtSecret, nil
// 		})
// 		if err != nil || !token.Valid {
// 			http.Error(w, "Invalid token", http.StatusUnauthorized)
// 			return
// 		}

// 		// Store UserID in context
// 		ctx := context.WithValue(r.Context(), globals.UserIDKey, claims.UserID)
// 		// Pass updated context to the next handler
// 		next(w, r.WithContext(ctx), ps)
// 	}
// }

// func ValidateJWT(tokenString string) (*Claims, error) {
// 	if tokenString == "" || len(tokenString) < 8 {
// 		return nil, fmt.Errorf("invalid token")
// 	}

// 	claims := &Claims{}
// 	_, err := jwt.ParseWithClaims(tokenString[7:], claims, func(token *jwt.Token) (any, error) {
// 		return globals.JwtSecret, nil
// 	})
// 	if err != nil {
// 		return nil, fmt.Errorf("unauthorized: %w", err)
// 	}
// 	return claims, nil
// }
