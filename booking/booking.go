package booking

import (
	"context"
	"encoding/json"
	"naevis/db"
	"naevis/utils"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ---------- Utility ----------
func genID() string {
	return utils.GenerateRandomDigitString(22)
}

// ---------- Models ----------
type Slot struct {
	ID         string `json:"id" bson:"id"`
	EntityType string `json:"entityType" bson:"entityType"`
	EntityId   string `json:"entityId" bson:"entityId"`
	Date       string `json:"date" bson:"date"`
	Start      string `json:"start" bson:"start"`
	End        string `json:"end,omitempty" bson:"end,omitempty"`
	Capacity   int    `json:"capacity" bson:"capacity"`
	TierId     string `json:"tierId,omitempty" bson:"tierId,omitempty"`
	TierName   string `json:"tierName,omitempty" bson:"tierName,omitempty"`
	CreatedAt  int64  `json:"createdAt" bson:"createdAt"`
}

type Booking struct {
	ID         string  `json:"id" bson:"id"`
	SlotId     string  `json:"slotId,omitempty" bson:"slotId,omitempty"`
	TierId     string  `json:"tierId,omitempty" bson:"tierId,omitempty"`
	TierName   string  `json:"tierName,omitempty" bson:"tierName,omitempty"`
	PricePaid  float64 `json:"pricePaid,omitempty" bson:"pricePaid,omitempty"`
	EntityType string  `json:"entityType" bson:"entityType"`
	EntityId   string  `json:"entityId" bson:"entityId"`
	UserId     string  `json:"userId" bson:"userId"`
	Date       string  `json:"date" bson:"date"`
	Start      string  `json:"start" bson:"start"`
	End        string  `json:"end,omitempty" bson:"end,omitempty"`
	Status     string  `json:"status" bson:"status"` // pending, confirmed, cancelled
	CreatedAt  int64   `json:"createdAt" bson:"createdAt"`
}

type DateCap struct {
	EntityType string `json:"entityType" bson:"entityType"`
	EntityId   string `json:"entityId" bson:"entityId"`
	Date       string `json:"date" bson:"date"`
	Capacity   int    `json:"capacity" bson:"capacity"`
}

type Tier struct {
	ID         string   `json:"id" bson:"id"`
	EntityType string   `json:"entityType" bson:"entityType"`
	EntityId   string   `json:"entityId" bson:"entityId"`
	Name       string   `json:"name" bson:"name"`
	Price      float64  `json:"price" bson:"price"`
	Capacity   int      `json:"capacity" bson:"capacity"`
	TimeRange  []string `json:"timeRange,omitempty" bson:"timeRange,omitempty"`   // ["09:00", "17:00"]
	DaysOfWeek []int    `json:"daysOfWeek,omitempty" bson:"daysOfWeek,omitempty"` // 0=Sun..6=Sat
	Features   []string `json:"features,omitempty" bson:"features,omitempty"`
	CreatedAt  int64    `json:"createdAt" bson:"createdAt"`
}

// ---------- Tier handlers ----------
func ListTiers(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	entityType := r.URL.Query().Get("entityType")
	entityId := r.URL.Query().Get("entityId")

	filter := bson.M{}
	if entityType != "" {
		filter["entityType"] = entityType
	}
	if entityId != "" {
		filter["entityId"] = entityId
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cur, err := db.TiersCollection.Find(ctx, filter)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer cur.Close(ctx)

	var tiers []Tier
	for cur.Next(ctx) {
		var t Tier
		cur.Decode(&t)
		tiers = append(tiers, t)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"tiers": tiers})
}

func CreateTier(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var tier Tier
	if err := json.NewDecoder(r.Body).Decode(&tier); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// basic validation
	if tier.ID == "" || tier.EntityType == "" || tier.EntityId == "" || tier.Name == "" {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}

	tier.CreatedAt = time.Now().Unix()

	// insert into Mongo
	if _, err := db.TiersCollection.InsertOne(r.Context(), tier); err != nil {
		http.Error(w, "db insert failed", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]any{"tier": tier})
}

func DeleteTier(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tierId := ps.ByName("id")
	if tierId == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.TiersCollection.DeleteOne(ctx, bson.M{"id": tierId})
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	// Note: we do not automatically delete slots/bookings tied to this tier here.
	// If you want that behaviour, add deletion of slots/bookings with tierId == tierId.
	w.WriteHeader(http.StatusNoContent)
}

// GenerateSlotsFromTier expects JSON: { "startDate": "YYYY-MM-DD", "endDate": "YYYY-MM-DD" }
func GenerateSlotsFromTier(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tierId := ps.ByName("id")
	if tierId == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	var body struct {
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if body.StartDate == "" || body.EndDate == "" {
		http.Error(w, "missing date range", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var tier Tier
	if err := db.TiersCollection.FindOne(ctx, bson.M{"id": tierId}).Decode(&tier); err != nil {
		http.Error(w, "tier not found", http.StatusNotFound)
		return
	}

	startDate, err1 := time.Parse("2006-01-02", body.StartDate)
	endDate, err2 := time.Parse("2006-01-02", body.EndDate)
	if err1 != nil || err2 != nil || startDate.After(endDate) {
		http.Error(w, "invalid date range", http.StatusBadRequest)
		return
	}

	var slots []Slot
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dow := int(d.Weekday())
		if len(tier.DaysOfWeek) > 0 {
			allowed := false
			for _, allowedDay := range tier.DaysOfWeek {
				if allowedDay == dow {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}

		start := "09:00"
		end := "17:00"
		if len(tier.TimeRange) == 2 {
			start = tier.TimeRange[0]
			end = tier.TimeRange[1]
		}

		s := Slot{
			ID:         genID(),
			EntityType: tier.EntityType,
			EntityId:   tier.EntityId,
			Date:       d.Format("2006-01-02"),
			Start:      start,
			End:        end,
			Capacity:   tier.Capacity,
			TierId:     tier.ID,
			TierName:   tier.Name,
			CreatedAt:  time.Now().Unix(),
		}
		slots = append(slots, s)
	}

	if len(slots) > 0 {
		docs := make([]interface{}, len(slots))
		for i, s := range slots {
			docs[i] = s
		}
		_, err := db.SlotCollection.InsertMany(ctx, docs)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "slots": slots})
}

// ---------- Slots ----------
func ListSlots(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	entityType := r.URL.Query().Get("entityType")
	entityId := r.URL.Query().Get("entityId")
	filter := bson.M{}
	if entityType != "" {
		filter["entityType"] = entityType
	}
	if entityId != "" {
		filter["entityId"] = entityId
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cur, err := db.SlotCollection.Find(ctx, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cur.Close(ctx)
	var slots []Slot
	for cur.Next(ctx) {
		var s Slot
		cur.Decode(&s)
		slots = append(slots, s)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"slots": slots})
}

func CreateSlot(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var s Slot
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if s.EntityType == "" || s.EntityId == "" || s.Date == "" || s.Start == "" || s.Capacity <= 0 {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}

	// If a tierId was provided, attempt to fetch its name for convenience
	if s.TierId != "" {
		ctxTmp, cancelTmp := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancelTmp()
		var t Tier
		if err := db.TiersCollection.FindOne(ctxTmp, bson.M{"id": s.TierId}).Decode(&t); err == nil {
			s.TierName = t.Name
		}
	}

	s.ID = genID()
	s.CreatedAt = time.Now().Unix()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.SlotCollection.InsertOne(ctx, s)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"slot": s})
}

func DeleteSlot(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	slotId := ps.ByName("id")
	if slotId == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.SlotCollection.DeleteOne(ctx, bson.M{"id": slotId})
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	_, _ = db.BookingsCollection.DeleteMany(ctx, bson.M{"slotId": slotId})
	w.WriteHeader(http.StatusNoContent)
}

// ---------- Bookings ----------
func ListBookings(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	entityType := r.URL.Query().Get("entityType")
	entityId := r.URL.Query().Get("entityId")
	status := r.URL.Query().Get("status")

	filter := bson.M{}
	if entityType != "" {
		filter["entityType"] = entityType
	}
	if entityId != "" {
		filter["entityId"] = entityId
	}
	if status != "" {
		filter["status"] = status
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cur, err := db.BookingsCollection.Find(ctx, filter)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer cur.Close(ctx)
	var bookings []Booking
	for cur.Next(ctx) {
		var b Booking
		cur.Decode(&b)
		bookings = append(bookings, b)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"bookings": bookings})
}

func CreateBooking(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var p Booking
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if p.UserId == "" || p.EntityType == "" || p.EntityId == "" || p.Date == "" || p.Start == "" {
		http.Error(w, "missing fields", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// enforce one booking per user per date (excluding cancelled)
	count, err := db.BookingsCollection.CountDocuments(ctx, bson.M{
		"entityType": p.EntityType, "entityId": p.EntityId,
		"userId": p.UserId, "date": p.Date, "status": bson.M{"$ne": "cancelled"},
	})
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if count > 0 {
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "reason": "one-per-day"})
		return
	}

	// SLOT-BASED booking: check slot exists and capacity
	if p.SlotId != "" {
		var slot Slot
		if err := db.SlotCollection.FindOne(ctx, bson.M{"id": p.SlotId}).Decode(&slot); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "reason": "slot-missing"})
			return
		}
		// count non-cancelled bookings for that slot and sum seats
		// Note: if you store seats per booking, you should sum seats; current model doesn't have seats field on Booking — if you add seats, adapt this to sum.
		slotCount, err := db.BookingsCollection.CountDocuments(ctx, bson.M{
			"entityType": p.EntityType, "entityId": p.EntityId,
			"slotId": p.SlotId, "status": bson.M{"$ne": "cancelled"},
		})
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if int(slotCount) >= slot.Capacity {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "reason": "slot-full"})
			return
		}
		// attach tier info for convenience if slot has it
		if slot.TierId != "" {
			p.TierId = slot.TierId
			p.TierName = slot.TierName
		}
	}

	// TIER-BASED booking (no slot, but tier specified): enforce tier capacity per date
	if p.SlotId == "" && p.TierId != "" {
		var tier Tier
		if err := db.TiersCollection.FindOne(ctx, bson.M{"id": p.TierId}).Decode(&tier); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "reason": "tier-missing"})
			return
		}
		// count bookings for this entity/date/tier (excluding cancelled)
		tCount, err := db.BookingsCollection.CountDocuments(ctx, bson.M{
			"entityType": p.EntityType, "entityId": p.EntityId,
			"tierId": p.TierId, "date": p.Date, "status": bson.M{"$ne": "cancelled"},
		})
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if int(tCount) >= tier.Capacity {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "reason": "tier-full"})
			return
		}
		// attach tier name & price by default if not provided
		p.TierName = tier.Name
		if p.PricePaid == 0 {
			p.PricePaid = tier.Price
		}
	}

	// global date capacity (for custom bookings without slot)
	if p.SlotId == "" && p.TierId == "" {
		var dc DateCap
		err := db.DateCapsCollection.FindOne(ctx, bson.M{
			"entityType": p.EntityType, "entityId": p.EntityId, "date": p.Date,
		}).Decode(&dc)
		if err == nil {
			totalCount, err := db.BookingsCollection.CountDocuments(ctx, bson.M{
				"entityType": p.EntityType, "entityId": p.EntityId, "date": p.Date, "status": bson.M{"$ne": "cancelled"},
			})
			if err != nil {
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}
			if int(totalCount) >= dc.Capacity {
				json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "reason": "date-full"})
				return
			}
		}
	}

	p.ID = genID()
	p.Status = "pending"
	p.CreatedAt = time.Now().Unix()

	_, err = db.BookingsCollection.InsertOne(ctx, p)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "booking": p})
}

