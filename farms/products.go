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
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Items
func GetItems(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	filter := bson.M{}
	if t := r.URL.Query().Get("type"); t != "" {
		filter["type"] = t
	}
	if c := r.URL.Query().Get("category"); c != "" {
		filter["category"] = c
	}
	if s := r.URL.Query().Get("search"); s != "" {
		filter["name"] = utils.RegexFilter("name", s)["name"]
	}

	skip, limit := utils.ParsePagination(r, 10, 100)
	sort := utils.ParseSort(r.URL.Query().Get("sort"), bson.D{{Key: "name", Value: 1}}, map[string]bson.D{
		"price_asc":  {{Key: "price", Value: 1}},
		"price_desc": {{Key: "price", Value: -1}},
		"name_asc":   {{Key: "name", Value: 1}},
		"name_desc":  {{Key: "name", Value: -1}},
	})

	opts := options.Find().SetSort(sort).SetSkip(skip).SetLimit(limit)
	items, err := utils.FindAndDecode[models.Product](ctx, db.ProductCollection, filter, opts)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "Failed to fetch items")
		return
	}

	total, err := db.ProductCollection.CountDocuments(ctx, filter)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "Failed to count items")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]interface{}{
		"items": items,
		"total": total,
	})
}

// func GetItems(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
// 	defer cancel()

// 	filter := bson.M{}
// 	if t := r.URL.Query().Get("type"); t != "" {
// 		filter["type"] = t
// 	}
// 	if c := r.URL.Query().Get("category"); c != "" {
// 		filter["category"] = c
// 	}
// 	if s := r.URL.Query().Get("search"); s != "" {
// 		filter["name"] = utils.RegexFilter("name", s)["name"]
// 	}

// 	skip, limit := utils.ParsePagination(r, 10, 100)
// 	sort := utils.ParseSort(r.URL.Query().Get("sort"),
// 		bson.D{{Key: "name", Value: 1}},
// 		map[string]bson.D{
// 			"price_asc":  {{Key: "price", Value: 1}},
// 			"price_desc": {{Key: "price", Value: -1}},
// 			"name_asc":   {{Key: "name", Value: 1}},
// 			"name_desc":  {{Key: "name", Value: -1}},
// 		})

// 	opts := options.Find().SetSort(sort).SetSkip(skip).SetLimit(limit)
// 	items, err := utils.FindAndDecode[models.Product](ctx, db.ProductCollection, filter, opts)
// 	if err != nil {
// 		utils.Error(w, http.StatusInternalServerError, "Failed to fetch items")
// 		return
// 	}

// 	total, err := db.ProductCollection.CountDocuments(ctx, filter)
// 	if err != nil {
// 		utils.Error(w, http.StatusInternalServerError, "Failed to count items")
// 		return
// 	}

// 	utils.JSON(w, http.StatusOK, map[string]interface{}{
// 		"items": items,
// 		"total": total,
// 	})
// }

// // func GetItems(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// // 	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
// // 	defer cancel()

// // 	itemType := r.URL.Query().Get("type")
// // 	search := r.URL.Query().Get("search")
// // 	category := r.URL.Query().Get("category")
// // 	limitStr := r.URL.Query().Get("limit")
// // 	offsetStr := r.URL.Query().Get("offset")
// // 	sortParam := r.URL.Query().Get("sort")

// // 	limit := int64(10)
// // 	offset := int64(0)

// // 	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
// // 		limit = int64(l)
// // 	}
// // 	if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
// // 		offset = int64(o)
// // 	}

// // 	filter := bson.M{}
// // 	if itemType != "" {
// // 		filter["type"] = itemType
// // 	}
// // 	if category != "" {
// // 		filter["category"] = category
// // 	}
// // 	if search != "" {
// // 		// sanitize search here if needed
// // 		filter["name"] = bson.M{"$regex": primitiveRegex(search)}
// // 	}

