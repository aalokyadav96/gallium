package search

import (
	"context"
	"log"
	"naevis/rdx"
	"regexp"
	"strings"

	"github.com/redis/go-redis/v9"
)

// -------------------------
// Tokenization
// -------------------------

var tokenRegex = regexp.MustCompile(`(?i)(#\w+)|([A-Za-z0-9_]+)`)
var stopWords = map[string]bool{
	"the": true, "and": true, "of": true, "in": true, "to": true,
	"for": true, "on": true, "with": true, "a": true, "an": true,
}

func Tokenize(text string) []string {
	log.Printf("[Tokenize] START text=%q", text)
	if strings.TrimSpace(text) == "" {
		log.Println("[Tokenize] Empty or whitespace-only input, returning nil")
		return nil
	}
	matches := tokenRegex.FindAllString(text, -1)
	log.Printf("[Tokenize] Raw matches=%v", matches)

	out := make([]string, 0, len(matches))
	seen := map[string]struct{}{}
	for _, m := range matches {
		t := strings.ToLower(m)
		if stopWords[t] {
			log.Printf("[Tokenize] Skipping stopword=%q", t)
			continue
		}
		if _, ok := seen[t]; ok {
			log.Printf("[Tokenize] Skipping duplicate=%q", t)
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
		log.Printf("[Tokenize] Added token=%q", t)
	}

	log.Printf("[Tokenize] END tokens=%v", out)
	return out
}

func ExtractHashtags(text string) []string {
	log.Printf("[ExtractHashtags] START text=%q", text)
	tokens := Tokenize(text)
	var tags []string
	for _, t := range tokens {
		if strings.HasPrefix(t, "#") {
			tags = append(tags, t)
			log.Printf("[ExtractHashtags] Found hashtag=%q", t)
		}
	}
	log.Printf("[ExtractHashtags] END hashtags=%v", tags)
	return tags
}

// -------------------------
// Redis inverted index helpers
// -------------------------

func invertedKey(token string) string { return "inverted:" + token }
func hashtagKey(token string) string  { return "hashtag:" + token }

func addToIndexPipeline(ctx context.Context, pipe redis.Pipeliner, key, member string, createdAtUnixNano float64) {
	log.Printf("[addToIndexPipeline] key=%q member=%q score=%v", key, member, createdAtUnixNano)
	pipe.ZAdd(ctx, key, redis.Z{Score: createdAtUnixNano, Member: member})
}

func deleteFromIndexPipeline(ctx context.Context, pipe redis.Pipeliner, key, member string) {
	log.Printf("[deleteFromIndexPipeline] key=%q member=%q", key, member)
	pipe.ZRem(ctx, key, member)
}

func GetIndexIDsForToken(ctx context.Context, token string) ([]string, error) {
	log.Printf("[GetIndexIDsForToken] token=%q", token)
	ids, err := rdx.Conn.ZRevRange(ctx, invertedKey(token), 0, -1).Result()
	log.Printf("[GetIndexIDsForToken] ids=%v err=%v", ids, err)
	return ids, err
}
