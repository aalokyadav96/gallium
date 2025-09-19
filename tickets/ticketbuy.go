package tickets

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"naevis/db"
	"naevis/globals"
	"naevis/models"
	"naevis/mq"
	"naevis/stripe"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// A global map to manage event-specific update channels
var eventUpdateChannels = struct {
	sync.RWMutex
	channels map[string]chan map[string]any
}{
	channels: make(map[string]chan map[string]any),
}

// Helper function to get or create the updates channel for an event
func GetUpdatesChannel(eventId string) chan map[string]any {
	eventUpdateChannels.RLock()
	if ch, exists := eventUpdateChannels.channels[eventId]; exists {
		eventUpdateChannels.RUnlock()
		return ch
	}
	eventUpdateChannels.RUnlock()

	// Create a new channel if not exists
	eventUpdateChannels.Lock()
	defer eventUpdateChannels.Unlock()
	newCh := make(chan map[string]any, 10) // Buffered channel
	eventUpdateChannels.channels[eventId] = newCh
	return newCh
}

// POST /ticket/event/:eventid/:ticketid/payment-session
func CreateTicketPaymentSession(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ticketId := ps.ByName("ticketid")
	eventId := ps.ByName("eventid")

	// Parse request body for quantity
	var body struct {
		Quantity int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Quantity < 1 {
		http.Error(w, "Invalid request or quantity", http.StatusBadRequest)
		return
	}

	// Generate a Stripe payment session
	session, err := stripe.CreateTicketSession(ticketId, eventId, body.Quantity)
	if err != nil {
		log.Printf("Error creating payment session: %v", err)
		http.Error(w, "Failed to create payment session", http.StatusInternalServerError)
		return
	}

	dataResponse := map[string]any{
		"paymentUrl": session.URL,
		"eventId":    session.EventID,
		"ticketId":   session.TicketID,
		"stock":      session.Quantity,
	}

	response := map[string]any{
		"success": true,
		"data":    dataResponse,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GET /events/:eventId/updates
func EventUpdates(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventId := ps.ByName("eventId")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	updatesChannel := GetUpdatesChannel(eventId)
	defer func() {
		// Optionally close the channel when the connection ends
		log.Printf("Closing updates channel for event: %s", eventId)
	}()

	// Listen for updates or client disconnection
	for {
		select {
		case update := <-updatesChannel:
			jsonUpdate, _ := json.Marshal(update)
			fmt.Fprintf(w, "data: %s\n\n", jsonUpdate)
			flusher.Flush()
		case <-r.Context().Done():
			// Client disconnected
			return
		}
	}
}

// BroadcastTicketUpdate sends real-time ticket updates to subscribers
func BroadcastTicketUpdate(eventId, ticketId string, remainingTickets int) {
	update := map[string]any{
		"type":             "ticket_update",
		"ticketId":         ticketId,
		"remainingTickets": remainingTickets,
	}
	channel := GetUpdatesChannel(eventId)
	select {
	case channel <- update:
		// Successfully sent update
	default:
		// If the channel is full, log a warning or handle the overflow
		log.Printf("Warning: Updates channel for event %s is full. Dropping update.", eventId)
	}
}

// TicketPurchaseRequest represents the request body for purchasing tickets
type TicketPurchaseRequest struct {
	TicketID string `json:"ticketId"`
	EventID  string `json:"eventId"`
	Quantity int    `json:"quantity"`
}

// TicketPurchaseResponse represents the response body for ticket purchase confirmation
type TicketPurchaseResponse struct {
	Message    string `json:"message"`
	Success    string `json:"success"`
	UniqueCode string `json:"uniquecode"`
}

// ProcessTicketPayment simulates the payment processing logic
func ProcessTicketPayment(ticketID, eventID string, quantity int) bool {
	// Implement actual payment processing logic (e.g., calling a payment gateway)
	// For the sake of this example, we'll assume payment is always successful.
	log.Printf("Processing payment for TicketID: %s, EventID: %s, Quantity: %d", ticketID, eventID, quantity)
	return true
}

// UpdateTicketStatus simulates updating the ticket status in the database
func UpdateTicketStatus(ticketID, eventID string, quantity int) error {
	// Implement actual logic to update ticket status in the database
	log.Printf("Updating ticket status for TicketID: %s, EventID: %s, Quantity: %d", ticketID, eventID, quantity)
	return nil
}

// ConfirmTicketPurchase handles the POST request for confirming the ticket purchase
func ConfirmTicketPurchase(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var request TicketPurchaseRequest

	// Parse JSON body
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Process payment
	if !ProcessTicketPayment(request.TicketID, request.EventID, request.Quantity) {
		http.Error(w, "Payment failed", http.StatusBadRequest)
		return
	}

	// Update ticket status and store purchase
	if err := UpdateTicketStatus(request.TicketID, request.EventID, request.Quantity); err != nil {
		http.Error(w, "Failed to update ticket status", http.StatusInternalServerError)
		return
	}

	// Complete ticket purchase
	buyTicket(w, r, request)
}

// PurchaseTicket validates and deducts ticket quantity, returns generated ticket codes
func PurchaseTicket(eventID, ticketID, userID string, quantity int) ([]string, error) {
	var ticket models.Ticket
	err := db.TicketsCollection.FindOne(context.TODO(), bson.M{"eventid": eventID, "ticketid": ticketID}).Decode(&ticket)
	if err != nil {
		return nil, fmt.Errorf("ticket not found")
	}

	if ticket.Quantity < quantity {
		return nil, fmt.Errorf("not enough tickets available")
	}

	// Deduct quantity
	_, err = db.TicketsCollection.UpdateOne(
		context.TODO(),
		bson.M{"eventid": eventID, "ticketid": ticketID},
		bson.M{"$inc": bson.M{"quantity": -quantity, "sold": quantity}},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update ticket quantity")
	}

	// Generate unique codes
	var codes []string
	for i := 0; i < quantity; i++ {
		codes = append(codes, utils.GetUUID())
	}

	return codes, nil
}

// StorePurchasedTickets inserts purchased tickets and user data into DB
func StorePurchasedTickets(eventID, ticketID, userID string, codes []string) error {
	if len(codes) == 0 {
		return fmt.Errorf("no tickets to store")
	}

	now := time.Now()
	createdAt := now.Format(time.RFC3339)

	var purchasedDocs []interface{}
	var userDataDocs []models.UserData

	for _, code := range codes {
		purchasedDocs = append(purchasedDocs, models.PurchasedTicket{
			EventID:      eventID,
			TicketID:     ticketID,
			UserID:       userID,
			UniqueCode:   code,
			PurchaseDate: now,
		})

		userDataDocs = append(userDataDocs, models.UserData{
			UserID:     userID,
			EntityID:   code,
			EntityType: "ticket",
			ItemID:     ticketID,
			ItemType:   "ticket",
			CreatedAt:  createdAt,
		})
	}

	_, err := db.PurchasedTicketsCollection.InsertMany(context.TODO(), purchasedDocs)
	if err != nil {
		return fmt.Errorf("failed to store purchased tickets: %v", err)
	}

	userdata.AddUserDataBatch(userDataDocs)
	mq.Notify("ticket-bought", models.Index{})
	return nil
}

// Handler
func buyTicket(w http.ResponseWriter, r *http.Request, request TicketPurchaseRequest) {
	userID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok || userID == "" {
		http.Error(w, "invalid user", http.StatusBadRequest)
		return
	}

	codes, err := PurchaseTicket(request.EventID, request.TicketID, userID, request.Quantity)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := StorePurchasedTickets(request.EventID, request.TicketID, userID, codes); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := struct {
		Message     string   `json:"message"`
		Success     string   `json:"success"`
		UniqueCodes []string `json:"uniqueCodes"`
	}{
		Message:     "Payment successfully processed. Tickets purchased.",
		Success:     "true",
		UniqueCodes: codes,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// Get Available Seats
func GetAvailableSeats(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")

	filter := bson.M{"event_id": eventID}
	var ticket struct {
		Seats []struct {
			SeatID string `bson:"seat_id"`
			Status string `bson:"status"`
		} `bson:"seats"`
	}

	err := db.TicketsCollection.FindOne(context.Background(), filter).Decode(&ticket)
	if err != nil {
		http.Error(w, `{"error": "No tickets found for this event"}`, http.StatusNotFound)
		return
	}

	var availableSeats []string
	for _, seat := range ticket.Seats {
		if seat.Status == "available" {
			availableSeats = append(availableSeats, seat.SeatID)
		}
	}

	if len(availableSeats) == 0 {
		availableSeats = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"seats": availableSeats})
}

// Lock Seats
func LockSeats(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	var request struct {
		UserID string   `json:"user_id"`
		Seats  []string `json:"seats"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
		return
	}

	filter := bson.M{"event_id": eventID, "seats.seat_id": bson.M{"$in": request.Seats}}
	update := bson.M{"$set": bson.M{"seats.$[].status": "locked", "seats.$[].user_id": request.UserID}}

	_, err := db.TicketsCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		http.Error(w, `{"error": "Failed to lock seats"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "Seats locked successfully"})
}

// Unlock Seats
func UnlockSeats(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	var request struct {
		UserID string   `json:"user_id"`
		Seats  []string `json:"seats"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
		return
	}

	filter := bson.M{"event_id": eventID, "seats.seat_id": bson.M{"$in": request.Seats}, "seats.user_id": request.UserID}
	update := bson.M{"$set": bson.M{"seats.$[].status": "available", "seats.$[].user_id": nil}}

	_, err := db.TicketsCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		http.Error(w, `{"error": "Failed to unlock seats"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "Seats unlocked successfully"})
}

// Confirm Seat Purchase
func ConfirmSeatPurchase(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	ticketID := ps.ByName("ticketid")

	var request struct {
		UserID string   `json:"user_id"`
		Seats  []string `json:"seats"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
		return
	}

	filter := bson.M{"_id": ticketID, "event_id": eventID, "seats.seat_id": bson.M{"$in": request.Seats}}
	var ticket struct {
		Seats []struct {
			SeatID string `bson:"seat_id"`
			Status string `bson:"status"`
			UserID string `bson:"user_id"`
		} `bson:"seats"`
	}

	err := db.TicketsCollection.FindOne(context.Background(), filter).Decode(&ticket)
	if err != nil {
		http.Error(w, `{"error": "Ticket or seats not found"}`, http.StatusNotFound)
		return
	}

	for _, seat := range ticket.Seats {
		if seat.Status != "locked" || seat.UserID != request.UserID {
			http.Error(w, `{"error": "Some seats are not properly locked or have been taken"}`, http.StatusConflict)
			return
		}
	}

	update := bson.M{"$set": bson.M{"seats.$[].status": "booked"}}

	_, err = db.TicketsCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		http.Error(w, `{"error": "Failed to confirm purchase"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "Ticket purchased successfully"})
}
