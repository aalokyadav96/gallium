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
// Returns the updated grouped cart.
func AddToCart(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var item models.CartItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		log.Println("AddToCart decode error:", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	userID := utils.GetUserIDFromRequest(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	item.UserID = userID

	if item.ItemId == "" || item.ItemName == "" || item.Category == "" || item.Quantity <= 0 || item.Price <= 0 {
		http.Error(w, "Missing or invalid fields", http.StatusBadRequest)
		return
	}

	filter := bson.M{
		"userId":     item.UserID,
		"itemId":     item.ItemId,
		"entityId":   item.EntityId,
		"entityType": item.EntityType,
		"category":   item.Category,
	}
	update := bson.M{
		"$inc": bson.M{"quantity": item.Quantity},
		"$setOnInsert": bson.M{
			"itemName":   item.ItemName,
			"itemType":   item.ItemType,
			"price":      item.Price,
			"unit":       item.Unit,
			"entityName": item.EntityName,
			"addedAt":    time.Now(),
		},
	}
	opts := options.Update().SetUpsert(true)

	if _, err := db.CartCollection.UpdateOne(ctx, filter, update, opts); err != nil {
		log.Println("AddToCart UpdateOne error:", err)
		http.Error(w, "Failed to add to cart", http.StatusInternalServerError)
		return
	}

	groupedCart, err := getGroupedCart(ctx, userID)
	if err != nil {
		http.Error(w, "Failed to fetch updated cart", http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, http.StatusCreated, groupedCart)
}

// UpdateCart replaces all items in a given category for the user.
// Returns the updated grouped cart.
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

	groupedCart, err := getGroupedCart(ctx, userID)
	if err != nil {
		http.Error(w, "Failed to fetch updated cart", http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, http.StatusCreated, groupedCart)
}

// GetCart returns all cart items for the user, optional ?category= filter,
// grouped by category as map[string][]CartItem
func GetCart(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	userID := utils.GetUserIDFromRequest(r)
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	groupedCart, err := getGroupedCart(ctx, userID)
	if err != nil {
		http.Error(w, "Failed to fetch cart", http.StatusInternalServerError)
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, groupedCart)
}

// InitiateCheckout is a placeholder for any pre-checkout locking or analytics
func InitiateCheckout(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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

	utils.RespondWithJSON(w, http.StatusCreated, session)
}

// PlaceOrder records a finalized order, clears the cart for the user,
// and stores the current cart grouped by category in Order.Items
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

	// Fetch the latest cart and store it in the order
	cartItems, err := getGroupedCart(ctx, userID)
	if err != nil {
		http.Error(w, "Failed to fetch cart for order", http.StatusInternalServerError)
		return
	}
	order.Items = cartItems

	if _, err := db.OrderCollection.InsertOne(ctx, order); err != nil {
		log.Println("PlaceOrder InsertOne error:", err)
		http.Error(w, "Order creation failed", http.StatusInternalServerError)
		return
	}

	// Clear the user’s cart
	if _, err := db.CartCollection.DeleteMany(ctx, bson.M{"userId": userID}); err != nil {
		log.Println("PlaceOrder Cart cleanup error:", err)
	}

	utils.RespondWithJSON(w, http.StatusCreated, order)
}

// getGroupedCart fetches all cart items for a user and groups them by category
func getGroupedCart(ctx context.Context, userID string) (map[string][]models.CartItem, error) {
	filter := bson.M{"userId": userID}
	cursor, err := db.CartCollection.Find(ctx, filter)
	if err != nil {
		log.Println("getGroupedCart Find error:", err)
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []models.CartItem
	if err := cursor.All(ctx, &items); err != nil {
		log.Println("getGroupedCart cursor.All error:", err)
		return nil, err
	}

	grouped := make(map[string][]models.CartItem)
	for _, item := range items {
		grouped[item.Category] = append(grouped[item.Category], item)
	}
	return grouped, nil
}
