package utils

import (
	"naevis/globals"
	"naevis/middleware"
	"net/http"
)

func GetUserIDFromRequest(r *http.Request) string {
	ctx := r.Context()
	requestingUserID, ok := ctx.Value(globals.UserIDKey).(string)
	if !ok || requestingUserID == "" {
		return ""
	}
	return requestingUserID
}

func GetUsernameFromRequest(r *http.Request) string {
	tokenString := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(tokenString)
	if err != nil {
		return ""
	}
	return claims.Username
}
