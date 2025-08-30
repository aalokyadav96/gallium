package search

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"naevis/db"
	"naevis/globals"
	"naevis/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/redis/go-redis/v9"
)

// -------------------------
// Types
// -------------------------

type Entity struct {
	EntityID    string    `json:"entityid" bson:"entityid"`
	EntityType  string    `json:"entitytype" bson:"entitytype"`
	Title       string    `json:"title" bson:"title"`
	Image       string    `json:"image" bson:"image"`
	Description string    `json:"description" bson:"description"`
	CreatedAt   time.Time `json:"createdAt" bson:"createdAt"`
}

// -------------------------
// Mongo helpers
// -------------------------

func SaveEntityToDB(ctx context.Context, entity Entity) error {
	log.Printf("[SaveEntityToDB] START entity=%+v", entity)
	coll := globals.MongoClient.Database("naevis").Collection("search")
	_, err := coll.UpdateOne(ctx,
		bson.M{"entityid": entity.EntityID, "entitytype": entity.EntityType},
		bson.M{"$set": entity},
		options.Update().SetUpsert(true),
	)
	log.Printf("[SaveEntityToDB] END err=%v", err)
	return err
}

func FetchEntityFromSearchDB(ctx context.Context, id string) (Entity, error) {
	log.Printf("[FetchEntityFromSearchDB] START id=%q", id)
	var ent Entity
	err := globals.MongoClient.Database("naevis").Collection("search").
		FindOne(ctx, bson.M{"entityid": id}).Decode(&ent)
	log.Printf("[FetchEntityFromSearchDB] END entity=%+v err=%v", ent, err)
	return ent, err
}

func FetchAndDecode(ctx context.Context, collectionName string, filter bson.M, out interface{}) error {
	log.Printf("[FetchAndDecode] START collection=%q filter=%v", collectionName, filter)
	projection, exists := Projections[collectionName]
	if !exists {
		projection = bson.M{}
	}
	log.Printf("[FetchAndDecode] projection=%v", projection)
	opts := options.FindOne().SetProjection(projection)
	err := globals.MongoClient.Database("eventdb").Collection(collectionName).FindOne(ctx, filter, opts).Decode(out)
	log.Printf("[FetchAndDecode] END err=%v", err)
	return err
}

// -------------------------
// Search fetching
// -------------------------

func fetchOne[T any](ctx context.Context, coll *mongo.Collection, idField string, id string) (T, error) {
	var doc T
	err := coll.FindOne(ctx, bson.M{idField: id}).Decode(&doc)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		log.Printf("[fetchOne] Error fetching ID=%v: %v", id, err)
	}
	return doc, err
}

func fetchResults[T any](ctx context.Context, query string, limit int, coll *mongo.Collection, entityType string) ([]T, error) {
	ids, err := GetIndexResults(ctx, query, limit)
	if err != nil || len(ids) == 0 {
		return nil, err
	}

	log.Println("[fetchResults] ids:", ids)

	// Filter by both entityid and entitytype
	filter := bson.M{
		"entityid":   bson.M{"$in": ids},
		"entitytype": entityType,
	}
	cur, err := coll.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	log.Println("[fetchResults] cursor:", cur)

	var results []T
	if err := cur.All(ctx, &results); err != nil {
		return nil, err
	}
	log.Println("[fetchResults] results:", results)

	// Preserve Redis order
	idIndex := make(map[string]int, len(ids))
	for i, id := range ids {
		idIndex[fmt.Sprint(id)] = i
	}
	log.Println("[fetchResults] idIndex:", idIndex)

	ordered := make([]T, len(results))
	count := 0
	for _, doc := range results {
		raw, _ := bson.Marshal(doc)
		var m bson.M
		_ = bson.Unmarshal(raw, &m)
		if entityID, ok := m["entityid"].(string); ok {
			if idx, exists := idIndex[entityID]; exists {
				ordered[idx] = doc
				count++
			}
		}
	}
	log.Println("[fetchResults] ordered:", ordered)

	final := make([]T, 0, count)
	for _, doc := range ordered {
		if !isZero(doc) {
			final = append(final, doc)
		}
	}
	log.Println("[fetchResults] final:", final)
	return final, nil
}

