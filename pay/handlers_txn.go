package pay

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"naevis/db"
	"naevis/models"
	"naevis/rdx"
	"naevis/utils"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// lockTTL defines the duration to hold Redis lock per user
const lockTTL = 5 * time.Second

// --- TopUp ---
func (p *PaymentService) TopUp(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := context.Background()
	userID := utils.GetUserIDFromRequest(r)

	var body struct {
		Amount float64 `json:"amount"`
		Method string  `json:"method"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Amount <= 0 {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey != "" {
		var existing models.Transaction
		if err := db.TransactionCollection.FindOne(ctx, bson.M{"external_ref": idempotencyKey, "type": "topup"}).Decode(&existing); err == nil {
			utils.RespondWithJSON(w, http.StatusOK, existing)
			return
		}
	}

	// Acquire Redis lock per user
	acquired, err := rdx.RdxSetNX("wallet_lock:"+userID, "1", lockTTL)
	if err != nil || !acquired {
		http.Error(w, "please retry", http.StatusTooManyRequests)
		return
	}
	defer rdx.RdxDel("wallet_lock:" + userID)

	userAccID, err := getOrCreateAccount(ctx, userID)
	if err != nil {
		http.Error(w, "account error", http.StatusInternalServerError)
		return
	}

	txn := models.Transaction{
		ID:             utils.GetUUID(),
		UserID:         userID,
		Type:           "topup",
		Method:         body.Method,
		FromAccount:    "external:bank",
		ToAccount:      userAccID,
		Amount:         body.Amount,
		Currency:       "INR",
		Status:         "initiated",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		IdempotencyKey: idempotencyKey,
		Meta:           models.Meta{"note": "topup"},
	}

	// Insert transaction record first
	if _, err := db.TransactionCollection.InsertOne(ctx, txn); err != nil {
		http.Error(w, "topup failed", http.StatusInternalServerError)
		return
	}

	// Insert journal entry
	journal := models.JournalEntry{
		ID:            utils.GetUUID(),
		TxnID:         txn.ID,
		DebitAccount:  "external:bank",
		CreditAccount: userAccID,
		Amount:        body.Amount,
		Currency:      "INR",
		CreatedAt:     time.Now(),
		Meta:          models.Meta{"note": "topup"},
	}
	if _, err := db.JournalCollection.InsertOne(ctx, journal); err != nil {
		txn.Status = "failed"
		txn.UpdatedAt = time.Now()
		_, _ = db.TransactionCollection.UpdateOne(ctx, bson.M{"_id": txn.ID}, bson.M{"$set": txn})
		http.Error(w, "topup failed", http.StatusInternalServerError)
		return
	}

	// Update account balance
	if _, err := db.AccountsCollection.UpdateOne(ctx, bson.M{"_id": userAccID}, bson.M{
		"$inc": bson.M{"cached_balance": body.Amount, "version": 1},
		"$set": bson.M{"updated_at": time.Now()},
	}); err != nil {
		txn.Status = "failed"
		txn.UpdatedAt = time.Now()
		_, _ = db.TransactionCollection.UpdateOne(ctx, bson.M{"_id": txn.ID}, bson.M{"$set": txn})
		http.Error(w, "topup failed", http.StatusInternalServerError)
		return
	}

	// Mark success
	txn.Status = "success"
	txn.UpdatedAt = time.Now()
	_, _ = db.TransactionCollection.UpdateOne(ctx, bson.M{"_id": txn.ID}, bson.M{"$set": txn})

	utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success":        true,
		"transaction_id": txn.ID,
	})
}

// --- Pay ---
func (p *PaymentService) Pay(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	userID := utils.GetUserIDFromRequest(r)

	var req models.PayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.EntityType == "user" {
		utils.RespondWithJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "Use /wallet/transfer"})
		return
	}

	resolver, err := p.GetResolver(req.EntityType)
	if err != nil {
		utils.RespondWithJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "message": "Unsupported entity type"})
		return
	}

	price, err := resolver(ctx, req.EntityID)
	if err != nil {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}
	if req.Amount > 0 {
		price = req.Amount
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey != "" {
		var existing models.Transaction
		if err := db.TransactionCollection.FindOne(ctx, bson.M{
			"external_ref": idempotencyKey,
			"type":         "payment",
		}).Decode(&existing); err == nil {
			utils.RespondWithJSON(w, http.StatusOK, existing)
			return
		}
	}

	// Lock payer
	acquired, err := rdx.RdxSetNX("wallet_lock:"+userID, "1", lockTTL)
	if err != nil || !acquired {
		http.Error(w, "please retry", http.StatusTooManyRequests)
		return
	}
	defer rdx.RdxDel("wallet_lock:" + userID)

	userAccID, err := getOrCreateAccount(ctx, userID)
	if err != nil {
		http.Error(w, "account error", http.StatusInternalServerError)
		return
	}

	merchantAccID := "merchant:default"
	_, _ = getOrCreateAccount(ctx, merchantAccID)

	txn := models.Transaction{
		ID:             utils.GetUUID(),
		UserID:         userID, // fix: populate userID
		Type:           "payment",
		Method:         req.Method,
		FromAccount:    userAccID,
		ToAccount:      merchantAccID,
		Amount:         price,
		Currency:       "INR",
		Status:         "initiated", // initial state
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		IdempotencyKey: idempotencyKey,
		Meta: models.Meta{
			"entity_id":   req.EntityID,
			"entity_type": req.EntityType,
			"note":        "payment",
		},
	}

	// Insert txn
	if _, err := db.TransactionCollection.InsertOne(ctx, txn); err != nil {
		http.Error(w, "payment failed", http.StatusInternalServerError)
		return
	}

	// Verify balance
	var payerAcc struct {
		CachedBalance float64 `bson:"cached_balance"`
	}
	if err := db.AccountsCollection.FindOne(ctx, bson.M{"_id": userAccID}).Decode(&payerAcc); err != nil {
		txn.Status = "failed"
		txn.UpdatedAt = time.Now()
		_, _ = db.TransactionCollection.UpdateOne(ctx, bson.M{"_id": txn.ID}, bson.M{"$set": txn})
		http.Error(w, "payment failed", http.StatusInternalServerError)
		return
	}
	if req.Method == "wallet" && payerAcc.CachedBalance < price {
		utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{"success": false, "message": "Insufficient wallet balance"})
		return
	}

	// Insert journal
	j := models.JournalEntry{
		ID:            utils.GetUUID(),
		TxnID:         txn.ID,
		DebitAccount:  userAccID,
		CreditAccount: merchantAccID,
		Amount:        price,
		Currency:      "INR",
		CreatedAt:     time.Now(),
		Meta:          models.Meta{"note": "payment"},
	}
	if _, err := db.JournalCollection.InsertOne(ctx, j); err != nil {
		txn.Status = "failed"
		txn.UpdatedAt = time.Now()
		_, _ = db.TransactionCollection.UpdateOne(ctx, bson.M{"_id": txn.ID}, bson.M{"$set": txn})
		http.Error(w, "payment failed", http.StatusInternalServerError)
		return
	}

	// Update balances
	if _, err := db.AccountsCollection.UpdateOne(ctx, bson.M{"_id": userAccID}, bson.M{
		"$inc": bson.M{"cached_balance": -price, "version": 1},
		"$set": bson.M{"updated_at": time.Now()},
	}); err != nil {
		txn.Status = "failed"
		txn.UpdatedAt = time.Now()
		_, _ = db.TransactionCollection.UpdateOne(ctx, bson.M{"_id": txn.ID}, bson.M{"$set": txn})
		http.Error(w, "payment failed", http.StatusInternalServerError)
		return
	}

	if _, err := db.AccountsCollection.UpdateOne(ctx, bson.M{"_id": merchantAccID}, bson.M{
		"$inc": bson.M{"cached_balance": price, "version": 1},
		"$set": bson.M{"updated_at": time.Now()},
	}); err != nil {
		txn.Status = "failed"
		txn.UpdatedAt = time.Now()
		_, _ = db.TransactionCollection.UpdateOne(ctx, bson.M{"_id": txn.ID}, bson.M{"$set": txn})
		http.Error(w, "payment failed", http.StatusInternalServerError)
		return
	}

	txn.Status = "success"
	txn.UpdatedAt = time.Now()
	_, _ = db.TransactionCollection.UpdateOne(ctx, bson.M{"_id": txn.ID}, bson.M{"$set": txn})

	utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{"success": true, "transaction_id": txn.ID})
}

// --- Transfer ---
func (p *PaymentService) Transfer(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	senderID := utils.GetUserIDFromRequest(r)

	var body struct {
		Recipient string  `json:"recipient"`
		Amount    float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Recipient == "" || body.Amount <= 0 {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Idempotency
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey != "" {
		var existing models.Transaction
		if err := db.TransactionCollection.FindOne(ctx, bson.M{"external_ref": idempotencyKey, "type": "transfer"}).Decode(&existing); err == nil {
			utils.RespondWithJSON(w, http.StatusOK, existing)
			return
		}
	}

	senderAccID, err := getOrCreateAccount(ctx, senderID)
	if err != nil {
		http.Error(w, "account error", http.StatusInternalServerError)
		return
	}
	recipientAccID, err := getOrCreateAccount(ctx, body.Recipient)
	if err != nil {
		http.Error(w, "recipient account error", http.StatusInternalServerError)
		return
	}

	// Acquire locks in deterministic order
	lockA := "wallet_lock:" + senderAccID
	lockB := "wallet_lock:" + recipientAccID
	if lockB < lockA {
		lockA, lockB = lockB, lockA
	}
	ok, err := rdx.RdxSetNX(lockA, "1", lockTTL)
	if err != nil || !ok {
		http.Error(w, "please retry", http.StatusTooManyRequests)
		return
	}
	defer rdx.RdxDel(lockA)

	ok2, err := rdx.RdxSetNX(lockB, "1", lockTTL)
	if err != nil || !ok2 {
		rdx.RdxDel(lockA)
		http.Error(w, "please retry", http.StatusTooManyRequests)
		return
	}
	defer rdx.RdxDel(lockB)

	masterTxn := models.Transaction{
		ID:             utils.GetUUID(),
		Type:           "transfer",
		Method:         "transfer",
		FromAccount:    senderAccID,
		ToAccount:      recipientAccID,
		Amount:         body.Amount,
		Currency:       "INR",
		Status:         "initiated",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		IdempotencyKey: idempotencyKey,
		Meta:           models.Meta{"note": "transfer"},
	}

	// Insert master transaction
	if _, err := db.TransactionCollection.InsertOne(ctx, masterTxn); err != nil {
		http.Error(w, "transfer failed", http.StatusInternalServerError)
		return
	}

	// Check sender balance while locks are held
	var senderAcc struct {
		CachedBalance float64 `bson:"cached_balance"`
	}
	if err := db.AccountsCollection.FindOne(ctx, bson.M{"_id": senderAccID}).Decode(&senderAcc); err != nil {
		masterTxn.Status = "failed"
		masterTxn.UpdatedAt = time.Now()
		_, _ = db.TransactionCollection.UpdateOne(ctx, bson.M{"_id": masterTxn.ID}, bson.M{"$set": masterTxn})
		http.Error(w, "transfer failed", http.StatusInternalServerError)
		return
	}
	if senderAcc.CachedBalance < body.Amount {
		utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{"success": false, "message": "insufficient balance"})
		return
	}

	// Insert journal entry
	j := models.JournalEntry{
		ID:            utils.GetUUID(),
		TxnID:         masterTxn.ID,
		DebitAccount:  senderAccID,
		CreditAccount: recipientAccID,
		Amount:        body.Amount,
		Currency:      "INR",
		CreatedAt:     time.Now(),
		Meta:          models.Meta{"note": "transfer"},
	}
	if _, err := db.JournalCollection.InsertOne(ctx, j); err != nil {
		masterTxn.Status = "failed"
		masterTxn.UpdatedAt = time.Now()
		_, _ = db.TransactionCollection.UpdateOne(ctx, bson.M{"_id": masterTxn.ID}, bson.M{"$set": masterTxn})
		http.Error(w, "transfer failed", http.StatusInternalServerError)
		return
	}

	// Update balances
	if _, err := db.AccountsCollection.UpdateOne(ctx, bson.M{"_id": senderAccID}, bson.M{
		"$inc": bson.M{"cached_balance": -body.Amount, "version": 1},
		"$set": bson.M{"updated_at": time.Now()},
	}); err != nil {
		masterTxn.Status = "failed"
		masterTxn.UpdatedAt = time.Now()
		_, _ = db.TransactionCollection.UpdateOne(ctx, bson.M{"_id": masterTxn.ID}, bson.M{"$set": masterTxn})
		http.Error(w, "transfer failed", http.StatusInternalServerError)
		return
	}
	if _, err := db.AccountsCollection.UpdateOne(ctx, bson.M{"_id": recipientAccID}, bson.M{
		"$inc": bson.M{"cached_balance": body.Amount, "version": 1},
		"$set": bson.M{"updated_at": time.Now()},
	}); err != nil {
		// In standalone mongo we cannot atomically rollback sender decrement.
		// Mark txn failed and surface to be reconciled via journals.
		masterTxn.Status = "failed"
		masterTxn.UpdatedAt = time.Now()
		_, _ = db.TransactionCollection.UpdateOne(ctx, bson.M{"_id": masterTxn.ID}, bson.M{"$set": masterTxn})
		http.Error(w, "transfer failed", http.StatusInternalServerError)
		return
	}

	// Insert per-user debit/credit transactions (best-effort)
	debitTxn := models.Transaction{
		ID:         utils.GetUUID(),
		ParentTxn:  masterTxn.ID,
		UserID:     senderID,
		Type:       "debit",
		Method:     "transfer",
		EntityType: "user",
		EntityID:   body.Recipient,
		Amount:     body.Amount,
		Status:     "success",
		CreatedAt:  time.Now(),
		Meta:       models.Meta{"note": "transfer"},
	}
	creditTxn := models.Transaction{
		ID:         utils.GetUUID(),
		ParentTxn:  masterTxn.ID,
		UserID:     body.Recipient,
		Type:       "credit",
		Method:     "transfer",
		EntityType: "user",
		EntityID:   senderID,
		Amount:     body.Amount,
		Status:     "success",
		CreatedAt:  time.Now(),
		Meta:       models.Meta{"note": "transfer"},
	}
	_, _ = db.TransactionCollection.InsertMany(ctx, []interface{}{debitTxn, creditTxn})

	// Mark master txn success
	masterTxn.Status = "success"
	masterTxn.UpdatedAt = time.Now()
	_, _ = db.TransactionCollection.UpdateOne(ctx, bson.M{"_id": masterTxn.ID}, bson.M{"$set": masterTxn})

	utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{"success": true, "transaction_id": masterTxn.ID})
}

// --- Refund ---
func (p *PaymentService) Refund(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	requester := utils.GetUserIDFromRequest(r)
	_ = requester

	var body struct {
		TransactionID string `json:"transaction_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TransactionID == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey != "" {
		var existing models.Transaction
		if err := db.TransactionCollection.FindOne(ctx, bson.M{"external_ref": idempotencyKey, "type": "refund"}).Decode(&existing); err == nil {
			utils.RespondWithJSON(w, http.StatusOK, existing)
			return
		}
	}

	var origTxn models.Transaction
	if err := db.TransactionCollection.FindOne(ctx, bson.M{"_id": body.TransactionID}).Decode(&origTxn); err != nil {
		http.Error(w, "transaction not found", http.StatusNotFound)
		return
	}
	if origTxn.Status != "success" {
		http.Error(w, "cannot refund non-success transaction", http.StatusBadRequest)
		return
	}

	// money flows back from origTxn.ToAccount to origTxn.FromAccount
	fromAcc := origTxn.ToAccount
	toAcc := origTxn.FromAccount
	if fromAcc == "" || toAcc == "" {
		http.Error(w, "invalid original transaction accounts", http.StatusBadRequest)
		return
	}

	// Acquire locks in deterministic order
	lockA := "wallet_lock:" + fromAcc
	lockB := "wallet_lock:" + toAcc
	if lockB < lockA {
		lockA, lockB = lockB, lockA
	}
	ok, err := rdx.RdxSetNX(lockA, "1", lockTTL)
	if err != nil || !ok {
		http.Error(w, "please retry", http.StatusTooManyRequests)
		return
	}
	defer rdx.RdxDel(lockA)

	ok2, err := rdx.RdxSetNX(lockB, "1", lockTTL)
	if err != nil || !ok2 {
		rdx.RdxDel(lockA)
		http.Error(w, "please retry", http.StatusTooManyRequests)
		return
	}
	defer rdx.RdxDel(lockB)

	refundTxn := models.Transaction{
		ID:             utils.GetUUID(),
		Type:           "refund",
		Method:         "refund",
		FromAccount:    fromAcc,
		ToAccount:      toAcc,
		Amount:         origTxn.Amount,
		Currency:       origTxn.Currency,
		Status:         "initiated",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		IdempotencyKey: idempotencyKey,
		Meta:           models.Meta{"original_txn": origTxn.ID},
	}

	// Insert refund txn
	if _, err := db.TransactionCollection.InsertOne(ctx, refundTxn); err != nil {
		http.Error(w, "refund failed", http.StatusInternalServerError)
		return
	}

	// Insert reversing journal
	j := models.JournalEntry{
		ID:            utils.GetUUID(),
		TxnID:         refundTxn.ID,
		DebitAccount:  refundTxn.FromAccount,
		CreditAccount: refundTxn.ToAccount,
		Amount:        refundTxn.Amount,
		Currency:      refundTxn.Currency,
		CreatedAt:     time.Now(),
		Meta:          models.Meta{"note": "refund", "original_txn": origTxn.ID},
	}
	if _, err := db.JournalCollection.InsertOne(ctx, j); err != nil {
		refundTxn.Status = "failed"
		refundTxn.UpdatedAt = time.Now()
		_, _ = db.TransactionCollection.UpdateOne(ctx, bson.M{"_id": refundTxn.ID}, bson.M{"$set": refundTxn})
		http.Error(w, "refund failed", http.StatusInternalServerError)
		return
	}

	// Decrement original recipient (fromAcc)
	if _, err := db.AccountsCollection.UpdateOne(ctx, bson.M{"_id": refundTxn.FromAccount}, bson.M{
		"$inc": bson.M{"cached_balance": -refundTxn.Amount, "version": 1},
		"$set": bson.M{"updated_at": time.Now()},
	}); err != nil {
		refundTxn.Status = "failed"
		refundTxn.UpdatedAt = time.Now()
		_, _ = db.TransactionCollection.UpdateOne(ctx, bson.M{"_id": refundTxn.ID}, bson.M{"$set": refundTxn})
		http.Error(w, "refund failed", http.StatusInternalServerError)
		return
	}

	// Credit original payer (toAcc)
	if _, err := db.AccountsCollection.UpdateOne(ctx, bson.M{"_id": refundTxn.ToAccount}, bson.M{
		"$inc": bson.M{"cached_balance": refundTxn.Amount, "version": 1},
		"$set": bson.M{"updated_at": time.Now()},
	}); err != nil {
		// Cannot rollback the decrement on standalone mongo. Mark failed and surface for reconciliation.
		refundTxn.Status = "failed"
		refundTxn.UpdatedAt = time.Now()
		_, _ = db.TransactionCollection.UpdateOne(ctx, bson.M{"_id": refundTxn.ID}, bson.M{"$set": refundTxn})
		http.Error(w, "refund failed", http.StatusInternalServerError)
		return
	}

	// Mark refund success
	refundTxn.Status = "success"
	refundTxn.UpdatedAt = time.Now()
	_, _ = db.TransactionCollection.UpdateOne(ctx, bson.M{"_id": refundTxn.ID}, bson.M{"$set": refundTxn})

	// Best-effort mark original txn reversed
	_, _ = db.TransactionCollection.UpdateOne(ctx, bson.M{"_id": origTxn.ID}, bson.M{"$set": bson.M{"status": "reversed", "updated_at": time.Now()}})

	utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{"success": true, "transaction_id": refundTxn.ID})
}

// --- Helper: fetch or create account ---
func getOrCreateAccount(ctx context.Context, userID string) (string, error) {
	var acc models.Account
	err := db.AccountsCollection.FindOne(ctx, bson.M{"userid": userID}).Decode(&acc)
	if err == nil {
		return acc.ID, nil
	}

	newAcc := models.Account{
		ID:            utils.GetUUID(),
		UserID:        userID,
		Currency:      "INR",
		Status:        "active",
		CachedBalance: 0,
		Version:       1,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	_, err = db.AccountsCollection.InsertOne(ctx, newAcc)
	if err != nil {
		// If concurrent create happened, try to read again
		if err := db.AccountsCollection.FindOne(ctx, bson.M{"userid": userID}).Decode(&acc); err == nil {
			return acc.ID, nil
		}
		return "", err
	}

	return newAcc.ID, nil
}
