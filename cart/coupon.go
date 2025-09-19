package cart

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"naevis/db"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

type Coupon struct {
	Code      string    `bson:"code" json:"code"`
	Discount  float64   `bson:"discount" json:"discount"` // % value e.g. 10 means 10%
	ExpiresAt time.Time `bson:"expiresAt" json:"expiresAt"`
	Active    bool      `bson:"active" json:"active"`
}

type CouponRequest struct {
	Code string  `json:"code"`
	Cart float64 `json:"cart"` // cart subtotal (optional, for min spend rules)
}

type CouponResponse struct {
	Valid    bool    `json:"valid"`
	Discount float64 `json:"discount"` // absolute amount, not %
	Message  string  `json:"message"`
}

// func ValidateCouponHandler(db *mongo.Database) httprouter.Handle {
// return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
func ValidateCouponHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var req CouponRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	code := strings.TrimSpace(strings.ToLower(req.Code))
	if code == "" {
		writeJSON(w, CouponResponse{Valid: false, Message: "No coupon provided"})
		return
	}

	var coupon Coupon
	err := db.CouponCollection.FindOne(context.TODO(), bson.M{"code": code}).Decode(&coupon)
	if err != nil {
		writeJSON(w, CouponResponse{Valid: false, Message: "Coupon not found"})
		return
	}

	// validate
	if !coupon.Active {
		writeJSON(w, CouponResponse{Valid: false, Message: "Coupon inactive"})
		return
	}
	if time.Now().After(coupon.ExpiresAt) {
		writeJSON(w, CouponResponse{Valid: false, Message: "Coupon expired"})
		return
	}

	// calculate discount (flat %)
	discount := 0.0
	if req.Cart > 0 {
		discount = (req.Cart * coupon.Discount) / 100
	}

	writeJSON(w, CouponResponse{
		Valid:    true,
		Discount: discount,
		Message:  "Coupon applied successfully",
	})
}

// }
// }

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
