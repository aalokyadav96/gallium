package merch

import (
	"context"
	"encoding/json"
	"log"
	"naevis/db"
	"naevis/globals"
	"naevis/models"
	"naevis/mq"
	"naevis/stripe"
	"naevis/userdata"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// POST /merch/event/:eventId/:merchId/payment-session
func CreateMerchPaymentSession(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	merchId := ps.ByName("merchid")
	eventId := ps.ByName("eventid")

	// Parse request body for quantity
	var body struct {
		Stock int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Stock < 1 {
		http.Error(w, "Invalid request or quantity", http.StatusBadRequest)
		return
	}

	// Generate a Stripe payment session
	session, err := stripe.CreateMerchSession(merchId, eventId, body.Stock)
	if err != nil {
		log.Printf("Error creating payment session: %v", err)
		http.Error(w, "Failed to create payment session", http.StatusInternalServerError)
		return
	}

	dataResponse := map[string]any{
		"paymentUrl": session.URL,
		"eventId":    session.EventID,
		"merchId":    session.MerchID,
		"quantity":   session.Stock,
	}

	response := map[string]any{
		"success": true,
		"data":    dataResponse,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// POST /merch/event/:eventId/:merchId/confirm-purchase
func ConfirmMerchPurchase(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var request struct {
		MerchID  string `json:"merchId"`
		EventID  string `json:"eventId"`
		Quantity int    `json:"quantity"`
	}

	// Parse JSON body
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Quantity < 1 {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Override IDs from URL params to prevent manipulation
	request.EventID = ps.ByName("eventid")
	request.MerchID = ps.ByName("merchid")

	// Retrieve requesting user ID from context
	userID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok || userID == "" {
		http.Error(w, "Invalid user", http.StatusUnauthorized)
		return
	}

	// Fetch merch from DB
	var merch models.Merch
	err := db.MerchCollection.FindOne(context.TODO(), bson.M{
		"entity_id": request.EventID,
		"merchid":   request.MerchID,
	}).Decode(&merch)
	if err != nil {
		http.Error(w, "Merch not found", http.StatusNotFound)
		return
	}

	if merch.Stock < request.Quantity {
		http.Error(w, "Not enough merch available", http.StatusBadRequest)
		return
	}

	// Deduct the purchased quantity atomically
	update := bson.M{"$inc": bson.M{"stock": -request.Quantity}}
	_, err = db.MerchCollection.UpdateOne(context.TODO(), bson.M{
		"entity_id": request.EventID,
		"merchid":   request.MerchID,
	}, update)
	if err != nil {
		http.Error(w, "Failed to update merch stock", http.StatusInternalServerError)
		return
	}

	// Store user purchase data
	userdata.SetUserData("merch", request.MerchID, userID, merch.EntityType, merch.EntityID)

	// Notify other services
	mq.Notify("merch-bought", models.Index{})

	// Respond with JSON success
	resp := map[string]any{
		"success": true,
		"data": map[string]any{
			"message":        "Merch purchased successfully",
			"merchId":        request.MerchID,
			"eventId":        request.EventID,
			"quantityBought": request.Quantity,
			"remainingStock": merch.Stock - request.Quantity,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
