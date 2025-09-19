package newchat

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"naevis/db"
	"naevis/filemgr"
	"naevis/middleware"
	"naevis/utils"

	"github.com/julienschmidt/httprouter"
)

func UploadHandler(hub *Hub) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		// 1) Validate JWT
		token, err := extractToken(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		claims, err := middleware.ValidateJWT(token)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// 2) Parse multipart form
		if err := r.ParseMultipartForm(32 << 20); err != nil { // allow up to 32MB total
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		room := strings.TrimSpace(r.FormValue("chat"))
		if room == "" {
			http.Error(w, "room missing", http.StatusBadRequest)
			return
		}

		hdrs := r.MultipartForm.File["file"]
		if len(hdrs) == 0 {
			http.Error(w, "no files provided", http.StatusBadRequest)
			return
		}

		// 3) Save each file
		var attachments []Attachment
		for _, hdr := range hdrs {
			file, err := hdr.Open()
			if err != nil {
				http.Error(w, "file error", http.StatusBadRequest)
				return
			}
			savedName, err := filemgr.SaveFileForEntity(file, hdr, filemgr.EntityChat, filemgr.PicPhoto)
			file.Close()
			if err != nil {
				log.Println("save failed:", err)
				http.Error(w, "save failed", http.StatusInternalServerError)
				return
			}
			attachments = append(attachments, Attachment{
				Filename: hdr.Filename,
				Path:     savedName,
			})
		}

		// 4) Build DB message
		ts := time.Now().Unix()
		msg := Message{
			MessageID: utils.GenerateRandomString(16),
			Room:      room,
			SenderID:  claims.UserID,
			Content:   "", // optional caption, if you want to support it later
			Files:     attachments,
			Timestamp: ts,
		}
		if _, err := db.MessagesCollection.InsertOne(context.Background(), msg); err != nil {
			log.Println("db error:", err)
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}

		// 5) Broadcast to WebSocket clients
		out := outboundPayload{
			Action:    "chat",
			ID:        msg.MessageID,
			Room:      msg.Room,
			SenderID:  msg.SenderID,
			Content:   msg.Content,
			Files:     msg.Files,
			Timestamp: msg.Timestamp,
		}
		data, _ := json.Marshal(out)
		hub.broadcast <- broadcastMsg{Room: msg.Room, Data: data}

		// 6) Respond to HTTP client
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}
