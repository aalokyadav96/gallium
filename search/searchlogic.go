package search

import (
	"context"
	"log"
	"naevis/rdx"
	"sort"
	"strings"
	"sync"
)

// -------------------------
// Search logic
// -------------------------

func GetIndexedResults(ctx context.Context, query string, limit int) ([]string, error) {
	log.Printf("[GetIndexedResults] START query=%q limit=%d", query, limit)
	tokens := Tokenize(query)
	if len(tokens) == 0 {
		log.Println("[GetIndexedResults] No tokens, returning nil")
		return nil, nil
	}

	type tokenList struct {
		ids []string
		err error
	}
	tl := make([]tokenList, len(tokens))

	var wg sync.WaitGroup
	for i, token := range tokens {
		wg.Add(1)
		go func(i int, token string) {
			defer wg.Done()
			ids, err := GetIndexIDsForToken(ctx, token)
			tl[i] = tokenList{ids: ids, err: err}
		}(i, token)
	}
	wg.Wait()

	for i, t := range tl {
		if t.err != nil {
			log.Printf("[GetIndexedResults] Token %q error: %v", tokens[i], t.err)
			return nil, t.err
		}
		if len(t.ids) == 0 {
			log.Printf("[GetIndexedResults] Token %q returned no IDs", tokens[i])
			return nil, nil
		}
	}

	sort.Slice(tl, func(i, j int) bool { return len(tl[i].ids) < len(tl[j].ids) })
	base := tl[0].ids
	log.Printf("[GetIndexedResults] Base token IDs=%v", base)

	otherSets := make([]map[string]struct{}, len(tl)-1)
	for i := 1; i < len(tl); i++ {
		m := make(map[string]struct{}, len(tl[i].ids))
		for _, id := range tl[i].ids {
			m[id] = struct{}{}
		}
		otherSets[i-1] = m
	}

	out := make([]string, 0, len(base))
	for _, id := range base {
		match := true
		for _, s := range otherSets {
			if _, ok := s[id]; !ok {
				match = false
				break
			}
		}
		if match {
			out = append(out, id)
			log.Printf("[GetIndexedResults] Matched ID=%q", id)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}

	log.Printf("[GetIndexedResults] END matchedIDs=%v", out)
	return out, nil
}

func SearchWithHashtagBoost(ctx context.Context, query string, limit int) ([]string, error) {
	log.Printf("[SearchWithHashtagBoost] START query=%q limit=%d", query, limit)
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		log.Println("[SearchWithHashtagBoost] Empty query, returning nil")
		return nil, nil
	}

	tokens := Tokenize(query)
	if len(tokens) == 0 {
		return nil, nil
	}
	hashtags := ExtractHashtags(query)

	scoreMap := make(map[string]int)
	log.Printf("[SearchWithHashtagBoost] Tokens=%v Hashtags=%v", tokens, hashtags)

	for _, t := range tokens {
		ids, err := GetIndexIDsForToken(ctx, t)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			scoreMap[id] += 3
			log.Printf("[SearchWithHashtagBoost] Token %q added ID=%q score=+3 total=%d", t, id, scoreMap[id])
		}
	}

	for _, h := range hashtags {
		ids, err := GetIndexIDsForToken(ctx, h)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			scoreMap[id] += 7
			log.Printf("[SearchWithHashtagBoost] Hashtag %q added ID=%q score=+7 total=%d", h, id, scoreMap[id])
		}
	}

	if len(scoreMap) == 0 {
		log.Println("[SearchWithHashtagBoost] No matches, returning nil")
		return nil, nil
	}

	type pair struct {
		id    string
		score int
	}
	pairs := make([]pair, 0, len(scoreMap))
	for id, sc := range scoreMap {
		pairs = append(pairs, pair{id: id, score: sc})
	}

	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].score != pairs[j].score {
			return pairs[i].score > pairs[j].score
		}
		key := invertedKey(tokens[0])
		si, erri := rdx.Conn.ZScore(ctx, key, pairs[i].id).Result()
		sj, errj := rdx.Conn.ZScore(ctx, key, pairs[j].id).Result()
		if erri != nil || errj != nil {
			return pairs[i].id < pairs[j].id
		}
		return si > sj
	})

	ids := make([]string, 0, len(pairs))
	for i, p := range pairs {
		if limit > 0 && i >= limit {
			break
		}
		ids = append(ids, p.id)
	}
	log.Printf("[SearchWithHashtagBoost] END IDs=%v", ids)
	return ids, nil
}

func GetIndexResults(ctx context.Context, query string, limit int) ([]string, error) {
	log.Printf("[GetIndexResults] query=%q limit=%d", query, limit)
	if strings.Contains(query, "#") {
		log.Println("[GetIndexResults] Detected hashtag, using SearchWithHashtagBoost")
		return SearchWithHashtagBoost(ctx, query, limit)
	}
	log.Println("[GetIndexResults] No hashtag, using GetIndexedResults")
	return GetIndexedResults(ctx, query, limit)
}
