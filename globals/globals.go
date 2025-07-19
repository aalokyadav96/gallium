package globals

import (
	"context"
	"naevis/db"
	"naevis/rdx"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	RefreshTokenTTL = 7 * 24 * time.Hour // 7 days
	AccessTokenTTL  = 15 * time.Minute   // 15 minutes
)

var (
	// tokenSigningAlgo = jwt.SigningMethodHS256
	JwtSecret = []byte("your_secret_key") // Replace with a secure secret key
)

type ContextKey string

const UserIDKey ContextKey = "userId"

var CTX = context.Background()

var RedisClient *redis.Client = rdx.Conn
var MongoClient *mongo.Client = db.Client
