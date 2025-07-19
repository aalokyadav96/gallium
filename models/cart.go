package models

import "time"

type CartItem struct {
	UserID   string    `json:"userId" bson:"userId"`
	Category string    `json:"category" bson:"category"` // e.g. "crops", "merchandise"
	Item     string    `json:"item" bson:"item"`
	Unit     string    `json:"unit,omitempty" bson:"unit,omitempty"`
	Farm     string    `json:"farm,omitempty" bson:"farm,omitempty"`
	FarmId   string    `json:"farmid,omitempty" bson:"farmid,omitempty"`
	Quantity int       `json:"quantity" bson:"quantity"`
	Price    float64   `json:"price" bson:"price"`
	AddedAt  time.Time `json:"added_at" bson:"added_at"`
}

type CheckoutSession struct {
	UserID    string                `json:"userId" bson:"userId"`
	Items     map[string][]CartItem `json:"items" bson:"items"` // grouped by category
	Address   string                `json:"address" bson:"address"`
	Total     float64               `json:"total" bson:"total"`
	CreatedAt time.Time             `json:"createdAt" bson:"createdAt"`
}

type Order struct {
	OrderID       string                `json:"orderId" bson:"orderId"`
	UserID        string                `json:"userId" bson:"userId"`
	Items         map[string][]CartItem `json:"items" bson:"items"`
	Address       string                `json:"address" bson:"address"`
	PaymentMethod string                `json:"paymentMethod" bson:"paymentMethod"`
	Total         float64               `json:"total" bson:"total"`
	Status        string                `json:"status" bson:"status"` // e.g. "pending", "completed"
	ApprovedBy    []string              `json:"approvedBy" bson:"approvedBy"`
	CreatedAt     time.Time             `json:"createdAt" bson:"createdAt"`
}
