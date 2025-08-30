package home

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"naevis/db"
	"naevis/utils"
)

// HomeCard response type
type HomeCard struct {
	Banner      string `json:"banner,omitempty" bson:"-"`
	Title       string `json:"title" bson:"title"`
	Description string `json:"description" bson:"description"`
	Href        string `json:"href" bson:"-"`
}

// Ensure indexes once during startup
func EnsureHomeCardsIndexes(ctx context.Context) error {
	collections := []*mongo.Collection{
		db.PlacesCollection,
		db.EventsCollection,
		db.BaitoCollection,
		db.ProductCollection,
		db.BlogPostsCollection,
	}
	model := mongo.IndexModel{Keys: bson.D{{Key: "created_at", Value: -1}}}
	for _, c := range collections {
		if c == nil {
			continue
		}
		if _, err := c.Indexes().CreateOne(ctx, model); err != nil {
			return err
		}
	}
	return nil
}

// categoryProjection returns collection and projection function
func categoryProjection(category string) (*mongo.Collection, func(bson.M) HomeCard) {
	switch category {
	case "Places":
		return db.PlacesCollection, func(doc bson.M) HomeCard {
			id := ""
			if val, ok := doc["placeid"].(string); ok {
				id = val
			}
			banner := ""
			if val, ok := doc["banner"].(string); ok {
				banner = val
			}
			title, _ := doc["name"].(string)
			desc, _ := doc["description"].(string)
			return HomeCard{
				Banner:      banner,
				Title:       title,
				Description: desc,
				Href:        "/place/" + id,
			}
		}
	case "Events":
		return db.EventsCollection, func(doc bson.M) HomeCard {
			id := ""
			if val, ok := doc["eventid"].(string); ok {
				id = val
			}
			banner := ""
			if val, ok := doc["banner"].(string); ok {
				banner = val
			}
			title, _ := doc["title"].(string)
			desc, _ := doc["description"].(string)
			return HomeCard{
				Banner:      banner,
				Title:       title,
				Description: desc,
				Href:        "/event/" + id,
			}
		}
	case "Baitos":
		return db.BaitoCollection, func(doc bson.M) HomeCard {
			id := ""
			if val, ok := doc["baitoid"].(string); ok {
				id = val
			}
			banner := ""
			if val, ok := doc["banner"].(string); ok {
				banner = val
			}
			title, _ := doc["title"].(string)
			desc, _ := doc["description"].(string)
			return HomeCard{
				Banner:      banner,
				Title:       title,
				Description: desc,
				Href:        "/baito/" + id,
			}
		}
	case "Products":
		return db.ProductCollection, func(doc bson.M) HomeCard {
			id := ""
			if val, ok := doc["productid"].(string); ok {
				id = val
			}
			banner := ""
			if arr, ok := doc["imageUrls"].(bson.A); ok && len(arr) > 0 {
				if s, ok := arr[0].(string); ok {
					banner = s
				}
			}
			title, _ := doc["name"].(string)
			desc, _ := doc["description"].(string)
			return HomeCard{
				Banner:      banner,
				Title:       title,
				Description: desc,
				Href:        "/product/" + id,
			}
		}
	case "Posts":
		return db.BlogPostsCollection, func(doc bson.M) HomeCard {
			id := ""
			if val, ok := doc["postid"].(string); ok {
				id = val
			}
			banner := doc["thumb"].(string)
			// banner := ""
			// if arr, ok := doc["imagePaths"].(bson.A); ok && len(arr) > 0 {
			// 	if s, ok := arr[0].(string); ok {
			// 		banner = s
			// 	}
			// }
			title, _ := doc["title"].(string)
			desc, _ := doc["content"].(string)
			return HomeCard{
				Banner:      banner,
				Title:       title,
				Description: desc,
				Href:        "/post/" + id,
			}
		}
	default:
		return nil, nil
	}
}

func HomeCardsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	reqID := r.Header.Get("X-Request-Id")
	if reqID == "" {
		reqID = strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + strconv.FormatInt(rand.Int63(), 36)
	}
	w.Header().Set("X-Request-Id", reqID)
	w.Header().Set("Content-Type", "application/json")

	category := r.URL.Query().Get("category")
	collection, projector := categoryProjection(category)
	if collection == nil || projector == nil {
		_ = json.NewEncoder(w).Encode([]HomeCard{})
		return
	}

	skip, limit := utils.ParsePagination(r, 0, 20)

	opts := options.Find().SetSkip(skip).SetLimit(limit).SetSort(bson.D{{Key: "created_at", Value: -1}})

	cur, err := collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		log.Println("Find error:", err, "req_id:", reqID)
		w.Header().Set("X-Error-Request-Id", reqID)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch home cards")
		return
	}
	defer cur.Close(ctx)

	var cards []HomeCard
	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err != nil {
			log.Println("Decode error:", err, "req_id:", reqID)
			continue
		}
		cards = append(cards, projector(doc))
	}

	utils.RespondWithJSON(w, http.StatusOK, cards)
}

// package home

// import (
// 	"context"
// 	"encoding/json"
// 	"log"
// 	"math/rand"
// 	"net/http"
// 	"strconv"
// 	"time"

