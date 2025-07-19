package cart

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"naevis/db"
	"naevis/models"
	"naevis/utils"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AddToCart increments quantity if the item exists, or inserts a new CartItem.
func AddToCart(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var item models.CartItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		log.Println("AddToCart decode error:", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Must be logged in
	userID := utils.GetUserIDFromRequest(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	item.UserID = userID

	// Validate required fields
	if item.Item == "" || item.Category == "" || item.Quantity <= 0 || item.Price <= 0 {
		http.Error(w, "Missing or invalid fields", http.StatusBadRequest)
		return
	}

	// Upsert: increment quantity if same user/item/farm/category exists
	filter := bson.M{
		"userId":   item.UserID,
		"item":     item.Item,
		"farm":     item.Farm,
		"farmid":   item.FarmId,
		"category": item.Category,
	}
	update := bson.M{
		"$inc": bson.M{"quantity": item.Quantity},
		"$setOnInsert": bson.M{
			"price":   item.Price,
			"unit":    item.Unit,
			"addedAt": time.Now(),
		},
	}
	opts := options.Update().SetUpsert(true)

	if _, err := db.CartCollection.UpdateOne(ctx, filter, update, opts); err != nil {
		log.Println("AddToCart UpdateOne error:", err)
		http.Error(w, "Failed to add to cart", http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, http.StatusCreated, map[string]string{"status": "added"})
}

// GetCart returns all cart items for the user, optional ?category= filter
func GetCart(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	userID := utils.GetUserIDFromRequest(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Build filter
	filter := bson.M{"userId": userID}
	if cat := r.URL.Query().Get("category"); cat != "" {
		filter["category"] = cat
	}

	cursor, err := db.CartCollection.Find(ctx, filter)
	if err != nil {
		log.Println("GetCart Find error:", err)
		http.Error(w, "Could not retrieve cart", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var items []models.CartItem
	if err := cursor.All(ctx, &items); err != nil {
		log.Println("GetCart cursor.All error:", err)
		http.Error(w, "Error reading cart data", http.StatusInternalServerError)
		return
	}

	if len(items) == 0 {
		items = []models.CartItem{}
	}

	utils.RespondWithJSON(w, http.StatusOK, items)
	// utils.RespondWithJSONWithHeader(w, http.StatusOK, items, map[string]string{
	// 	"Content-Type": "application/json",
	// })
}

// UpdateCart replaces *all* items in a given category for the user
func UpdateCart(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var payload struct {
		Category string            `json:"category"`
		Items    []models.CartItem `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Println("UpdateCart decode error:", err)
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}
	if payload.Category == "" {
		http.Error(w, "Category is required", http.StatusBadRequest)
		return
	}

	userID := utils.GetUserIDFromRequest(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Delete existing items in this category
	if _, err := db.CartCollection.DeleteMany(ctx, bson.M{
		"userId":   userID,
		"category": payload.Category,
	}); err != nil {
		log.Println("UpdateCart DeleteMany error:", err)
		http.Error(w, "Failed to clear existing cart items", http.StatusInternalServerError)
		return
	}

	// Insert the new items
	if len(payload.Items) > 0 {
		now := time.Now()
		docs := make([]interface{}, 0, len(payload.Items))
		for _, it := range payload.Items {
			it.UserID = userID
			it.Category = payload.Category
			it.AddedAt = now
			docs = append(docs, it)
		}
		if _, err := db.CartCollection.InsertMany(ctx, docs); err != nil {
			log.Println("UpdateCart InsertMany error:", err)
			http.Error(w, "Failed to update cart", http.StatusInternalServerError)
			return
		}
	}

	utils.RespondWithJSON(w, http.StatusCreated, map[string]string{"status": "updated"})
}

// InitiateCheckout is a placeholder for any pre-checkout locking or analytics
func InitiateCheckout(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// no-op for now
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"status": "checkout_initiated"})
}

// CreateCheckoutSession accepts cart/session details and returns a session object
func CreateCheckoutSession(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	_, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var session models.CheckoutSession
	if err := json.NewDecoder(r.Body).Decode(&session); err != nil {
		log.Println("CreateCheckoutSession decode error:", err)
		http.Error(w, "Invalid session data", http.StatusBadRequest)
		return
	}

	userID := utils.GetUserIDFromRequest(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	session.UserID = userID
	session.CreatedAt = time.Now()

	// TODO: persist session in Redis or DB if needed
	utils.RespondWithJSON(w, http.StatusCreated, session)
}

// PlaceOrder records a finalized order, clears the cart for the user
func PlaceOrder(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var order models.Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		log.Println("PlaceOrder decode error:", err)
		http.Error(w, "Invalid order payload", http.StatusBadRequest)
		return
	}

	userID := utils.GetUserIDFromRequest(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	order.UserID = userID
	order.OrderID = "ORD" + strconv.FormatInt(time.Now().UnixNano()%1e6, 10)
	order.CreatedAt = time.Now()

	if order.Status == "" {
		order.Status = "pending"
	}
	if order.ApprovedBy == nil {
		order.ApprovedBy = []string{}
	}

	if _, err := db.OrderCollection.InsertOne(ctx, order); err != nil {
		log.Println("PlaceOrder InsertOne error:", err)
		http.Error(w, "Order creation failed", http.StatusInternalServerError)
		return
	}

	// Optionally clear the user’s cart:
	/**/
	if _, err := db.CartCollection.DeleteMany(ctx, bson.M{"userId": userID}); err != nil {
		log.Println("PlaceOrder Cart cleanup error:", err)
	}
	/**/

	utils.RespondWithJSON(w, http.StatusCreated, order)
}

// package cart

// import (
// 	"context"
// 	"encoding/json"
// 	"naevis/db"
// 	"naevis/models"
// 	"naevis/utils"
// 	"net/http"
// 	"strconv"
// 	"time"

// 	"github.com/julienschmidt/httprouter"
// 	"go.mongodb.org/mongo-driver/bson"
// )

// func AddToCart(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
// 	defer cancel()

// 	var item models.CartItem
// 	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
// 		http.Error(w, "Invalid payload", http.StatusBadRequest)
// 		return
// 	}

// 	item.UserID = utils.GetUserIDFromContext(r.Context())
// 	if item.UserID == "" {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}

// 	_, err := db.CartCollection.InsertOne(ctx, item)
// 	if err != nil {
// 		http.Error(w, "Failed to add to cart", http.StatusInternalServerError)
// 		return
// 	}

// 	utils.RespondWithJSON(w, 201, "")
// }

// func GetCart(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
// 	defer cancel()

// 	userID := utils.GetUserIDFromContext(r.Context())
// 	cursor, err := db.CartCollection.Find(ctx, bson.M{"userId": userID})
// 	if err != nil {
// 		http.Error(w, "Could not retrieve cart", http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(ctx)

// 	var items []models.CartItem
// 	if err := cursor.All(ctx, &items); err != nil {
// 		http.Error(w, "Error reading cart data", http.StatusInternalServerError)
// 		return
// 	}

// 	utils.RespondWithJSON(w, 201, items)
// }

// func UpdateCart(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
// 	defer cancel()

// 	var payload struct {
// 		Category string            `json:"category"`
// 		Items    []models.CartItem `json:"items"`
// 	}
// 	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
// 		http.Error(w, "Invalid request", http.StatusBadRequest)
// 		return
// 	}

// 	userID := utils.GetUserIDFromContext(r.Context())
// 	if _, err := db.CartCollection.DeleteMany(ctx, bson.M{"userId": userID, "category": payload.Category}); err != nil {
// 		http.Error(w, "Failed to clear existing cart items", http.StatusInternalServerError)
// 		return
// 	}

// 	if len(payload.Items) > 0 {
// 		var docs []interface{}
// 		for _, item := range payload.Items {
// 			item.UserID = userID
// 			item.Category = payload.Category
// 			docs = append(docs, item)
// 		}
// 		if _, err := db.CartCollection.InsertMany(ctx, docs); err != nil {
// 			http.Error(w, "Failed to update cart", http.StatusInternalServerError)
// 			return
// 		}
// 	}

// 	utils.RespondWithJSON(w, 201, "")
// }

// func InitiateCheckout(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	// This can be a placeholder for locking or tagging the cart if needed
// 	utils.RespondWithJSON(w, 201, "")
// }

// func CreateCheckoutSession(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	_, cancel := context.WithTimeout(r.Context(), 10*time.Second)
// 	defer cancel()

// 	var session models.CheckoutSession
// 	if err := json.NewDecoder(r.Body).Decode(&session); err != nil {
// 		http.Error(w, "Invalid session data", http.StatusBadRequest)
// 		return
// 	}

// 	session.UserID = utils.GetUserIDFromContext(r.Context())
// 	session.CreatedAt = time.Now()

// 	// You may store this session in Redis or return it directly
// 	utils.RespondWithJSON(w, 201, session)
// }

// func PlaceOrder(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
// 	defer cancel()

// 	var order models.Order
// 	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
// 		http.Error(w, "Invalid order", http.StatusBadRequest)
// 		return
// 	}

// 	order.UserID = utils.GetUserIDFromContext(r.Context())
// 	order.OrderID = "ORD" + strconv.FormatInt(time.Now().UnixNano()%1e6, 10)
// 	order.CreatedAt = time.Now()

// 	_, err := db.OrderCollection.InsertOne(ctx, order)
// 	if err != nil {
// 		http.Error(w, "Order creation failed", http.StatusInternalServerError)
// 		return
// 	}

// 	utils.RespondWithJSON(w, 201, order)
// }