func isZero[T any](v T) bool {
	return reflect.ValueOf(v).IsZero()
}

func GetResultsOfType(ctx context.Context, entityType, query string, limit int) (interface{}, error) {
	log.Printf("entityType: %s, query: %s, limit: %d", entityType, query, limit)
	allowedTypes := []string{
		"songs", "users", "recipes", "products", "blogposts", "feedposts",
		"places", "merch", "menu", "media", "farms", "events", "crops",
		"baitoworkers", "baitos", "artists",
	}
	if !contains(allowedTypes, entityType) {
		return nil, nil
	}
	return fetchResults[Entity](ctx, query, limit, db.Client.Database("naevis").Collection("search"), entityType)
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func GetResultsByTypeRaw(ctx context.Context, entityType, id string) (interface{}, error) {
	log.Printf("[GetResultsByTypeRaw] START entityType=%q id=%q", entityType, id)

	switch entityType {
	case "song":
		return fetchOne[models.ArtistSong](ctx, db.SongsCollection, "songid", id)
	case "user":
		return fetchOne[models.User](ctx, db.UserCollection, "userid", id)
	case "recipe":
		return fetchOne[models.Recipe](ctx, db.RecipeCollection, "recipeid", id)
	case "product":
		return fetchOne[models.Product](ctx, db.ProductCollection, "productid", id)
	case "blogpost":
		return fetchOne[models.BlogPost](ctx, db.BlogPostsCollection, "postid", id)
	case "place":
		return fetchOne[models.MPlace](ctx, db.PlacesCollection, "placeid", id)
	case "merch":
		return fetchOne[models.Merch](ctx, db.MerchCollection, "merchid", id)
	case "menu":
		return fetchOne[models.Menu](ctx, db.MenuCollection, "menuid", id)
	case "media":
		return fetchOne[models.Media](ctx, db.MediaCollection, "mediaid", id)
	case "farm":
		return fetchOne[models.Farm](ctx, db.FarmsCollection, "farmid", id)
	case "event":
		return fetchOne[models.MEvent](ctx, db.EventsCollection, "eventid", id)
	case "crop":
		return fetchOne[models.Crop](ctx, db.CropsCollection, "cropid", id)
	case "baitoworker":
		return fetchOne[models.BaitoWorker](ctx, db.BaitoWorkerCollection, "baito_user_id", id)
	case "baito":
		return fetchOne[models.Baito](ctx, db.BaitoCollection, "baitoid", id)
	case "artist":
		return fetchOne[models.Artist](ctx, db.ArtistsCollection, "artistid", id)
	case "feedpost":
		return fetchOne[models.FeedPost](ctx, db.PostsCollection, "postid", id)
	default:
		err := fmt.Errorf("unsupported entity type: %s", entityType)
		log.Printf("[GetResultsByTypeRaw] ERROR %v", err)
		return nil, err
	}
}

// -------------------------
// Indexing flows
// -------------------------

func IndexEntity(ctx context.Context, entity Entity) error {
	log.Printf("[IndexEntity] START entity=%+v", entity)

	if err := SaveEntityToDB(ctx, entity); err != nil {
		return fmt.Errorf("[IndexEntity] save entity to db: %w", err)
	}

	text := strings.TrimSpace(entity.Title + " " + entity.Description)
	tokens := Tokenize(text)
	log.Printf("[IndexEntity] tokens=%v", tokens)

	if len(tokens) == 0 {
		log.Println("[IndexEntity] No tokens, skipping indexing")
		return nil
	}

	pipe := globals.RedisClient.Pipeline()
	for _, token := range tokens {
		addToIndexPipeline(ctx, pipe, invertedKey(token), entity.EntityID, float64(entity.CreatedAt.UnixNano()))
		if strings.HasPrefix(token, "#") {
			addToIndexPipeline(ctx, pipe, hashtagKey(token), entity.EntityID, float64(entity.CreatedAt.UnixNano()))
		}
		pipe.ZAdd(ctx, autocompleteZSet(), redis.Z{Score: 0, Member: token})
		log.Printf("[IndexEntity] Added token=%q to autocomplete zset", token)
	}

	_, err := pipe.Exec(ctx)
	log.Printf("[IndexEntity] END err=%v", err)
	return err
}

func DeleteEntity(ctx context.Context, id string) error {
	log.Printf("[DeleteEntity] START id=%q", id)
	ent, err := FetchEntityFromSearchDB(ctx, id)
	if err != nil {
		log.Printf("[DeleteEntity] entity not found err=%v", err)
		return err
	}

	tokens := Tokenize(ent.Title + " " + ent.Description)
	log.Printf("[DeleteEntity] tokens=%v", tokens)

	pipe := globals.RedisClient.Pipeline()
	for _, token := range tokens {
		deleteFromIndexPipeline(ctx, pipe, invertedKey(token), id)
		if strings.HasPrefix(token, "#") {
			deleteFromIndexPipeline(ctx, pipe, hashtagKey(token), id)
		}
	}
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("[DeleteEntity] Redis pipeline error=%v", err)
		return err
	}

	_, err = globals.MongoClient.Database("naevis").Collection("search").DeleteOne(ctx, bson.M{"entityid": id})
	log.Printf("[DeleteEntity] END err=%v", err)
	return err
}