// // 	sort := bson.D{{Key: "name", Value: 1}}
// // 	switch sortParam {
// // 	case "price_asc":
// // 		sort = bson.D{{Key: "price", Value: 1}}
// // 	case "price_desc":
// // 		sort = bson.D{{Key: "price", Value: -1}}
// // 	case "name_asc":
// // 		sort = bson.D{{Key: "name", Value: 1}}
// // 	case "name_desc":
// // 		sort = bson.D{{Key: "name", Value: -1}}
// // 	}

// // 	findOptions := options.Find().SetLimit(limit).SetSkip(offset).SetSort(sort)

// // 	cursor, err := db.ProductCollection.Find(ctx, filter, findOptions)
// // 	if err != nil {
// // 		http.Error(w, "Failed to fetch items", http.StatusInternalServerError)
// // 		return
// // 	}
// // 	defer cursor.Close(ctx)

// // 	var items []models.Product
// // 	if err := cursor.All(ctx, &items); err != nil {
// // 		http.Error(w, "Failed to decode items", http.StatusInternalServerError)
// // 		return
// // 	}
// // 	if items == nil {
// // 		items = []models.Product{}
// // 	}

// // 	count, err := db.ProductCollection.CountDocuments(ctx, filter)
// // 	if err != nil {
// // 		http.Error(w, "Failed to count items", http.StatusInternalServerError)
// // 		return
// // 	}

