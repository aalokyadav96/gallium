package farms

import (
	"context"
	"encoding/json"
	"fmt"
	"naevis/db"
	"naevis/filemgr"
	"naevis/models"
	"naevis/mq"
	"naevis/utils"
	"net/http"
	"strconv"
	"strings"
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
	item, err := parseProductForm(r, itemType)
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

	item, err := parseProductForm(r, itemType)
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

// parseProductForm parses multipart form into models.Product including image saving.
func parseProductForm(r *http.Request, itemType string) (models.Product, error) {
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		return models.Product{}, err
	}

	item := models.Product{
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		Category:    r.FormValue("category"),
		SKU:         r.FormValue("sku"),
		Unit:        r.FormValue("unit"),
		Type:        itemType,
		Featured:    r.FormValue("featured") == "true" || r.FormValue("featured") == "on",
	}

	if price, err := strconv.ParseFloat(r.FormValue("price"), 64); err == nil {
		item.Price = price
	}
	if quantity, err := strconv.ParseFloat(r.FormValue("quantity"), 64); err == nil {
		item.Quantity = quantity
	}
	if date := r.FormValue("availableFrom"); date != "" {
		if t, err := time.Parse("2006-01-02", date); err == nil {
			item.AvailableFrom = &models.SafeTime{Time: t}
		}
	}
	if date := r.FormValue("availableTo"); date != "" {
		if t, err := time.Parse("2006-01-02", date); err == nil {
			item.AvailableTo = &models.SafeTime{Time: t}
		}
	}

	if r.MultipartForm == nil {
		return item, fmt.Errorf("multipart form missing")
	}

	imageKeys := []string{}
	for key := range r.MultipartForm.File {
		if strings.HasPrefix(key, "images_") {
			imageKeys = append(imageKeys, key)
		}
	}

	if len(imageKeys) > 0 {
		files, err := filemgr.SaveFormFilesByKeys(r.MultipartForm, imageKeys, filemgr.EntityProduct, filemgr.PicPhoto, false)
		if err != nil {
			return item, err
		}
		item.ImageURLs = files
	} else {
		item.ImageURLs = []string{}
	}

	return item, nil
}
