package products

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

type Product struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       float64  `json:"price"`
	Unit        string   `json:"unit"`
	ImageURLs   []string `json:"imageUrls"`
}

func GetProductDetails(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entityType")
	entityId := ps.ByName("entityId")

	// Dummy data
	product := Product{
		ID:          entityId,
		Name:        "Sample " + entityType,
		Description: "This is a dummy description for a " + entityType,
		Price:       199.99,
		Unit:        "piece",
		ImageURLs: []string{
			"example1.jpg",
			"example2.jpg",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(product)
}