// // 	w.Header().Set("Content-Type", "application/json")
// // 	if err := json.NewEncoder(w).Encode(map[string]interface{}{
// // 		"items": items,
// // 		"total": count,
// // 	}); err != nil {
// // 		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
// // 	}
// // }

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
	if err := json.NewEncoder(w).Encode(categories); err != nil {
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

// // primitiveRegex is a small helper to build regex bson query part safely
// func primitiveRegex(pattern string) primitive.Regex {
// 	return primitive.Regex{Pattern: pattern, Options: "i"}
// }

// package farms

// import (
// 	"context"
// 	"encoding/json"
// 	"naevis/db"
// 	"naevis/filemgr"
// 	"naevis/models"
// 	"naevis/mq"
// 	"naevis/utils"
// 	"net/http"
// 	"strconv"
// 	"strings"
// 	"time"

// 	"github.com/julienschmidt/httprouter"
// 	"go.mongodb.org/mongo-driver/bson"
// 	"go.mongodb.org/mongo-driver/bson/primitive"
// 	"go.mongodb.org/mongo-driver/mongo/options"
// )

// func GetItems(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	itemType := r.URL.Query().Get("type")     // "product" or "tool"
// 	search := r.URL.Query().Get("search")     // search text
// 	category := r.URL.Query().Get("category") // filter by category
// 	limitStr := r.URL.Query().Get("limit")
// 	offsetStr := r.URL.Query().Get("offset")
// 	sortParam := r.URL.Query().Get("sort") // e.g. price_asc, name_desc

// 	limit := int64(10)
// 	offset := int64(0)

// 	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
// 		limit = int64(l)
// 	}
// 	if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
// 		offset = int64(o)
// 	}

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

// 	// Determine sort order
// 	sort := bson.D{{Key: "name", Value: 1}} // default
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

// 	findOptions := options.Find().
// 		SetLimit(limit).
// 		SetSkip(offset).
// 		SetSort(sort)

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
// 	if len(items) == 0 {
// 		items = []models.Product{}
// 	}

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

// func CreateProduct(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	createItem(w, r, "product")
// }

// func CreateTool(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	createItem(w, r, "tool")
// }

// func createItem(w http.ResponseWriter, r *http.Request, itemType string) {
// 	err := r.ParseMultipartForm(32 << 20)
// 	if err != nil {
// 		http.Error(w, "Failed to parse form", http.StatusBadRequest)
// 		return
// 	}

// 	item := models.Product{
// 		Name:        r.FormValue("name"),
// 		Description: r.FormValue("description"),
// 		Category:    r.FormValue("category"),
// 		SKU:         r.FormValue("sku"),
// 		Unit:        r.FormValue("unit"),
// 		Type:        itemType,
// 		Featured:    r.FormValue("featured") == "true" || r.FormValue("featured") == "on",
// 	}

// 	if price, err := strconv.ParseFloat(r.FormValue("price"), 64); err == nil {
// 		item.Price = price
// 	}
// 	if quantity, err := strconv.ParseFloat(r.FormValue("quantity"), 64); err == nil {
// 		item.Quantity = quantity
// 	}
// 	if date := r.FormValue("availableFrom"); date != "" {
// 		if t, err := time.Parse("2006-01-02", date); err == nil {
// 			item.AvailableFrom = &models.SafeTime{Time: t}
// 		}
// 	}
// 	if date := r.FormValue("availableTo"); date != "" {
// 		if t, err := time.Parse("2006-01-02", date); err == nil {
// 			item.AvailableTo = &models.SafeTime{Time: t}
// 		}
// 	}

// 	// ⬇️ Save images with new filemgr.SaveFormFiles signature
// 	if r.MultipartForm == nil {
// 		http.Error(w, "Multipart form missing", http.StatusBadRequest)
// 		return
// 	}
// 	// Collect all file field names like images_1, images_2, ...
// 	imageKeys := []string{}
// 	for key := range r.MultipartForm.File {
// 		if strings.HasPrefix(key, "images_") {
// 			imageKeys = append(imageKeys, key)
// 		}
// 	}

// 	if len(imageKeys) == 0 {
// 		item.ImageURLs = []string{}
// 	} else {
// 		files, err := filemgr.SaveFormFilesByKeys(
// 			r.MultipartForm,
// 			imageKeys,
// 			filemgr.EntityProduct,
// 			filemgr.PicPhoto,
// 			false,
// 		)
// 		if err != nil {
// 			http.Error(w, "Failed to save images", http.StatusInternalServerError)
// 			return
// 		}
// 		item.ImageURLs = files
// 	}

// 	// files, err := filemgr.SaveFormFiles(
// 	// 	r.MultipartForm,
// 	// 	"images",
// 	// 	filemgr.EntityType("product"),
// 	// 	filemgr.PictureType("gallery"),
// 	// 	false,
// 	// )
// 	// if err != nil {
// 	// 	http.Error(w, "Failed to save images", http.StatusInternalServerError)
// 	// 	return
// 	// }
// 	// item.ImageURLs = files

// 	item.ProductID = utils.GenerateID(13)
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	_, err = db.ProductCollection.InsertOne(ctx, item)
// 	if err != nil {
// 		http.Error(w, "Failed to insert item", http.StatusInternalServerError)
// 		return
// 	}

// 	go mq.Emit(ctx, "farmitem-created", models.Index{EntityType: "product", EntityId: item.ProductID, Method: "POST"})

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(item)
// }

// func updateItem(w http.ResponseWriter, r *http.Request, ps httprouter.Params, itemType string) {
// 	idParam := ps.ByName("id")
// 	// objID, err := primitive.ObjectIDFromHex(idParam)
// 	// if err != nil {
// 	// 	http.Error(w, "Invalid ID", http.StatusBadRequest)
// 	// 	return
// 	// }

// 	err := r.ParseMultipartForm(32 << 20)
// 	if err != nil {
// 		http.Error(w, "Failed to parse form", http.StatusBadRequest)
// 		return
// 	}

// 	item := models.Product{
// 		Name:        r.FormValue("name"),
// 		Description: r.FormValue("description"),
// 		Category:    r.FormValue("category"),
// 		SKU:         r.FormValue("sku"),
// 		Unit:        r.FormValue("unit"),
// 		Type:        itemType,
// 		Featured:    r.FormValue("featured") == "true" || r.FormValue("featured") == "on",
// 	}

// 	if price, err := strconv.ParseFloat(r.FormValue("price"), 64); err == nil {
// 		item.Price = price
// 	}
// 	if quantity, err := strconv.ParseFloat(r.FormValue("quantity"), 64); err == nil {
// 		item.Quantity = quantity
// 	}
// 	if date := r.FormValue("availableFrom"); date != "" {
// 		if t, err := time.Parse("2006-01-02", date); err == nil {
// 			item.AvailableFrom = &models.SafeTime{Time: t}
// 		}
// 	}
// 	if date := r.FormValue("availableTo"); date != "" {
// 		if t, err := time.Parse("2006-01-02", date); err == nil {
// 			item.AvailableTo = &models.SafeTime{Time: t}
// 		}
// 	}

// 	// ⬇️ Save new images with new signature
// 	if r.MultipartForm == nil {
// 		http.Error(w, "Multipart form missing", http.StatusBadRequest)
// 		return
// 	}
// 	// Collect all file field names like images_1, images_2, ...
// 	imageKeys := []string{}
// 	for key := range r.MultipartForm.File {
// 		if strings.HasPrefix(key, "images_") {
// 			imageKeys = append(imageKeys, key)
// 		}
// 	}

// 	if len(imageKeys) == 0 {
// 		item.ImageURLs = []string{}
// 	} else {
// 		files, err := filemgr.SaveFormFilesByKeys(
// 			r.MultipartForm,
// 			imageKeys,
// 			filemgr.EntityProduct,
// 			filemgr.PicPhoto,
// 			false,
// 		)
// 		if err != nil {
// 			http.Error(w, "Failed to save images", http.StatusInternalServerError)
// 			return
// 		}
// 		item.ImageURLs = files
// 	}

// 	// files, err := filemgr.SaveFormFiles(
// 	// 	r.MultipartForm,
// 	// 	"images",
// 	// 	filemgr.EntityType("product"),
// 	// 	filemgr.PictureType("gallery"),
// 	// 	false,
// 	// )
// 	// if err != nil {
// 	// 	http.Error(w, "Failed to save images", http.StatusInternalServerError)
// 	// 	return
// 	// }
// 	// if len(files) > 0 {
// 	// 	item.ImageURLs = files
// 	// }

// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	update := bson.M{"$set": item}
// 	_, err = db.ProductCollection.UpdateByID(ctx, idParam, update)
// 	if err != nil {
// 		http.Error(w, "Failed to update item", http.StatusInternalServerError)
// 		return
// 	}

// 	go mq.Emit(ctx, "farmitem-updated", models.Index{EntityType: "product", EntityId: idParam, Method: "PUT"})

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
// }

// func UpdateTool(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	updateItem(w, r, ps, "tool")
// }

// func UpdateProduct(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	updateItem(w, r, ps, "product")
// }

// func DeleteProduct(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	deleteItem(w, r, ps)
// }

// func DeleteTool(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	deleteItem(w, r, ps)
// }

// func deleteItem(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	_ = r
// 	idParam := ps.ByName("id")

// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	_, err := db.ProductCollection.DeleteOne(ctx, bson.M{"productid": idParam})
// 	if err != nil {
// 		http.Error(w, "Failed to delete item", http.StatusInternalServerError)
// 		return
// 	}

// 	go mq.Emit(ctx, "farmitem-deleted", models.Index{EntityType: "product", EntityId: idParam, Method: "DELETE"})

// 	w.WriteHeader(http.StatusOK)
// 	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
// }

// // GetItemCategories returns distinct categories from the items collection based on the type (product/tool)
// func GetItemCategories(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	itemType := r.URL.Query().Get("type")

// 	var categories []string

// 	switch itemType {
// 	case "tool":
// 		categories = []string{
// 			"Cutting Tools",
// 			"Irrigation Tools",
// 			"Harvesting Tools",
// 			"Hand Tools",
// 			"Protective Gear",
// 			"Fertilizer Applicators",
// 		}
// 	case "product":
// 		fallthrough
// 	default:
// 		categories = []string{
// 			"Spices",
// 			"Pickles",
// 			"Flour",
// 			"Oils",
// 			"Honey",
// 			"Tea & Coffee",
// 			"Dry Fruits",
// 			"Natural Sweeteners",
// 		}
// 	}

// 	if len(categories) == 0 {
// 		categories = []string{}
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(categories)
// }
