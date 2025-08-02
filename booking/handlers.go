package booking

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"naevis/db"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Models

type Booking struct {
	EntityType string                 `bson:"entityType"`
	EntityId   string                 `bson:"entityId"`
	SlotId     string                 `bson:"slotId"`
	Group      string                 `bson:"group"`
	UserId     string                 `bson:"userId"`
	Metadata   map[string]interface{} `bson:"metadata,omitempty"`
	Timestamp  int64                  `bson:"timestamp"`
}

type Config struct {
	EntityType string              `bson:"entityType"`
	EntityId   string              `bson:"entityId"`
	SlotGroups map[string][]string `bson:"slotGroups"` // e.g., {"morning": ["1-3"], "evening": ["6-8"]}
	SlotMeta   map[string]SlotMeta `bson:"slotMeta"`   // optional metadata per slotId
}

type SlotMeta struct {
	Label     string   `bson:"label,omitempty"`
	Price     int      `bson:"price,omitempty"`
	Location  string   `bson:"location,omitempty"`
	TimeStart int64    `bson:"timeStart,omitempty"`
	TimeEnd   int64    `bson:"timeEnd,omitempty"`
	Tags      []string `bson:"tags,omitempty"`
}

type WSMessage struct {
	Type string `json:"type"`
}

func broadcastUpdate(topic string) {
	msg := WSMessage{Type: "update"}
	data, _ := json.Marshal(msg)
	broadcast(topic, data)
}

// expandSlotRanges expands ["1-3", "5"] -> ["1", "2", "3", "5"]
func expandSlotRanges(ranges []string) []string {
	slots := []string{}
	for _, r := range ranges {
		if strings.Contains(r, "-") {
			parts := strings.Split(r, "-")
			start, _ := strconv.Atoi(parts[0])
			end, _ := strconv.Atoi(parts[1])
			for i := start; i <= end; i++ {
				slots = append(slots, strconv.Itoa(i))
			}
		} else {
			slots = append(slots, r)
		}
	}
	return slots
}

// GET /availability/:entityType/:entityId/:group
func GetAvailability(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entityType")
	entityId := ps.ByName("entityId")
	group := ps.ByName("group")

	w.Header().Set("Content-Type", "application/json")

	var config Config
	err := db.ConfigsCollection.FindOne(context.TODO(), bson.M{
		"entityType": entityType,
		"entityId":   entityId,
	}).Decode(&config)
	if err != nil {
		http.Error(w, `{"error":"Config not found"}`, http.StatusNotFound)
		return
	}

	slotRange, ok := config.SlotGroups[group]
	if !ok {
		http.Error(w, `{"error":"Group not found"}`, http.StatusBadRequest)
		return
	}

	slotIds := expandSlotRanges(slotRange)

	cursor, err := db.BookingsCollection.Find(context.TODO(), bson.M{
		"entityType": entityType,
		"entityId":   entityId,
	})
	if err != nil {
		http.Error(w, `{"error":"DB error"}`, http.StatusInternalServerError)
		return
	}
	defer cursor.Close(context.TODO())

	booked := map[string]bool{}
	for cursor.Next(context.TODO()) {
		var b Booking
		if err := cursor.Decode(&b); err != nil {
			continue
		}
		booked[b.SlotId] = true
	}

	available := []string{}
	for _, slot := range slotIds {
		if !booked[slot] {
			available = append(available, slot)
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"available": available,
	})
}

