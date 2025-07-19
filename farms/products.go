package farms

import (
	"context"
	"encoding/json"
	"naevis/db"
	"naevis/models"
	"naevis/utils"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetItems(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	itemType := r.URL.Query().Get("type")     // "product" or "tool"
	search := r.URL.Query().Get("search")     // search text
	category := r.URL.Query().Get("category") // filter by category
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	sortParam := r.URL.Query().Get("sort") // e.g. price_asc, name_desc

	limit := int64(10)
	offset := int64(0)

	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = int64(l)
	}
	if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
		offset = int64(o)
	}

	filter := bson.M{}
	if itemType != "" {
		filter["type"] = itemType
	}
	if category != "" {
		filter["category"] = category
	}
	if search != "" {
		filter["name"] = bson.M{"$regex": primitive.Regex{Pattern: search, Options: "i"}}
	}

	// Determine sort order
	sort := bson.D{{Key: "name", Value: 1}} // default
	switch sortParam {
	case "price_asc":
		sort = bson.D{{Key: "price", Value: 1}}
	case "price_desc":
		sort = bson.D{{Key: "price", Value: -1}}
	case "name_asc":
		sort = bson.D{{Key: "name", Value: 1}}
	case "name_desc":
		sort = bson.D{{Key: "name", Value: -1}}
	}

	findOptions := options.Find().
		SetLimit(limit).
		SetSkip(offset).
		SetSort(sort)

	cursor, err := db.ProductCollection.Find(ctx, filter, findOptions)
	if err != nil {
		http.Error(w, "Failed to fetch items", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var items []models.Product
	if err := cursor.All(ctx, &items); err != nil {
		http.Error(w, "Failed to decode items", http.StatusInternalServerError)
		return
	}
	if len(items) == 0 {
		items = []models.Product{}
	}

	count, err := db.ProductCollection.CountDocuments(ctx, filter)
	if err != nil {
		http.Error(w, "Failed to count items", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"items": items,
		"total": count,
	})
}

// func GetItems(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	// Query parameters
// 	itemType := r.URL.Query().Get("type")     // "product" or "tool"
// 	search := r.URL.Query().Get("search")     // search text
// 	category := r.URL.Query().Get("category") // filter by category
// 	limitStr := r.URL.Query().Get("limit")
// 	offsetStr := r.URL.Query().Get("offset")

// 	limit := int64(10)
// 	offset := int64(0)

// 	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
// 		limit = int64(l)
// 	}
// 	if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
// 		offset = int64(o)
// 	}

// 	// Build filter
// 	filter := bson.M{}
// 	if itemType != "" {
// 		filter["type"] = itemType
// 	}
// 	if category != "" {
// 		filter["category"] = category
// 	}
// 	if search != "" {
// 		filter["name"] = bson.M{"$regex": primitive.Regex{Pattern: search, Options: "i"}}
// 	}

// 	findOptions := options.Find().
// 		SetLimit(limit).
// 		SetSkip(offset).
// 		SetSort(bson.M{"name": 1})

// 	cursor, err := db.ProductCollection.Find(ctx, filter, findOptions)
// 	if err != nil {
// 		http.Error(w, "Failed to fetch items", http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(ctx)

// 	var items []models.Product
// 	if err := cursor.All(ctx, &items); err != nil {
// 		http.Error(w, "Failed to decode items", http.StatusInternalServerError)
// 		return
// 	}

// 	// Total count (optional)
// 	count, err := db.ProductCollection.CountDocuments(ctx, filter)
// 	if err != nil {
// 		http.Error(w, "Failed to count items", http.StatusInternalServerError)
// 		return
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(map[string]interface{}{
// 		"items": items,
// 		"total": count,
// 	})
// }

// func GetProducts(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	cursor, err := db.ProductCollection.Find(ctx, struct{}{})
// 	if err != nil {
// 		http.Error(w, "Failed to fetch products", http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(ctx)

// 	var products []models.Product
// 	if err := cursor.All(ctx, &products); err != nil {
// 		http.Error(w, "Failed to parse products", http.StatusInternalServerError)
// 		return
// 	}

// 	json.NewEncoder(w).Encode(products)
// }
// func GetTools(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	query := bson.M{}

// 	// Optional filtering
// 	category := r.URL.Query().Get("category")
// 	if category != "" {
// 		query["category"] = category
// 	}

// 	search := r.URL.Query().Get("search")
// 	if search != "" {
// 		query["$or"] = []bson.M{
// 			{"name": bson.M{"$regex": search, "$options": "i"}},
// 			{"description": bson.M{"$regex": search, "$options": "i"}},
// 		}
// 	}

// 	// Sorting
// 	sortParam := r.URL.Query().Get("sort")
// 	sort := bson.D{} // default: no sorting

// 	switch sortParam {
// 	case "price_asc":
// 		sort = bson.D{{Key: "price", Value: 1}}
// 	case "price_desc":
// 		sort = bson.D{{Key: "price", Value: -1}}
// 	case "name_asc":
// 		sort = bson.D{{Key: "name", Value: 1}}
// 	case "name_desc":
// 		sort = bson.D{{Key: "name", Value: -1}}
// 	}

// 	findOptions := options.Find()
// 	if len(sort) > 0 {
// 		findOptions.SetSort(sort)
// 	}

// 	cursor, err := db.ProductCollection.Find(ctx, query, findOptions)
// 	if err != nil {
// 		http.Error(w, "Failed to fetch tools", http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(ctx)

// 	var tools []models.Tool
// 	if err := cursor.All(ctx, &tools); err != nil {
// 		http.Error(w, "Failed to parse tools", http.StatusInternalServerError)
// 		return
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(map[string]any{
// 		"items": tools,
// 		"total": len(tools),
// 	})
// }

func CreateProduct(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	createItem(w, r, "product")
}

func CreateTool(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	createItem(w, r, "tool")
}

func createItem(w http.ResponseWriter, r *http.Request, itemType string) {
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
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

	// Handle image uploads
	formImages := r.MultipartForm.File["images"]
	savedURLs := make([]string, 0, len(formImages))

	for _, fh := range formImages {
		file, err := fh.Open()
		if err != nil {
			continue
		}
		defer file.Close()

		filename, err := utils.SaveFile(file, fh, "./uploads")
		if err != nil {
			continue
		}

		// You can adjust this to include domain/prefix if needed
		savedURLs = append(savedURLs, filename)
	}

	item.ImageURLs = savedURLs

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := db.ProductCollection.InsertOne(ctx, item)
	if err != nil {
		http.Error(w, "Failed to insert item", http.StatusInternalServerError)
		return
	}

	item.ID = res.InsertedID.(primitive.ObjectID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

// func createItem(w http.ResponseWriter, r *http.Request, itemType string) {
// 	var item models.Product
// 	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
// 		http.Error(w, "Invalid request body", http.StatusBadRequest)
// 		return
// 	}
// 	item.Type = itemType

// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	res, err := db.ProductCollection.InsertOne(ctx, item)
// 	if err != nil {
// 		http.Error(w, "Failed to insert item", http.StatusInternalServerError)
// 		return
// 	}

// 	item.ID = res.InsertedID.(primitive.ObjectID)
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(item)
// }

func UpdateProduct(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	updateItem(w, r, ps, "product")
}

func UpdateTool(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	updateItem(w, r, ps, "tool")
}
func updateItem(w http.ResponseWriter, r *http.Request, ps httprouter.Params, itemType string) {
	idParam := ps.ByName("id")
	objID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	err = r.ParseMultipartForm(32 << 20)
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
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

	// Handle optional new images
	formImages := r.MultipartForm.File["images"]
	savedURLs := make([]string, 0, len(formImages))

	for _, fh := range formImages {
		file, err := fh.Open()
		if err != nil {
			continue
		}
		defer file.Close()

		filename, err := utils.SaveFile(file, fh, "./uploads")
		if err != nil {
			continue
		}

		savedURLs = append(savedURLs, filename)
	}

	// Only update images if any were uploaded
	if len(savedURLs) > 0 {
		item.ImageURLs = savedURLs
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{"$set": item}
	_, err = db.ProductCollection.UpdateByID(ctx, objID, update)
	if err != nil {
		http.Error(w, "Failed to update item", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// func updateItem(w http.ResponseWriter, r *http.Request, ps httprouter.Params, itemType string) {
// 	idParam := ps.ByName("id")
// 	objID, err := primitive.ObjectIDFromHex(idParam)
// 	if err != nil {
// 		http.Error(w, "Invalid ID", http.StatusBadRequest)
// 		return
// 	}

// 	var item models.Product
// 	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
// 		http.Error(w, "Invalid request body", http.StatusBadRequest)
// 		return
// 	}

// 	item.Type = itemType

// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	update := bson.M{"$set": item}

// 	_, err = db.ProductCollection.UpdateByID(ctx, objID, update)
// 	if err != nil {
// 		http.Error(w, "Failed to update item", http.StatusInternalServerError)
// 		return
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
// }

func DeleteProduct(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteItem(w, r, ps)
}

func DeleteTool(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteItem(w, r, ps)
}

func deleteItem(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	_ = r
	idParam := ps.ByName("id")
	objID, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = db.ProductCollection.DeleteOne(ctx, bson.M{"_id": objID})
	if err != nil {
		http.Error(w, "Failed to delete item", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// // GetItemCategories returns distinct categories from the items collection
// func GetItemCategories(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	// categories, err := db.FarmItemCollection.Distinct(context.TODO(), "category", bson.M{})
// 	// if err != nil {
// 	// 	http.Error(w, "Failed to fetch categories", http.StatusInternalServerError)
// 	// 	return
// 	// }
// 	categories := []string{
// 		"Spices",
// 		"Pickles",
// 		"Flour",
// 		"Oils",
// 		"Honey",
// 		"Tea & Coffee",
// 		"Dry Fruits",
// 		"Natural Sweeteners",
// 	}

// 	// // convert []interface{} to []string
// 	// result := make([]string, 0, len(categories))
// 	// for _, c := range categories {
// 	// 	if str, ok := c.(string); ok && str != "" {
// 	// 		result = append(result, str)
// 	// 	}
// 	// }

//		// json.NewEncoder(w).Encode(result)
//		json.NewEncoder(w).Encode(categories)
//	}
//
// GetItemCategories returns distinct categories from the items collection based on the type (product/tool)
func GetItemCategories(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	itemType := r.URL.Query().Get("type")

	var categories []string

	switch itemType {
	case "tool":
		categories = []string{
			"Cutting Tools",
			"Irrigation Tools",
			"Harvesting Tools",
			"Hand Tools",
			"Protective Gear",
			"Fertilizer Applicators",
		}
	case "product":
		fallthrough
	default:
		categories = []string{
			"Spices",
			"Pickles",
			"Flour",
			"Oils",
			"Honey",
			"Tea & Coffee",
			"Dry Fruits",
			"Natural Sweeteners",
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(categories)
}
