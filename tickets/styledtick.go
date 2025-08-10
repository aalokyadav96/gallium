package tickets

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/phpdave11/gofpdf"
	"github.com/skip2/go-qrcode"
	"go.mongodb.org/mongo-driver/bson"

	"naevis/db"
	"naevis/middleware"
	"naevis/models"
)

func PrintStyledTicket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	eventID := ps.ByName("eventid")
	ticketID := ps.ByName("ticketid")
	uniqueCode := r.URL.Query().Get("uniqueCode")
	token := r.Header.Get("Authorization")

	// Validate JWT
	claims, err := middleware.ValidateJWT(token)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if uniqueCode == "" {
		http.Error(w, "Missing unique code", http.StatusBadRequest)
		return
	}

	// Fetch ticket
	var ticket models.PurchasedTicket
	err = db.PurchasedTicketsCollection.FindOne(context.TODO(), bson.M{
		"eventid":    eventID,
		"ticketid":   ticketID,
		"uniquecode": uniqueCode,
	}).Decode(&ticket)
	if err != nil {
		http.Error(w, "Ticket not found", http.StatusNotFound)
		return
	}

	ticket.BuyerName = claims.Username
	timestamp := time.Now().Unix()

	// Generate QR content with timestamp
	qrData := fmt.Sprintf("eid=%s&tid=%s&code=%s&ts=%d", eventID, ticketID, uniqueCode, timestamp)
	qrCode, _ := qrcode.Encode(qrData, qrcode.Medium, 128)

	// Generate PDF
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.AddPage()
	pdf.SetFillColor(245, 245, 255)

	// Title
	pdf.SetFont("Arial", "B", 20)
	pdf.CellFormat(0, 15, "🎫 Your Event Ticket", "", 1, "C", false, 0, "")
	pdf.Ln(5)

	// Buyer Info
	pdf.SetFont("Arial", "", 12)
	pdf.MultiCell(0, 10, fmt.Sprintf(
		"Name: %s\nEvent ID: %s\nTicket ID: %s\nIssued: %s",
		ticket.BuyerName,
		eventID,
		ticketID,
		time.Now().Format("02 Jan 2006 15:04"),
	), "", "L", false)

	// QR Code Image
	imgOpts := gofpdf.ImageOptions{ImageType: "png"}
	pdf.RegisterImageOptionsReader("qr", imgOpts, bytes.NewReader(qrCode))
	pdf.ImageOptions("qr", 150, 60, 40, 40, false, imgOpts, 0, "")

	// Footer
	pdf.SetY(-30)
	pdf.SetFont("Arial", "I", 10)
	pdf.CellFormat(0, 10, "Show this ticket at entry. Scanners will validate with timestamp.", "T", 0, "C", false, 0, "")

	var buf bytes.Buffer
	err = pdf.Output(&buf)
	if err != nil {
		http.Error(w, "Failed to generate ticket", http.StatusInternalServerError)
		return
	}

	// Send PDF
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=ticket-"+ticketID+".pdf")
	w.Write(buf.Bytes())
}
