package feed

import (
	"encoding/json"
	"naevis/middleware"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// POST /api/v1/feed/post
func CreateFeedPost(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	token := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(token)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var payload PostPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	post, err := CreateOrEditPost(ctx, claims, payload, ActionCreate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":   true,
		"data": post,
	})
}

// PATCH /api/v1/feed/post/:postid
func EditPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	token := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(token)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var payload PostPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	payload.PostID = ps.ByName("postid")

	post, err := CreateOrEditPost(ctx, claims, payload, ActionEdit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":   true,
		"data": post,
	})
}