func UpdateEntityIndexes(ctx context.Context, newEntity Entity) error {
	log.Printf("[UpdateEntityIndexes] START newEntity=%+v", newEntity)

	oldEnt, err := FetchEntityFromSearchDB(ctx, newEntity.EntityID)
	if err != nil {
		log.Printf("[UpdateEntityIndexes] old entity not found, indexing new entity")
		return IndexEntity(ctx, newEntity)
	}

	oldTokens := Tokenize(oldEnt.Title + " " + oldEnt.Description)
	newTokens := Tokenize(newEntity.Title + " " + newEntity.Description)
	log.Printf("[UpdateEntityIndexes] oldTokens=%v newTokens=%v", oldTokens, newTokens)

	oldSet := make(map[string]struct{}, len(oldTokens))
	newSet := make(map[string]struct{}, len(newTokens))
	for _, t := range oldTokens {
		oldSet[t] = struct{}{}
	}
	for _, t := range newTokens {
		newSet[t] = struct{}{}
	}

	var toAdd, toRemove []string
	for t := range oldSet {
		if _, ok := newSet[t]; !ok {
			toRemove = append(toRemove, t)
		}
	}
	for t := range newSet {
		if _, ok := oldSet[t]; !ok {
			toAdd = append(toAdd, t)
		}
	}
	log.Printf("[UpdateEntityIndexes] toAdd=%v toRemove=%v", toAdd, toRemove)

	if len(toAdd) == 0 && len(toRemove) == 0 {
		log.Printf("[UpdateEntityIndexes] No changes, only updating DB")
		return SaveEntityToDB(ctx, newEntity)
	}

	pipe := globals.RedisClient.Pipeline()
	for _, token := range toRemove {
		deleteFromIndexPipeline(ctx, pipe, invertedKey(token), newEntity.EntityID)
		if strings.HasPrefix(token, "#") {
			deleteFromIndexPipeline(ctx, pipe, hashtagKey(token), newEntity.EntityID)
		}
	}
	for _, token := range toAdd {
		addToIndexPipeline(ctx, pipe, invertedKey(token), newEntity.EntityID, float64(newEntity.CreatedAt.UnixNano()))
		if strings.HasPrefix(token, "#") {
			addToIndexPipeline(ctx, pipe, hashtagKey(token), newEntity.EntityID, float64(newEntity.CreatedAt.UnixNano()))
		}
		pipe.ZAdd(ctx, autocompleteZSet(), redis.Z{Score: 0, Member: token})
	}
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("[UpdateEntityIndexes] Redis pipeline error=%v", err)
		return err
	}

	err = SaveEntityToDB(ctx, newEntity)
	log.Printf("[UpdateEntityIndexes] END err=%v", err)
	return err
}

