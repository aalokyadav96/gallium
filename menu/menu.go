package menu

import (
	"context"
	"encoding/json"
	"fmt"
	"naevis/db"
	"naevis/models"
	"naevis/mq"
	"naevis/rdx"
	"naevis/utils"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func CreateMenu(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	placeID := ps.ByName("placeid")

	if placeID == "" {
		http.Error(w, "Place ID is required", http.StatusBadRequest)
		return
	}

	var body struct {
		Name    string  `json:"name"`
		Price   float64 `json:"price"`
		Stock   int     `json:"stock"`
		MenuPic string  `json:"menu_pic"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(body.Name) == 0 || len(body.Name) > 100 {
		http.Error(w, "Name must be between 1 and 100 characters.", http.StatusBadRequest)
		return
	}
	if body.Price < 0 {
		http.Error(w, "Invalid price value. Must be a non-negative number.", http.StatusBadRequest)
		return
	}
	if body.Stock < 0 {
		http.Error(w, "Invalid stock value. Must be a non-negative integer.", http.StatusBadRequest)
		return
	}

	menuID := utils.GenerateRandomString(14)

	menu := models.Menu{
		PlaceID:   placeID,
		Name:      body.Name,
		Price:     body.Price,
		Stock:     body.Stock,
		MenuID:    menuID,
		MenuPhoto: body.MenuPic,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if _, err := db.MenuCollection.InsertOne(ctx, menu); err != nil {
		http.Error(w, "Failed to insert menu: "+err.Error(), http.StatusInternalServerError)
		return
	}

	go mq.Emit(ctx, "menu-created", models.Index{
		EntityType: "menu", EntityId: menu.MenuID, Method: "POST", ItemType: "place", ItemId: placeID,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"message": "Menu created successfully.",
		"data":    menu,
	})
}

// Fetch a single menu item
func GetMenu(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	placeID := ps.ByName("placeid")
	menuID := ps.ByName("menuid")
	cacheKey := fmt.Sprintf("menu:%s:%s", placeID, menuID)

	// Check if the menu is cached
	cachedMenu, err := rdx.RdxGet(cacheKey)
	if err == nil && cachedMenu != "" {
		// Return cached data
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cachedMenu))
		return
	}

	// collection := client.Database("placedb").Collection("menu")
	var menu models.Menu
	err = db.MenuCollection.FindOne(context.TODO(), bson.M{"placeid": placeID, "menuid": menuID}).Decode(&menu)
	if err != nil {
		http.Error(w, fmt.Sprintf("Menu not found: %v", err), http.StatusNotFound)
		return
	}

	// Cache the result
	menuJSON, _ := json.Marshal(menu)
	rdx.RdxSet(cacheKey, string(menuJSON))

	// Respond with menu data
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(menu)
}

// Menus
func GetMenus(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	filter := bson.M{"placeid": ps.ByName("placeid")}
	menus, err := utils.FindAndDecode[models.Menu](ctx, db.MenuCollection, filter)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch menus")
		return
	}
	if len(menus) == 0 {
		menus = []models.Menu{}
	}

	utils.RespondWithJSON(w, http.StatusOK, menus)
}

// Edit a menu item
func EditMenu(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	placeID := ps.ByName("placeid")
	menuID := ps.ByName("menuid")

	// Parse the request body
	var menu models.Menu
	if err := json.NewDecoder(r.Body).Decode(&menu); err != nil {
		http.Error(w, "Invalid input data", http.StatusBadRequest)
		return
	}

	// Validate menu data
	if menu.Name == "" || menu.Price <= 0 || menu.Stock < 0 {
		http.Error(w, "Invalid menu data: Name, Price, and Stock are required.", http.StatusBadRequest)
		return
	}

	// Prepare update data
	updateFields := bson.M{}
	if menu.Name != "" {
		updateFields["name"] = menu.Name
	}
	if menu.Price > 0 {
		updateFields["price"] = menu.Price
	}
	if menu.Stock >= 0 {
		updateFields["stock"] = menu.Stock
	}

	// Update the menu in MongoDB
	// collection := client.Database("placedb").Collection("menu")
	updateResult, err := db.MenuCollection.UpdateOne(
		context.TODO(),
		bson.M{"placeid": placeID, "menuid": menuID},
		bson.M{"$set": updateFields},
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to update menu: %v", err), http.StatusInternalServerError)
		return
	}

	// Check if update was successful
	if updateResult.MatchedCount == 0 {
		http.Error(w, "Menu not found", http.StatusNotFound)
		return
	}

	// Invalidate the specific menu cache
	rdx.RdxDel(fmt.Sprintf("menu:%s:%s", placeID, menuID))

	m := models.Index{EntityType: "menu", EntityId: menuID, Method: "PUT", ItemType: "place", ItemId: placeID}
	go mq.Emit(ctx, "menu-edited", m)

	// Send response
	// w.Header().Set("Content-Type", "application/json")
	// w.WriteHeader(http.StatusOK)
	// json.NewEncoder(w).Encode("Menu updated successfully")
	// Respond with success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Menu updated successfully",
	})
}

// Delete a menu item
func DeleteMenu(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	placeID := ps.ByName("placeid")
	menuID := ps.ByName("menuid")

	// Delete the menu from MongoDB
	// collection := client.Database("placedb").Collection("menu")
	deleteResult, err := db.MenuCollection.DeleteOne(context.TODO(), bson.M{"placeid": placeID, "menuid": menuID})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete menu: %v", err), http.StatusInternalServerError)
		return
	}

	// Check if delete was successful
	if deleteResult.DeletedCount == 0 {
		http.Error(w, "Menu not found", http.StatusNotFound)
		return
	}

	// Invalidate the cache
	rdx.RdxDel(fmt.Sprintf("menu:%s:%s", placeID, menuID))

	m := models.Index{EntityType: "menu", EntityId: menuID, Method: "DELETE", ItemType: "place", ItemId: placeID}
	go mq.Emit(ctx, "menu-deleted", m)

	// // Send response
	// w.WriteHeader(http.StatusOK)
	// w.Write([]byte("Menu deleted successfully"))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Menu updated successfully",
	})
}
