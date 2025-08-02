package tickets

import (
	"context"
	"encoding/json"
	"log"
	"naevis/db"
	"naevis/structs"
	"naevis/utils"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

func BuysTicket(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	type Request struct {
		TicketID string `json:"ticketId"`
		EventID  string `json:"eventId"`
		Quantity int    `json:"quantity"`
	}

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TicketID == "" || req.EventID == "" || req.Quantity <= 0 {
		http.Error(w, "Missing or invalid parameters", http.StatusBadRequest)
		return
	}

	ctx := context.TODO()

	// Find the ticket
	var ticket structs.Ticket
	err := db.TicketsCollection.FindOne(ctx, bson.M{
		"ticketid": req.TicketID,
		"eventid":  req.EventID,
	}).Decode(&ticket)

	if err != nil {
		http.Error(w, "Ticket not found", http.StatusNotFound)
		return
	}

	if ticket.Available < req.Quantity {
		http.Error(w, "Not enough tickets available", http.StatusBadRequest)
		return
	}

	// Atomically update ticket availability and sold count
	update := bson.M{
		"$inc": bson.M{
			"available": -req.Quantity,
			"sold":      req.Quantity,
		},
		"$set": bson.M{
			"updatedat": time.Now(),
		},
	}

	_, err = db.TicketsCollection.UpdateOne(ctx, bson.M{
		"ticketid":  req.TicketID,
		"eventid":   req.EventID,
		"available": bson.M{"$gte": req.Quantity}, // prevent oversell
	}, update)

	if err != nil {
		http.Error(w, "Failed to update ticket", http.StatusInternalServerError)
		return
	}

	// Optional: Save booking info
	// booking := structs.Booking{
	booking := Ticking{
		BookingID: utils.GenerateID(14),
		EventID:   req.EventID,
		TicketID:  req.TicketID,
		Quantity:  req.Quantity,
		BookedAt:  time.Now(),
		// UserID:    userID, // if you track authenticated users
	}

	_, err = db.BookingsCollection.InsertOne(ctx, booking)
	if err != nil {
		log.Println("Warning: booking inserted failed, continuing:", err)
		// Not critical, don’t block main booking
	}

	_, err = db.PurchasedTicketsCollection.InsertOne(ctx, req.TicketID)
	if err != nil {
		log.Println("Warning: booking inserted failed, continuing:", err)
		// Not critical, don’t block main booking
	}

	// Response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Ticket booked successfully",
	})
}

type Ticking struct {
	BookingID string    `bson:"bookingid"`
	EventID   string    `bson:"eventid"`
	TicketID  string    `bson:"ticketid"`
	Quantity  int       `bson:"quantity"`
	BookedAt  time.Time `bson:"bookedat"`
	// UserID  string    `bson:"userid,omitempty"` // optional
}
