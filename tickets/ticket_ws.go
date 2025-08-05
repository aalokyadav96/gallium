package tickets

import (
	"context"
	"encoding/json"
	"naevis/db"
	"net/http"
	_ "net/http/pprof"

	"go.mongodb.org/mongo-driver/bson"
)

func ResellTicketHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TicketID string `json:"ticketId"`
		Price    string `json:"price"`
		UserID   string `json:"userId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Update the ticket in the database
	_, err := db.TicketsCollection.UpdateOne(context.TODO(),
		bson.M{"ticket_id": req.TicketID, "owner_id": req.UserID},
		bson.M{"$set": bson.M{"is_resold": true, "resale_price": req.Price}},
	)

	if err != nil {
		http.Error(w, "Failed to list ticket for resale", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
