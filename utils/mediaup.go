package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
)

var bannerDir string = "./static/placepic"

func UploadBanner(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	err := r.ParseMultipartForm(10 << 20) // 10MB limit
	if err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("banner")
	if err != nil {
		http.Error(w, "Banner file missing", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := filepath.Ext(handler.Filename)
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
		http.Error(w, "Unsupported file type", http.StatusBadRequest)
		return
	}

	id := uuid.New().String()
	filename := fmt.Sprintf("%s%s", id, ext)
	path := filepath.Join(bannerDir, filename)

	dst, err := os.Create(path)
	if err != nil {
		http.Error(w, "Unable to save file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	// Optionally generate thumbnail here
	CreateThumb(id, bannerDir, ext, 300, 200)

	resp := map[string]string{
		"bannerUrl": "/banners/" + filename, // adjust if serving from CDN or S3
	}
	RespondWithJSON(w, http.StatusOK, resp)
}
