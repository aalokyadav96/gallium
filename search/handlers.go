package search

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"naevis/globals"
	"naevis/models"

	"github.com/julienschmidt/httprouter"
)

func EventHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log.Printf("[EventHandler] START method=%s", r.Method)

	if r.Method != http.MethodPost && r.Method != http.MethodPut &&
		r.Method != http.MethodDelete && r.Method != http.MethodPatch {
		http.Error(w, "Only POST, PUT, PATCH, and DELETE requests are allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var event models.Index
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := globals.RedisClient.Publish(r.Context(), "indexing-events", string(body)).Err(); err != nil {
		http.Error(w, "Failed to enqueue indexing job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"message":"Event queued successfully"}`))
}

func SearchHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET allowed", http.StatusMethodNotAllowed)
		return
	}

	entityType := ps.ByName("entityType")
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		query = strings.TrimSpace(r.URL.Query().Get("query"))
	}
	if query == "" {
		http.Error(w, "Search query is required", http.StatusBadRequest)
		return
	}

	res, err := GetResultsOfType(r.Context(), entityType, query, 50)
	if err != nil {
		http.Error(w, "Error fetching search results", http.StatusInternalServerError)
		return
	}

	payload, err := json.Marshal(res)
	if err != nil {
		http.Error(w, "Error encoding JSON", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}