// POST /book
func BookSlot(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")

	type Req struct {
		EntityType string                 `json:"entityType"`
		EntityId   string                 `json:"entityId"`
		Group      string                 `json:"group"`
		UserId     string                 `json:"userId"`
		Metadata   map[string]interface{} `json:"metadata,omitempty"`
	}
	var req Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid body"}`, http.StatusBadRequest)
		return
	}

	var config Config
	err := db.ConfigsCollection.FindOne(context.TODO(), bson.M{
		"entityType": req.EntityType,
		"entityId":   req.EntityId,
	}).Decode(&config)
	if err != nil {
		http.Error(w, `{"error":"Config not found"}`, http.StatusNotFound)
		return
	}

	slotRange, ok := config.SlotGroups[req.Group]
	if !ok {
		http.Error(w, `{"error":"Group not found"}`, http.StatusBadRequest)
		return
	}

	possibleSlots := expandSlotRanges(slotRange)

	for _, slot := range possibleSlots {
		filter := bson.M{
			"entityType": req.EntityType,
			"entityId":   req.EntityId,
			"slotId":     slot,
		}
		update := bson.M{
			"$setOnInsert": Booking{
				EntityType: req.EntityType,
				EntityId:   req.EntityId,
				SlotId:     slot,
				Group:      req.Group,
				UserId:     req.UserId,
				Metadata:   req.Metadata,
				Timestamp:  time.Now().Unix(),
			},
		}
		opts := options.Update().SetUpsert(true)

		result, err := db.BookingsCollection.UpdateOne(context.TODO(), filter, update, opts)
		if err == nil && result.UpsertedCount == 1 {
			broadcastUpdate(req.EntityType + "_" + req.EntityId)
			json.NewEncoder(w).Encode(map[string]string{"booked": slot})
			return
		}
	}

	http.Error(w, `{"error":"No available slots"}`, http.StatusConflict)
}

// POST /slots
func CreateSlots(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")

	type Req struct {
		EntityType string              `json:"entityType"`
		EntityId   string              `json:"entityId"`
		Group      string              `json:"group"`
		Range      []string            `json:"range"`    // e.g., ["1-3", "5"]
		SlotMeta   map[string]SlotMeta `json:"slotMeta"` // optional
	}
	var req Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid body"}`, http.StatusBadRequest)
		return
	}

	if req.EntityType == "" || req.EntityId == "" || req.Group == "" || len(req.Range) == 0 {
		http.Error(w, `{"error":"Missing required fields"}`, http.StatusBadRequest)
		return
	}

	filter := bson.M{
		"entityType": req.EntityType,
		"entityId":   req.EntityId,
	}
	update := bson.M{
		"$set": bson.M{
			"slotGroups." + req.Group: req.Range,
		},
	}
	if req.SlotMeta != nil {
		update["$set"].(bson.M)["slotMeta"] = req.SlotMeta
	}

	opts := options.Update().SetUpsert(true)

	_, err := db.ConfigsCollection.UpdateOne(context.TODO(), filter, update, opts)
	if err != nil {
		http.Error(w, `{"error":"DB error"}`, http.StatusInternalServerError)
		return
	}

	broadcastUpdate(req.EntityType + "_" + req.EntityId)
	json.NewEncoder(w).Encode(map[string]string{"status": "created"})
}

// DELETE /book/:entityType/:entityId/:slotId?userId=u123
func CancelBooking(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entityType")
	entityId := ps.ByName("entityId")
	slotId := ps.ByName("slotId")
	userId := r.URL.Query().Get("userId")

	w.Header().Set("Content-Type", "application/json")

	if userId == "" {
		http.Error(w, `{"error":"Missing userId"}`, http.StatusBadRequest)
		return
	}

	filter := bson.M{
		"entityType": entityType,
		"entityId":   entityId,
		"slotId":     slotId,
		"userId":     userId, // only allow cancelling own booking
	}

	result, err := db.BookingsCollection.DeleteOne(context.TODO(), filter)
	if err != nil {
		http.Error(w, `{"error":"DB error"}`, http.StatusInternalServerError)
		return
	}

	if result.DeletedCount == 0 {
		http.Error(w, `{"error":"Booking not found or not owned by user"}`, http.StatusNotFound)
		return
	}

	broadcastUpdate(entityType + "_" + entityId)
	json.NewEncoder(w).Encode(map[string]string{
		"cancelled": slotId,
	})
}