// -------------------------
// Index data dispatcher
// -------------------------

func IndexDatainRedis(ctx context.Context, event models.Index) error {
	log.Printf("[IndexDatainRedis] START event=%+v", event)

	switch strings.ToUpper(event.Method) {
	case "DELETE":
		log.Println("[IndexDatainRedis] Method=DELETE")
		return DeleteEntity(ctx, event.EntityId)

	case "PATCH", "PUT":
		log.Printf("[IndexDatainRedis] Method=%s", event.Method)
		data, err := GetResultsByTypeRaw(ctx, event.EntityType, event.EntityId)
		if err != nil {
			return err
		}
		newEntity, err := ConvertToEntity(ctx, data)
		if err != nil {
			return err
		}
		return UpdateEntityIndexes(ctx, newEntity)

	case "POST":
		log.Println("[IndexDatainRedis] Method=POST")
		data, err := GetResultsByTypeRaw(ctx, event.EntityType, event.EntityId)
		if err != nil {
			return err
		}
		ent, err := ConvertToEntity(ctx, data)
		if err != nil {
			return err
		}
		return IndexEntity(ctx, ent)

	default:
		return fmt.Errorf("[IndexDatainRedis] unsupported method: %s", event.Method)
	}
}

// -------------------------
// Utility
// -------------------------

