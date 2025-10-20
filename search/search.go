package search

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"naevis/db"
	"naevis/models"
	"naevis/rdx"

	"go.mongodb.org/mongo-driver/bson"

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

	pipe := rdx.Conn.Pipeline()
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

	pipe := rdx.Conn.Pipeline()
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

	_, err = db.Client.Database("naevis").Collection("search").DeleteOne(ctx, bson.M{"entityid": id})
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

	pipe := rdx.Conn.Pipeline()
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
		return Entity{EntityID: v.UserID, EntityType: "users", Title: v.Username, Image: v.Avatar, Description: v.Bio, CreatedAt: parseTime(v.CreatedAt)}, nil
	case models.Recipe:
		// var img string
		// if len(v.ImageURLs) > 0 {
		// 	img = v.ImageURLs[0]
		// }
		return Entity{EntityID: v.RecipeId, EntityType: "recipes", Title: v.Title, Image: v.Banner, Description: v.Description, CreatedAt: parseTime(v.CreatedAt)}, nil
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
		return Entity{EntityID: v.CropId, EntityType: "crops", Title: v.Name, Image: v.Banner, Description: v.Category, CreatedAt: parseTime(v.CreatedAt)}, nil
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
