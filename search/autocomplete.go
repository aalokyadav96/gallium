package search

import (
	"context"
	"encoding/json"
	"log"
	"naevis/rdx"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/redis/go-redis/v9"
)

// Redis key helpers for autocomplete
func autocompleteZSet() string { return "autocomplete:zset" }
func autocompleteCacheKey(prefix string) string {
	return "autocomplete_cache:" + prefix
}

// Add words to autocomplete index
func AddAutocompleteWords(ctx context.Context, words []string) error {
	if len(words) == 0 {
		return nil
	}
	pipe := rdx.Conn.Pipeline()
	for _, w := range words {
		if w != "" {
			pipe.ZAdd(ctx, autocompleteZSet(), redis.Z{Score: 0, Member: w})
		}
	}
	_, err := pipe.Exec(ctx)
	return err
}

// Get autocomplete suggestions with optional Redis caching
func GetAutocompleteSuggestions(ctx context.Context, prefix string, limit int, ttl time.Duration) ([]string, error) {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix == "" {
		return nil, nil
	}

	cacheKey := autocompleteCacheKey(prefix)
	if data, err := rdx.Conn.Get(ctx, cacheKey).Result(); err == nil {
		var res []string
		if json.Unmarshal([]byte(data), &res) == nil {
			if len(res) > limit {
				return res[:limit], nil
			}
			return res, nil
		}
	}

	min := "[" + prefix
	max := "[" + prefix + "\xff"
	words, err := rdx.Conn.ZRangeByLex(ctx, autocompleteZSet(), &redis.ZRangeBy{
		Min:    min,
		Max:    max,
		Offset: 0,
		Count:  int64(limit),
	}).Result()
	if err != nil {
		return nil, err
	}

	if ttl > 0 {
		if data, err := json.Marshal(words); err == nil {
			_ = rdx.Conn.Set(ctx, cacheKey, data, ttl).Err()
		}
	}
	return words, nil
}

// HTTP handler for autocomplete
func Autocompleter(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	prefix := strings.TrimSpace(r.URL.Query().Get("prefix"))
	if prefix == "" {
		http.Error(w, "Search prefix is required", http.StatusBadRequest)
		return
	}

	results, err := GetAutocompleteSuggestions(r.Context(), strings.ToLower(prefix), 20, time.Minute)
	if err != nil {
		log.Printf("Autocompleter error: %v", err)
		http.Error(w, "Error retrieving autocomplete suggestions", http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(results)
	if err != nil {
		http.Error(w, "Error encoding autocomplete results", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
