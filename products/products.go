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

	// Physical product fields
	Size        string            `json:"size,omitempty"`
	Color       string            `json:"color,omitempty"`
	Ingredients string            `json:"ingredients,omitempty"`
	ExpiryDate  string            `json:"expiryDate,omitempty"`
	Weight      string            `json:"weight,omitempty"`
	Specs       map[string]string `json:"specs,omitempty"`

	// Media fields
	Author     string `json:"author,omitempty"`     // book
	ISBN       string `json:"isbn,omitempty"`       // book
	Platform   string `json:"platform,omitempty"`   // software
	Version    string `json:"version,omitempty"`    // software
	License    string `json:"license,omitempty"`    // software
	Instructor string `json:"instructor,omitempty"` // course
	Duration   string `json:"duration,omitempty"`   // course / subscription

	// Subscription fields
	BillingCycle string `json:"billingCycle,omitempty"`
	TrialPeriod  string `json:"trialPeriod,omitempty"`
	Scope        string `json:"scope,omitempty"`

	// Creative / art
	Artist     string `json:"artist,omitempty"`
	Medium     string `json:"medium,omitempty"`
	Dimensions string `json:"dimensions,omitempty"`

	// Vehicle fields
	Engine   string `json:"engine,omitempty"`
	Mileage  string `json:"mileage,omitempty"`
	FuelType string `json:"fuelType,omitempty"`

	Featured      bool   `json:"featured,omitempty"`
	SKU           string `json:"sku,omitempty"`
	AvailableFrom string `json:"availableFrom,omitempty"`
	AvailableTo   string `json:"availableTo,omitempty"`
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

		// Example physical product fields
		Size:        "M",
		Color:       "Red",
		Ingredients: "Cotton",
		ExpiryDate:  "2025-12-31",
		Weight:      "500g",
		Specs: map[string]string{
			"Material": "Cotton",
			"Origin":   "India",
		},

		// Example media fields
		Author:     "John Doe",
		ISBN:       "123-4567890123",
		Platform:   "Windows",
		Version:    "1.0.0",
		License:    "GPL",
		Instructor: "Jane Smith",
		Duration:   "10 hours",

		// Subscription fields
		BillingCycle: "Monthly",
		TrialPeriod:  "7 days",
		Scope:        "All courses",

		// Creative
		Artist:     "Jane Artist",
		Medium:     "Oil on Canvas",
		Dimensions: "24x36 inches",

		// Vehicle
		Engine:   "V6",
		Mileage:  "15 km/l",
		FuelType: "Petrol",

		Featured:      true,
		SKU:           "SKU123",
		AvailableFrom: "2025-01-01",
		AvailableTo:   "2025-12-31",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(product)
}
