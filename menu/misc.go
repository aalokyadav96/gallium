package menu

import (
	"context"
	"encoding/json"
	"fmt"
	"naevis/db"
	"naevis/models"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Fetch a single menu item
func GetStock(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	placeID := ps.ByName("placeid")
	menuID := ps.ByName("menuid")

	// collection := client.Database("placedb").Collection("menu")
	var menu models.Menu
	err := db.MenuCollection.FindOne(context.TODO(), bson.M{"placeid": placeID, "menuid": menuID}).Decode(&menu)
	if err != nil {
		http.Error(w, fmt.Sprintf("Menu not found: %v", err), http.StatusNotFound)
		return
	}

	// Respond with menu data
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(menu)
}

func BuyMenu(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	placeID := ps.ByName("placeid")
	menuID := ps.ByName("menuid")

	// Parse request body
	var body struct {
		Quantity int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Quantity <= 0 {
		http.Error(w, "Invalid quantity", http.StatusBadRequest)
		return
	}

	// Atomically check stock and decrement it
	filter := bson.M{
		"placeid": placeID,
		"menuid":  menuID,
		"stock":   bson.M{"$gte": body.Quantity}, // Ensure enough stock
	}
	update := bson.M{
		"$inc": bson.M{"stock": -body.Quantity},
		"$set": bson.M{"updated_at": time.Now()},
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var updatedMenu models.Menu
	err := db.MenuCollection.FindOneAndUpdate(context.TODO(), filter, update, opts).Decode(&updatedMenu)
	if err != nil {
		http.Error(w, "Insufficient stock or menu not found", http.StatusConflict)
		return
	}

	// Respond with remaining stock
	resp := map[string]interface{}{
		"success":        true,
		"remainingStock": updatedMenu.Stock,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