// update booking status
func UpdateBookingStatus(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	bookingId := ps.ByName("id")
	if bookingId == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if body.Status != "pending" && body.Status != "confirmed" && body.Status != "cancelled" {
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res := db.BookingsCollection.FindOneAndUpdate(ctx,
		bson.M{"id": bookingId},
		bson.M{"$set": bson.M{"status": body.Status}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	var updated Booking
	if err := res.Decode(&updated); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "booking": updated})
}

// cancel booking (shortcut, idempotent)
func CancelBooking(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	bookingId := ps.ByName("id")
	if bookingId == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res := db.BookingsCollection.FindOneAndUpdate(ctx,
		bson.M{"id": bookingId},
		bson.M{"$set": bson.M{"status": "cancelled"}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)

	var updated Booking
	if err := res.Decode(&updated); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"booking": updated,
	})
}

// ---------- Date capacity ----------
func GetDateCapacity(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	entityType := r.URL.Query().Get("entityType")
	entityId := r.URL.Query().Get("entityId")
	date := r.URL.Query().Get("date")
	if entityType == "" || entityId == "" || date == "" {
		http.Error(w, "missing params", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var dc DateCap
	err := db.DateCapsCollection.FindOne(ctx, bson.M{
		"entityType": entityType, "entityId": entityId, "date": date,
	}).Decode(&dc)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"capacity": nil})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"capacity": dc.Capacity})
}

func SetDateCapacity(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var p DateCap
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if p.EntityType == "" || p.EntityId == "" || p.Date == "" || p.Capacity <= 0 {
		http.Error(w, "missing fields", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.DateCapsCollection.UpdateOne(ctx,
		bson.M{"entityType": p.EntityType, "entityId": p.EntityId, "date": p.Date},
		bson.M{"$set": p},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}
