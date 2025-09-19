package pay

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"naevis/db"
	"naevis/models"
	"naevis/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// InitIdempotencyIndexes creates the necessary indexes (unique key + TTL).
func InitIdempotencyIndexes(ctx context.Context) error {
	idxs := []mongo.IndexModel{
		{
			Keys:    bson.M{"key": 1},
			Options: options.Index().SetUnique(true).SetName("unique_key"),
		},
		{
			Keys:    bson.M{"expires_at": 1},
			Options: options.Index().SetExpireAfterSeconds(0).SetName("ttl_expires_at"),
		},
	}
	_, err := db.IdempotencyCollection.Indexes().CreateMany(ctx, idxs)
	return err
}

func computeRequestHash(r *http.Request, bodyBytes []byte, userID string) string {
	h := sha256.New()
	h.Write([]byte(r.Method + ":" + r.URL.Path + ":" + userID + ":"))
	h.Write(bodyBytes)
	return hex.EncodeToString(h.Sum(nil))
}

// CaptureResponseWriter wraps http.ResponseWriter to capture status and body.
type CaptureResponseWriter struct {
	w           http.ResponseWriter
	statusCode  int
	buf         bytes.Buffer
	header      http.Header
	wroteHeader bool
}

func NewCaptureResponseWriter(w http.ResponseWriter) *CaptureResponseWriter {
	return &CaptureResponseWriter{
		w:          w,
		statusCode: http.StatusOK,
		header:     make(http.Header),
	}
}

func (c *CaptureResponseWriter) Header() http.Header {
	return c.w.Header()
}

func (c *CaptureResponseWriter) WriteHeader(statusCode int) {
	if !c.wroteHeader {
		c.statusCode = statusCode
		c.w.WriteHeader(statusCode)
		c.wroteHeader = true
	}
}

func (c *CaptureResponseWriter) Write(b []byte) (int, error) {
	c.buf.Write(b)
	return c.w.Write(b)
}

func (c *CaptureResponseWriter) Status() int {
	return c.statusCode
}

func (c *CaptureResponseWriter) BodyBytes() []byte {
	return c.buf.Bytes()
}

// helper to detect duplicate key errors from Mongo insert
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	if we, ok := err.(mongo.WriteException); ok {
		for _, e := range we.WriteErrors {
			if e.Code == 11000 {
				return true
			}
		}
	}
	return false
}

// IdempotencyMiddleware ensures safe replay behavior for mutating endpoints when client provides Idempotency-Key.
// Behavior:
// - If no header: pass-through.
// - If header present: compute request hash and try to insert a placeholder record (no response).
//   - If insert succeeds: let handler run; capture response and update record with response.
//   - If insert fails with duplicate key: fetch existing record:
//   - if request hash mismatches -> 409 Conflict
//   - if response available -> return cached response
//   - if response not available -> let handler run (handler should be idempotent at DB level, we also ensure external_ref = Idempotency-Key)
func IdempotencyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}

		userID := utils.GetUserIDFromRequest(r)

		// Limit body size to 1 MB to prevent memory issues
		bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		reqHash := computeRequestHash(r, bodyBytes, userID)
		now := time.Now()
		rec := models.IdempotencyRecord{
			Key:         key,
			Method:      r.Method,
			Path:        r.URL.Path,
			UserID:      userID,
			RequestHash: reqHash,
			CreatedAt:   now,
			ExpiresAt:   now.Add(24 * time.Hour),
		}

		ctx := r.Context()
		_, err = db.IdempotencyCollection.InsertOne(ctx, rec)
		if err == nil {
			// First time: capture response
			crw := NewCaptureResponseWriter(w)
			next.ServeHTTP(crw, r)

			var parsed interface{}
			if err := json.Unmarshal(crw.BodyBytes(), &parsed); err != nil {
				parsed = string(crw.BodyBytes()) // fallback to raw body
			}

			responseObj := map[string]interface{}{
				"status": crw.Status(),
				"body":   parsed,
			}

			_, _ = db.IdempotencyCollection.UpdateOne(ctx,
				bson.M{"key": key},
				bson.M{"$set": bson.M{"response": responseObj}},
			)
			return
		}

		// Check for duplicate key
		if !isDuplicateKeyError(err) {
			http.Error(w, "idempotency lookup error", http.StatusInternalServerError)
			return
		}

		// Fetch existing record
		var existing models.IdempotencyRecord
		if err := db.IdempotencyCollection.FindOne(ctx, bson.M{"key": key}).Decode(&existing); err != nil {
			http.Error(w, "idempotency lookup error", http.StatusInternalServerError)
			return
		}

		// Request hash mismatch -> conflict
		if existing.RequestHash != reqHash {
			http.Error(w, "idempotency-key conflict", http.StatusConflict)
			return
		}

		// Return cached response if available
		if existing.Response != nil {
			statusFloat, _ := existing.Response["status"].(float64)
			status := int(statusFloat)
			body := existing.Response["body"]
			utils.RespondWithJSON(w, status, body)
			return
		}

		// In-flight request, let handler run
		next.ServeHTTP(w, r)
	})
}