// 	"github.com/julienschmidt/httprouter"
// 	"go.mongodb.org/mongo-driver/bson"
// 	"go.mongodb.org/mongo-driver/mongo"
// 	"go.mongodb.org/mongo-driver/mongo/options"

// 	"naevis/db"
// 	"naevis/utils"
// )

// type HomeCard struct {
// 	Banner      string `json:"banner" bson:"banner"`
// 	Title       string `json:"title" bson:"title"`
// 	Description string `json:"description" bson:"description"`
// 	Href        string `json:"href" bson:"href"`
// }

// // Call this once during app startup (e.g., in main or your db init path)
// func EnsureHomeCardsIndexes(ctx context.Context) error {
// 	collections := []*mongo.Collection{
// 		db.PlacesCollection,
// 		db.EventsCollection,
// 		db.BaitoCollection,
// 		db.ProductCollection,
// 		db.BlogPostsCollection,
// 	}
// 	model := mongo.IndexModel{
// 		Keys: bson.D{{Key: "created_at", Value: -1}},
// 	}
// 	for _, c := range collections {
// 		if c == nil {
// 			continue
// 		}
// 		_, err := c.Indexes().CreateOne(ctx, model)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

// func HomeCardsHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
// 	defer cancel()

// 	// Generate/propagate a simple request id for correlation
// 	reqID := r.Header.Get("X-Request-Id")
// 	if reqID == "" {
// 		reqID = strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + strconv.FormatInt(rand.Int63(), 36)
// 	}
// 	w.Header().Set("X-Request-Id", reqID)
// 	w.Header().Set("Content-Type", "application/json")

// 	category := r.URL.Query().Get("category")
// 	var collection *mongo.Collection

// 	switch category {
// 	case "Places":
// 		collection = db.PlacesCollection
// 	case "Events":
// 		collection = db.EventsCollection
// 	case "Baitos":
// 		collection = db.BaitoCollection
// 	case "Products":
// 		collection = db.ProductCollection
// 	case "Posts":
// 		collection = db.BlogPostsCollection
// 	default:
// 		// empty result instead of error
// 		_ = json.NewEncoder(w).Encode([]HomeCard{})
// 		return
// 	}

// 	// pagination (default limit 20)
// 	skip, limit := utils.ParsePagination(r, 0, 20)

// 	opts := options.Find().
// 		SetSkip(skip).
// 		SetLimit(limit).
// 		SetSort(bson.D{{Key: "created_at", Value: -1}}).
// 		SetProjection(bson.D{
// 			{Key: "banner", Value: 1},
// 			{Key: "title", Value: 1},
// 			{Key: "description", Value: 1},
// 			{Key: "href", Value: 1},
// 		})

// 	cards, err := utils.FindAndDecode[HomeCard](ctx, collection, bson.M{}, opts)
// 	if err != nil {
// 		log.Println("FindAndDecode error:", err, "req_id:", reqID, "category:", category, "skip:", skip, "limit:", limit)
// 		// include request id in error for easy correlation
// 		w.Header().Set("X-Error-Request-Id", reqID)
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch home cards")
// 		return
// 	}

// 	utils.RespondWithJSON(w, http.StatusOK, cards)
// }

// // package home

// // import (
// // 	"context"
// // 	"encoding/json"
// // 	"log"
// // 	"net/http"
// // 	"time"

// // 	"github.com/julienschmidt/httprouter"
// // 	"go.mongodb.org/mongo-driver/bson"
// // 	"go.mongodb.org/mongo-driver/mongo"
// // 	"go.mongodb.org/mongo-driver/mongo/options"

// // 	"naevis/db"
// // 	"naevis/utils"
// // )

// // type HomeCard struct {
// // 	Banner      string `json:"banner" bson:"banner"`
// // 	Title       string `json:"title" bson:"title"`
// // 	Description string `json:"description" bson:"description"`
// // 	Href        string `json:"href" bson:"href"`
// // }

// // func HomeCardsHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// // 	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
// // 	defer cancel()

// // 	category := r.URL.Query().Get("category")
// // 	var collection *mongo.Collection

// // 	switch category {
// // 	case "Places":
// // 		collection = db.PlacesCollection
// // 	case "Events":
// // 		collection = db.EventsCollection
// // 	case "Baitos":
// // 		collection = db.BaitoCollection
// // 	case "Products":
// // 		collection = db.ProductCollection
// // 	case "Posts":
// // 		collection = db.BlogPostsCollection
// // 	default:
// // 		// empty result instead of error
// // 		w.Header().Set("Content-Type", "application/json")
// // 		json.NewEncoder(w).Encode([]HomeCard{})
// // 		return
// // 	}

// // 	skip, limit := utils.ParsePagination(r, 0, 20) // default 20 cards
// // 	opts := options.Find().SetSkip(skip).SetLimit(limit).SetSort(bson.D{{Key: "created_at", Value: -1}})

// // 	cards, err := utils.FindAndDecode[HomeCard](ctx, collection, bson.M{}, opts)
// // 	if err != nil {
// // 		log.Println("FindAndDecode error:", err)
// // 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch home cards")
// // 		return
// // 	}

// // 	utils.RespondWithJSON(w, http.StatusOK, cards)
// // }
