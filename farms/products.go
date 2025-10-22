package farms

import (
	"context"
	"encoding/json"
	"fmt"
	"naevis/db"
	"naevis/models"
	"naevis/mq"
	"naevis/utils"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func CreateProduct(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	createItem(w, r, "product")
}

func CreateTool(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	createItem(w, r, "tool")
}

func createItem(w http.ResponseWriter, r *http.Request, itemType string) {
	// item, err := parseProductForm(r, itemType)
	item, err := parseProductJSON(r, itemType)
	if err != nil {
		http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	item.ProductID = utils.GenerateRandomString(13)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	_, err = db.ProductCollection.InsertOne(ctx, item)
	if err != nil {
		http.Error(w, "Failed to insert item", http.StatusInternalServerError)
		return
	}

	go mq.Emit(ctx, "farmitem-created", models.Index{EntityType: "product", EntityId: item.ProductID, Method: "POST"})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(item); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func updateItem(w http.ResponseWriter, r *http.Request, ps httprouter.Params, itemType string) {
	idParam := ps.ByName("id")
	if idParam == "" {
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}

	// item, err := parseProductForm(r, itemType)
	item, err := parseProductJSON(r, itemType)
	if err != nil {
		http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	update := bson.M{"$set": item}
	_, err = db.ProductCollection.UpdateOne(ctx, bson.M{"productid": idParam}, update)
	if err != nil {
		http.Error(w, "Failed to update item", http.StatusInternalServerError)
		return
	}

	go mq.Emit(ctx, "farmitem-updated", models.Index{EntityType: "product", EntityId: idParam, Method: "PUT"})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "updated"}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func UpdateTool(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	updateItem(w, r, ps, "tool")
}

func UpdateProduct(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	updateItem(w, r, ps, "product")
}

func DeleteProduct(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteItem(w, r, ps)
}

func DeleteTool(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteItem(w, r, ps)
}

func deleteItem(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	idParam := ps.ByName("id")
	if idParam == "" {
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	_, err := db.ProductCollection.DeleteOne(ctx, bson.M{"productid": idParam})
	if err != nil {
		http.Error(w, "Failed to delete item", http.StatusInternalServerError)
		return
	}

	go mq.Emit(ctx, "farmitem-deleted", models.Index{EntityType: "product", EntityId: idParam, Method: "DELETE"})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "deleted"}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// parseProductJSON parses a JSON body into models.Product.
func parseProductJSON(r *http.Request, itemType string) (models.Product, error) {
	var payload struct {
		Name          string   `json:"name"`
		Description   string   `json:"description"`
		Category      string   `json:"category"`
		SKU           string   `json:"sku"`
		Unit          string   `json:"unit"`
		Type          string   `json:"type"`
		Featured      bool     `json:"featured"`
		Price         float64  `json:"price"`
		Quantity      float64  `json:"quantity"`
		AvailableFrom string   `json:"availableFrom"`
		AvailableTo   string   `json:"availableTo"`
		Images        []string `json:"images"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return models.Product{}, fmt.Errorf("invalid JSON: %w", err)
	}

	item := models.Product{
		Name:        payload.Name,
		Description: payload.Description,
		Category:    payload.Category,
		SKU:         payload.SKU,
		Unit:        payload.Unit,
		Type:        itemType,
		Featured:    payload.Featured,
		Price:       payload.Price,
		Quantity:    payload.Quantity,
		Images:      payload.Images,
	}

	// Parse date fields safely
	if payload.AvailableFrom != "" {
		if t, err := time.Parse("2006-01-02", payload.AvailableFrom); err == nil {
			item.AvailableFrom = &models.SafeTime{Time: t}
		}
	}
	if payload.AvailableTo != "" {
		if t, err := time.Parse("2006-01-02", payload.AvailableTo); err == nil {
			item.AvailableTo = &models.SafeTime{Time: t}
		}
	}

	return item, nil
}