func ConvertToEntity(ctx context.Context, data interface{}) (Entity, error) {
	switch v := data.(type) {
	case models.ArtistSong:
		return Entity{EntityID: v.SongID, EntityType: "songs", Title: v.Title, Image: v.Poster, Description: v.Description, CreatedAt: parseTime(v.UploadedAt)}, nil
	case models.User:
		return Entity{EntityID: v.UserID, EntityType: "users", Title: v.Username, Image: v.ProfilePicture, Description: v.Bio, CreatedAt: parseTime(v.CreatedAt)}, nil
	case models.Recipe:
		var img string
		if len(v.ImageURLs) > 0 {
			img = v.ImageURLs[0]
		}
		return Entity{EntityID: v.RecipeId, EntityType: "recipes", Title: v.Title, Image: img, Description: v.Description, CreatedAt: parseTime(v.CreatedAt)}, nil
	case models.Product:
		var img string
		if len(v.ImageURLs) > 0 {
			img = v.ImageURLs[0]
		}
		return Entity{EntityID: v.ProductID, EntityType: "products", Title: v.Name, Image: img, Description: v.Description, CreatedAt: parseTime(v.CreatedAt)}, nil
	case models.Menu:
		return Entity{EntityID: v.MenuID, EntityType: "menu", Title: v.Name, Image: v.MenuPhoto, Description: v.Description, CreatedAt: parseTime(v.CreatedAt)}, nil
	case models.Media:
		return Entity{EntityID: v.MediaID, EntityType: "media", Title: v.Caption, Image: v.ThumbnailURL, Description: v.Caption, CreatedAt: parseTime(v.CreatedAt)}, nil
	case models.Crop:
		return Entity{EntityID: v.CropId, EntityType: "crops", Title: v.Name, Image: v.ImageURL, Description: v.Category, CreatedAt: parseTime(v.CreatedAt)}, nil
	case models.BaitoWorker:
		return Entity{EntityID: v.BaitoUserID, EntityType: "baitoworkers", Title: v.Name, Image: v.ProfilePic, Description: v.Bio, CreatedAt: parseTime(v.CreatedAt)}, nil
	case models.Artist:
		return Entity{EntityID: v.ArtistID, EntityType: "artists", Title: v.Name, Image: v.Photo, Description: v.Bio, CreatedAt: parseTime(v.CreatedAt)}, nil
	case models.MEvent:
		return Entity{EntityID: v.EventID, EntityType: "events", Title: v.Title, Image: v.Image, Description: v.Description, CreatedAt: parseTime(v.Date)}, nil
	case models.MPlace:
		return Entity{EntityID: v.PlaceID, EntityType: "places", Title: v.Name, Image: v.Image, Description: v.Description, CreatedAt: parseTime(v.CreatedAt)}, nil
	case models.BlogPost:
		var img string
		var desc string
		for _, b := range v.Blocks {
			if b.Type == "image" && b.URL != "" {
				img = b.URL
				break
			}
		}
		for _, b := range v.Blocks {
			if b.Type == "text" && b.Content != "" {
				desc = b.Content
				break
			}
		}
		return Entity{EntityID: v.PostID, EntityType: "blogposts", Title: v.Title, Image: img, Description: desc, CreatedAt: parseTime(v.CreatedAt)}, nil
	case models.Merch:
		return Entity{EntityID: v.MerchID, EntityType: "merch", Title: v.Name, Image: v.MerchPhoto, Description: v.Category, CreatedAt: parseTime(v.CreatedAt)}, nil
	case models.FeedPost:
		var img string
		if len(v.Media) > 0 {
			img = v.Media[0]
		}
		return Entity{EntityID: v.PostID, EntityType: "feedposts", Title: v.Text, Image: img, Description: v.Content, CreatedAt: parseTime(v.CreatedAt)}, nil
	case models.Farm:
		return Entity{EntityID: v.FarmID, EntityType: "farms", Title: v.Name, Image: v.Banner, Description: v.Description, CreatedAt: parseTime(v.CreatedAt)}, nil
	case models.Baito:
		return Entity{EntityID: v.BaitoId, EntityType: "baitos", Title: v.Title, Image: v.BannerURL, Description: v.Description, CreatedAt: parseTime(v.CreatedAt)}, nil
	case Entity:
		return v, nil
	case bson.M:
		id, _ := v["entityid"].(string)
		typ, _ := v["entitytype"].(string)
		title, _ := v["title"].(string)
		desc, _ := v["description"].(string)
		img, _ := v["image"].(string)
		created := time.Now()
		if tval, ok := v["createdAt"]; ok {
			created = parseTime(tval)
		}
		return Entity{EntityID: id, EntityType: typ, Title: title, Image: img, Description: desc, CreatedAt: created}, nil
	default:
		return Entity{}, fmt.Errorf("unsupported type %T", v)
	}
}

func parseTime(v interface{}) time.Time {
	switch t := v.(type) {
	case int:
		return time.Unix(0, int64(t)*int64(time.Millisecond))
	case int32:
		return time.Unix(0, int64(t)*int64(time.Millisecond))
	case int64:
		return time.Unix(0, t*int64(time.Millisecond))
	case float64:
		return time.Unix(0, int64(t)*int64(time.Millisecond))
	case string:
		if ms, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64); err == nil {
			return time.Unix(0, ms*int64(time.Millisecond))
		}
	case time.Time:
		return t
	}
	return time.Now()
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
	ids, err := globals.RedisClient.ZRevRange(ctx, invertedKey(token), 0, -1).Result()
	log.Printf("[GetIndexIDsForToken] ids=%v err=%v", ids, err)
	return ids, err
}

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
		si, erri := globals.RedisClient.ZScore(ctx, key, pairs[i].id).Result()
		sj, errj := globals.RedisClient.ZScore(ctx, key, pairs[j].id).Result()
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
