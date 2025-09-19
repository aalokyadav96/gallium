package pay

import (
	"context"
	"errors"
	"log"
	"naevis/db"
	"naevis/models"
	"naevis/rdx"
	"naevis/utils"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// PriceResolver resolves entityID -> price
type PriceResolver func(ctx context.Context, entityID string) (float64, error)

// PaymentService handles all wallet/payment ops
type PaymentService struct {
	resolvers map[string]PriceResolver
	rLock     sync.RWMutex
	rdx       *redis.Client
}

// NewPaymentService creates a new service instance
func NewPaymentService() *PaymentService {
	return &PaymentService{
		resolvers: make(map[string]PriceResolver),
		rdx:       rdx.Conn,
	}
}

// RegisterResolver registers a resolver for entity type (thread-safe)
func (p *PaymentService) RegisterResolver(entityType string, resolver PriceResolver) {
	p.rLock.Lock()
	defer p.rLock.Unlock()
	p.resolvers[entityType] = resolver
}

// GetResolver fetches a resolver
func (p *PaymentService) GetResolver(entityType string) (PriceResolver, error) {
	p.rLock.RLock()
	defer p.rLock.RUnlock()
	resolver, ok := p.resolvers[entityType]
	if !ok {
		log.Printf("PaymentService: unsupported entity type %q\n", entityType)
		return nil, errors.New("unsupported entity type")
	}
	return resolver, nil
}

// RegisterDefaultResolvers adds built-in resolvers
func (p *PaymentService) RegisterDefaultResolvers() {
	p.RegisterResolver("ticket", func(ctx context.Context, entityID string) (float64, error) {
		var ticket struct {
			Price float64 `bson:"price"`
		}
		if err := db.TicketsCollection.FindOne(ctx, bson.M{"ticketid": entityID}).Decode(&ticket); err != nil {
			return 0, err
		}
		return ticket.Price, nil
	})

	p.RegisterResolver("restaurant", func(ctx context.Context, entityID string) (float64, error) {
		var menu struct {
			Price float64 `bson:"price"`
		}
		if err := db.MenuCollection.FindOne(ctx, bson.M{"menuid": entityID}).Decode(&menu); err != nil {
			return 0, err
		}
		return menu.Price, nil
	})

	p.RegisterResolver("barber", func(ctx context.Context, entityID string) (float64, error) {
		var service struct {
			Price float64 `bson:"price"`
		}
		if err := db.ServiceCollection.FindOne(ctx, bson.M{"serviceid": entityID}).Decode(&service); err != nil {
			return 0, err
		}
		return service.Price, nil
	})

	p.RegisterResolver("post", func(ctx context.Context, entityID string) (float64, error) {
		// posts have no fixed price; user chooses donation
		var post struct {
			PostID string `bson:"postid"`
		}
		if err := db.BlogPostsCollection.FindOne(ctx, bson.M{"postid": entityID}).Decode(&post); err != nil {
			return 0, err
		}
		return 0, nil
	})
	p.RegisterResolver("order", func(ctx context.Context, entityID string) (float64, error) {
		// var order struct {
		// 	OrderID string `bson:"orderid"`
		// }
		// if err := db.OrderCollection.FindOne(ctx, bson.M{"orderid": entityID}).Decode(&order); err != nil {
		// 	return 0, err
		// }
		return 0, nil
	})
}

// ===== Handlers =====

// GetBalance returns user's wallet balance (from AccountsCollection)
func (p *PaymentService) GetBalance(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	userID := utils.GetUserIDFromRequest(r)

	var acc struct {
		CachedBalance float64 `bson:"cached_balance"`
	}
	err := db.AccountsCollection.FindOne(ctx, bson.M{"userid": userID}).Decode(&acc)
	if err != nil {
		log.Printf("GetBalance: account not found for user %s, err=%v\n", userID, err)
		http.Error(w, "account not found", http.StatusNotFound)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{"balance": acc.CachedBalance})
}

// ListTransactions returns paginated wallet transactions for the logged-in user
func (p *PaymentService) ListTransactions(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	userID := utils.GetUserIDFromRequest(r)

	// Parse query parameters
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	skip, _ := strconv.Atoi(r.URL.Query().Get("skip"))
	if skip < 0 {
		skip = 0
	}

	findOptions := options.Find().
		SetSort(bson.M{"created_at": -1}).
		SetLimit(int64(limit)).
		SetSkip(int64(skip))

	cur, err := db.TransactionCollection.Find(ctx, bson.M{"userid": userID}, findOptions)
	if err != nil {
		log.Printf("ListTransactions: DB error for user %s, err=%v\n", userID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer cur.Close(ctx)

	var txns []models.Transaction
	if err = cur.All(ctx, &txns); err != nil {
		log.Printf("ListTransactions: decode error for user %s, err=%v\n", userID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"transactions": txns,
	})
}

// ===== Redis-based per-user lock =====

// AcquireLock tries to acquire a distributed lock for a user
func (p *PaymentService) AcquireLock(ctx context.Context, userID string, ttl time.Duration) (bool, error) {
	key := "wallet_lock:" + userID
	ok, err := p.rdx.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

// ReleaseLock releases the lock
func (p *PaymentService) ReleaseLock(ctx context.Context, userID string) {
	key := "wallet_lock:" + userID
	if err := p.rdx.Del(ctx, key).Err(); err != nil {
		log.Printf("ReleaseLock: failed for user %s, err=%v\n", userID, err)
	}
}
