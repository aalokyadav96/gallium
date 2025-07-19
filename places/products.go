// package places

// import (
// 	"context"
// 	"encoding/json"
// 	"naevis/db"
// 	"net/http"
// 	"time"

// 	"github.com/julienschmidt/httprouter"
// 	"go.mongodb.org/mongo-driver/bson"
// 	"go.mongodb.org/mongo-driver/bson/primitive"
// )

// type Product struct {
// 	ID      primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
// 	PlaceID string             `bson:"placeid,omitempty" json:"placeid"`
// 	Name    string             `bson:"name" json:"name"`
// 	Price   float64            `bson:"price" json:"price"`
// }

// func GetProducts(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	placeIDStr := ps.ByName("placeid")

// 	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
// 	defer cancel()

// 	cursor, err := db.ProductCollection.Find(ctx, bson.M{"placeid": placeIDStr})
// 	if err != nil {
// 		http.Error(w, "Database error", http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(ctx)

// 	var products []Product
// 	if err := cursor.All(ctx, &products); err != nil {
// 		http.Error(w, "Failed to decode", http.StatusInternalServerError)
// 		return
// 	}

// 	if len(products) == 0 {
// 		products = []Product{}
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(products)
// }

// func PostProduct(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	placeIDStr := ps.ByName("placeid")
// 	var product Product
// 	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
// 		http.Error(w, "Invalid JSON", http.StatusBadRequest)
// 		return
// 	}

// 	product.PlaceID = placeIDStr
// 	product.ID = primitive.NewObjectID()

// 	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
// 	defer cancel()

// 	_, err := db.ProductCollection.InsertOne(ctx, product)
// 	if err != nil {
// 		http.Error(w, "Insert failed", http.StatusInternalServerError)
// 		return
// 	}

// 	w.WriteHeader(http.StatusCreated)
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(product)
// }

// func PutProduct(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	id, err := primitive.ObjectIDFromHex(ps.ByName("productId"))
// 	if err != nil {
// 		http.Error(w, "Invalid product ID", http.StatusBadRequest)
// 		return
// 	}

// 	var updateData Product
// 	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
// 		http.Error(w, "Invalid JSON", http.StatusBadRequest)
// 		return
// 	}

// 	update := bson.M{
// 		"$set": bson.M{
// 			"name":  updateData.Name,
// 			"price": updateData.Price,
// 		},
// 	}

// 	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
// 	defer cancel()

// 	_, err = db.ProductCollection.UpdateByID(ctx, id, update)
// 	if err != nil {
// 		http.Error(w, "Update failed", http.StatusInternalServerError)
// 		return
// 	}

// 	w.WriteHeader(http.StatusOK)
// }

// func DeleteProduct(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	id, err := primitive.ObjectIDFromHex(ps.ByName("productId"))
// 	if err != nil {
// 		http.Error(w, "Invalid product ID", http.StatusBadRequest)
// 		return
// 	}

// 	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
// 	defer cancel()

// 	_, err = db.ProductCollection.DeleteOne(ctx, bson.M{"_id": id})
// 	if err != nil {
// 		http.Error(w, "Delete failed", http.StatusInternalServerError)
// 		return
// 	}

// 	w.WriteHeader(http.StatusOK)
// }

// func PostPlaceProductPurchase(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	productId, err := primitive.ObjectIDFromHex(ps.ByName("productId"))
// 	if err != nil {
// 		http.Error(w, "Invalid product ID", http.StatusBadRequest)
// 		return
// 	}

// 	// TODO: log purchase or update inventory
// 	_ = productId // placeholder

// 	w.WriteHeader(http.StatusOK)
// 	w.Write([]byte("Purchase confirmed"))
// }

// // Optional: fallback
// func GetProduct(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	http.Error(w, "Not implemented", http.StatusNotImplemented)
// }

//	func PostProductPurchase(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
//		http.Error(w, "Not implemented", http.StatusNotImplemented)
//	}

package places

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"naevis/db"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Product represents a product sold by a place
type Product struct {
	ID      primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	PlaceID string             `bson:"placeid,omitempty" json:"placeid"`
	Name    string             `bson:"name" json:"name"`
	Price   float64            `bson:"price" json:"price"`
}

// UnmarshalJSON supports both string and float for price
func (p *Product) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if name, ok := raw["name"].(string); ok {
		p.Name = name
	} else {
		return errors.New("name is required and must be a string")
	}

	if placeID, ok := raw["placeid"].(string); ok {
		p.PlaceID = placeID
	}

	switch price := raw["price"].(type) {
	case float64:
		p.Price = price
	case string:
		var parsed float64
		if err := json.Unmarshal([]byte(price), &parsed); err != nil {
			return errors.New("price must be a number")
		}
		p.Price = parsed
	default:
		return errors.New("price must be a number or numeric string")
	}

	return nil
}

// validateProduct ensures required fields are valid
func validateProduct(p Product) error {
	if p.Name == "" {
		return errors.New("name is required")
	}
	if p.Price <= 0 {
		return errors.New("price must be positive")
	}
	return nil
}

// parseJSON checks content-type, reads and parses JSON body
func parseJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if ct != "application/json" {
		http.Error(w, "Expected application/json", http.StatusUnsupportedMediaType)
		return errors.New("unsupported content type")
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return err
	}
	defer r.Body.Close()

	if err := json.Unmarshal(body, dst); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return err
	}
	return nil
}

func GetProducts(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	placeIDStr := ps.ByName("placeid")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	cursor, err := db.ProductCollection.Find(ctx, bson.M{"placeid": placeIDStr})
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var products []Product
	if err := cursor.All(ctx, &products); err != nil {
		http.Error(w, "Failed to decode", http.StatusInternalServerError)
		return
	}

	if products == nil {
		products = []Product{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}

func PostProduct(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	placeIDStr := ps.ByName("placeid")

	var product Product
	if err := parseJSON(w, r, &product); err != nil {
		return
	}

	product.PlaceID = placeIDStr
	product.ID = primitive.NewObjectID()

	if err := validateProduct(product); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	_, err := db.ProductCollection.InsertOne(ctx, product)
	if err != nil {
		http.Error(w, "Insert failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(product)
}

func PutProduct(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := primitive.ObjectIDFromHex(ps.ByName("productId"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	var updateData Product
	if err := parseJSON(w, r, &updateData); err != nil {
		return
	}

	if err := validateProduct(updateData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	update := bson.M{
		"$set": bson.M{
			"name":  updateData.Name,
			"price": updateData.Price,
		},
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	_, err = db.ProductCollection.UpdateByID(ctx, id, update)
	if err != nil {
		http.Error(w, "Update failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func DeleteProduct(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := primitive.ObjectIDFromHex(ps.ByName("productId"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	_, err = db.ProductCollection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		http.Error(w, "Delete failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func PostPlaceProductPurchase(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	productId, err := primitive.ObjectIDFromHex(ps.ByName("productId"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	// TODO: log purchase or update inventory
	_ = productId

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// Optional: fallback
func GetProduct(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

func PostProductPurchase(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}
