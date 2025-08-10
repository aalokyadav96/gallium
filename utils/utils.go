package utils

import (
	"crypto/md5"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	rndm "math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"naevis/models"
	"naevis/mq"

	"github.com/disintegration/imaging"
	"github.com/julienschmidt/httprouter"
)

// --- CSRF Token Generation ---

func CSRF(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	csrf := GenerateRandomString(12)
	RespondWithJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"csrf_token": csrf,
	})
}

// --- Random String and ID Generators ---

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ")
var digitRunes = []rune("0123456789")

// GenerateRandomString creates a random alphanumeric string of length n.
func GenerateRandomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rndm.Intn(len(letterRunes))]
	}
	return string(b)
}

// GenerateRandomDigitString creates a random numeric string of length n.
func GenerateRandomDigitString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = digitRunes[rndm.Intn(len(digitRunes))]
	}
	return string(b)
}

// --- Hashing ---

func EncrypIt(strToHash string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(strToHash)))
}

// --- HTTP Response Helpers ---

func SendResponse(w http.ResponseWriter, status int, data any, message string, err error) {
	resp := map[string]any{
		"status":  status,
		"message": message,
		"data":    data,
	}
	if err != nil {
		resp["error"] = err.Error()
	}
	RespondWithJSON(w, status, resp)
}

// func RespondWithJSON(w http.ResponseWriter, status int, data any) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.WriteHeader(status)
// 	if err := json.NewEncoder(w).Encode(data); err != nil {
// 		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
// 	}
// }

func SendJSONResponse(w http.ResponseWriter, status int, response any) {
	RespondWithJSON(w, status, response)
}

// --- Slice Helpers ---

func Contains(slice []string, value string) bool {
	return slices.Contains(slice, value)
}

// --- Image Validation ---

var SupportedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
	"image/bmp":  true,
	"image/tiff": true,
}

func ValidateImageFileType(w http.ResponseWriter, header *multipart.FileHeader) bool {
	mimeType := header.Header.Get("Content-Type")
	if !SupportedImageTypes[mimeType] {
		http.Error(w, "Invalid file type. Supported formats: JPEG, PNG, WebP, GIF, BMP, TIFF.", http.StatusBadRequest)
		return false
	}
	return true
}

// --- Thumbnail Creation ---

func CreateThumb(filename, fileLocation, fileType string, thumbWidth, thumbHeight int) error {
	inputPath := filepath.Join(fileLocation, filename+fileType)
	outputDir := filepath.Join(fileLocation, "thumb")
	outputPath := filepath.Join(outputDir, filename+fileType)

	// Ensure output directory exists
	if err := ensureDir(outputDir); err != nil {
		log.Printf("failed to create thumbnail directory: %v", err)
		return err
	}

	bgColor := color.Transparent

	img, err := imaging.Open(inputPath)
	if err != nil {
		log.Printf("failed to open input image: %v", err)
		return err
	}

	newWidth, newHeight := fitResolution(img.Bounds().Dx(), img.Bounds().Dy(), thumbWidth, thumbHeight)
	resizedImg := imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)

	thumbImg := imaging.New(thumbWidth, thumbHeight, bgColor)
	xPos := (thumbWidth - newWidth) / 2
	yPos := (thumbHeight - newHeight) / 2
	thumbImg = imaging.Paste(thumbImg, resizedImg, image.Pt(xPos, yPos))

	if err := imaging.Save(thumbImg, outputPath); err != nil {
		log.Printf("failed to save thumbnail: %v", err)
		return err
	}

	// Notify via MQ
	mq.Notify("thumbnail-created", models.Index{})

	return nil
}

func fitResolution(origWidth, origHeight, maxWidth, maxHeight int) (int, int) {
	if origWidth <= maxWidth && origHeight <= maxHeight {
		return origWidth, origHeight
	}

	widthRatio := float64(maxWidth) / float64(origWidth)
	heightRatio := float64(maxHeight) / float64(origHeight)
	scaleFactor := math.Min(widthRatio, heightRatio)

	return int(float64(origWidth) * scaleFactor), int(float64(origHeight) * scaleFactor)
}

// --- Directory Helper ---

func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}
