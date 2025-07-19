package farms

import (
	"context"
	"net/http"
	"time"

	"naevis/db"
	"naevis/globals"
	"naevis/utils"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func BuyCrop(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	farmID, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	if err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Invalid farm ID"})
		return
	}

	cropID, err := primitive.ObjectIDFromHex(ps.ByName("cropid"))
	if err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Invalid crop ID"})
		return
	}

	// Retrieve user ID
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}
	_ = requestingUserID

	// Decrement quantity
	filter := bson.M{
		"_id":              farmID,
		"crops._id":        cropID,
		"crops.quantity":   bson.M{"$gt": 0},
		"crops.outOfStock": false,
	}

	update := bson.M{
		"$inc": bson.M{"crops.$.quantity": -1},
		"$set": bson.M{"crops.$.updatedAt": time.Now()},
	}

	result, err := db.FarmsCollection.UpdateOne(context.Background(), filter, update)
	if err != nil || result.ModifiedCount == 0 {
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Crop not available or already out of stock"})
		return
	}

	// Set outOfStock if quantity hits 0
	filterZero := bson.M{
		"_id":            farmID,
		"crops._id":      cropID,
		"crops.quantity": 0,
	}
	db.FarmsCollection.UpdateOne(context.Background(), filterZero, bson.M{
		"$set": bson.M{"crops.$.outOfStock": true},
	})

	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true})
}

// func GetMyFarmOrders(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {}

// func GetIncomingFarmOrders(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {}

// POST /api/v1/farmorders/:id/accept
// POST /api/v1/farmorders/:id/reject
// POST /api/v1/farmorders/:id/deliver
// POST /api/v1/farmorders/:id/markpaid
// GET  /api/v1/farmorders/:id/receipt

// GET /api/v1/farmorders/mine
func GetMyFarmOrders(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// TODO: Fetch orders placed by the current user to other farms
}

// GET /api/v1/farmorders/incoming
func GetIncomingFarmOrders(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// TODO: Fetch orders received by the user's farm
}

// POST /api/v1/farmorders/:id/accept
func AcceptOrder(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	orderID := ps.ByName("id")
	// TODO: Mark order as accepted
	_ = orderID
}

// POST /api/v1/farmorders/:id/reject
func RejectOrder(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	orderID := ps.ByName("id")
	// TODO: Mark order as rejected
	_ = orderID
}

// POST /api/v1/farmorders/:id/deliver
func MarkOrderDelivered(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	orderID := ps.ByName("id")
	// TODO: Update delivery status to "Delivered"
	_ = orderID
}

// POST /api/v1/farmorders/:id/markpaid
func MarkOrderPaid(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	orderID := ps.ByName("id")
	// TODO: Update payment status to "Paid"
	_ = orderID
}

// GET /api/v1/farmorders/:id/receipt
func DownloadReceipt(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	orderID := ps.ByName("id")
	// TODO: Generate and send back receipt (PDF or JSON, depending on implementation)
	_ = orderID
}
