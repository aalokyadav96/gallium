package feed

import (
	"encoding/json"
	"naevis/middleware"
	"naevis/utils"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// POST /api/v1/feed/tweet
func CreateTweetPost(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	token := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(token)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	payload := PostPayload{
		Type:        r.FormValue("type"),
		Text:        r.FormValue("text"),
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		Tags:        utils.SplitTags(r.FormValue("tags")),
	}

	post, err := CreateOrEditPost(ctx, claims, payload, r, ActionCreate)
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

	post, err := CreateOrEditPost(ctx, claims, payload, r, ActionEdit)
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
