package farms

import (
	"context"
	"net/http"
	"time"

	"naevis/db"
	"naevis/globals"
	"naevis/models"
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
	userID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		utils.RespondWithJSON(w, http.StatusUnauthorized, utils.M{"success": false, "message": "Invalid user"})
		return
	}

	cursor, err := db.FarmOrdersCollection.Find(context.Background(), bson.M{"userId": userID})
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to fetch orders"})
		return
	}
	defer cursor.Close(context.Background())

	var orders []models.FarmOrder
	if err := cursor.All(context.Background(), &orders); err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to decode orders"})
		return
	}

	if len(orders) == 0 {
		orders = []models.FarmOrder{}
	}

	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true, "orders": orders})
}

// GET /api/v1/farmorders/incoming
func GetIncomingFarmOrders(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	userID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		utils.RespondWithJSON(w, http.StatusUnauthorized, utils.M{"success": false, "message": "Invalid user"})
		return
	}

	// Fetch farms owned by the user
	cursor, err := db.FarmsCollection.Find(context.Background(), bson.M{"owner": userID})
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to fetch farms"})
		return
	}
	defer cursor.Close(context.Background())

	var farms []bson.M
	if err := cursor.All(context.Background(), &farms); err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to decode farms"})
		return
	}

	var farmIDs []primitive.ObjectID
	for _, f := range farms {
		if id, ok := f["_id"].(primitive.ObjectID); ok {
			farmIDs = append(farmIDs, id)
		}
	}

	if len(farmIDs) == 0 {
		utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true, "orders": []models.FarmOrder{}})
		return
	}

	cursor, err = db.FarmOrdersCollection.Find(context.Background(), bson.M{"farmId": bson.M{"$in": farmIDs}})
	if err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to fetch orders"})
		return
	}
	defer cursor.Close(context.Background())

	var orders []models.FarmOrder
	if err := cursor.All(context.Background(), &orders); err != nil {
		utils.RespondWithJSON(w, http.StatusInternalServerError, utils.M{"success": false, "message": "Failed to decode orders"})
		return
	}

	if len(orders) == 0 {
		orders = []models.FarmOrder{}
	}

	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true, "orders": orders})
}

// POST /api/v1/farmorders/:id/accept
// func AcceptOrder(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	orderID := ps.ByName("id")
// 	// TODO: Mark order as accepted
// 	_ = orderID
// }

// POST /api/v1/farmorders/:id/reject
// func RejectOrder(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	orderID := ps.ByName("id")
// 	// TODO: Mark order as rejected
// 	_ = orderID
// }

// POST /api/v1/farmorders/:id/deliver
// func MarkOrderDelivered(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	orderID := ps.ByName("id")
// 	// TODO: Update delivery status to "Delivered"
// 	_ = orderID
// }

// POST /api/v1/farmorders/:id/markpaid
// func MarkOrderPaid(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	orderID := ps.ByName("id")
// 	// TODO: Update payment status to "Paid"
// 	_ = orderID
// }

func updateOrderStatus(w http.ResponseWriter, orderID string, newStatus string) {
	objID, err := primitive.ObjectIDFromHex(orderID)
	if err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Invalid order ID"})
		return
	}

	res, err := db.FarmOrdersCollection.UpdateOne(context.Background(),
		bson.M{"_id": objID},
		bson.M{"$set": bson.M{"status": newStatus}},
	)

	if err != nil || res.ModifiedCount == 0 {
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Order not found or already updated"})
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true, "status": newStatus})
}

func AcceptOrder(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	updateOrderStatus(w, ps.ByName("id"), "accepted")
}

func RejectOrder(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	updateOrderStatus(w, ps.ByName("id"), "rejected")
}

func MarkOrderDelivered(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	updateOrderStatus(w, ps.ByName("id"), "delivered")
}

func MarkOrderPaid(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	updateOrderStatus(w, ps.ByName("id"), "paid")
}

// GET /api/v1/farmorders/:id/receipt
func DownloadReceipt(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	orderID := ps.ByName("id")
	objID, err := primitive.ObjectIDFromHex(orderID)
	if err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, utils.M{"success": false, "message": "Invalid order ID"})
		return
	}

	var order models.FarmOrder
	err = db.FarmOrdersCollection.FindOne(context.Background(), bson.M{"_id": objID}).Decode(&order)
	if err != nil {
		utils.RespondWithJSON(w, http.StatusNotFound, utils.M{"success": false, "message": "Order not found"})
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, utils.M{
		"success": true,
		"receipt": order, // for now, just returning full order
	})
}
