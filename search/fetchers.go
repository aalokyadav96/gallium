package search

import (
	"context"
	"errors"
	"fmt"
	"log"
	"naevis/db"
	"naevis/models"
	"reflect"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// -------------------------
// Mongo helpers
// -------------------------

func SaveEntityToDB(ctx context.Context, entity Entity) error {
	log.Printf("[SaveEntityToDB] START entity=%+v", entity)
	coll := db.Client.Database("naevis").Collection("search")
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
	err := db.Client.Database("naevis").Collection("search").
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
	err := db.Client.Database("eventdb").Collection(collectionName).FindOne(ctx, filter, opts).Decode(out)
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
