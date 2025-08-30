package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"naevis/db"
	"naevis/filemgr"
	"naevis/globals"
	"naevis/models"
	"naevis/mq"
	"naevis/rdx"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	ffprobeTimeout = 30 * time.Second
	posterTimeout  = 45 * time.Second
)

func init() {
	// seed math/rand once
	rand.Seed(time.Now().UnixNano())
}

// ------------------- Command Runner -------------------

type Runner interface {
	Run(timeout time.Duration, name string, args ...string) (stdout string, stderr string, err error)
}

type realRunner struct{}

func (realRunner) Run(timeout time.Duration, name string, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return out.String(), errb.String(), fmt.Errorf("%s timed out after %s", name, timeout)
	}
	return out.String(), errb.String(), err
}

// Global runner, can be mocked in tests
var cmdRunner Runner = realRunner{}

func AddMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")
	if entityID == "" {
		http.Error(w, "Entity ID is required", http.StatusBadRequest)
		return
	}

	// parse with a sane limit
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		http.Error(w, "Unable to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}
	if r.MultipartForm == nil {
		http.Error(w, "Invalid multipart form", http.StatusBadRequest)
		return
	}

	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok || requestingUserID == "" {
		http.Error(w, "Invalid or missing user ID", http.StatusUnauthorized)
		return
	}

	files := r.MultipartForm.File["media"]
	if len(files) == 0 {
		http.Error(w, "No media file provided", http.StatusBadRequest)
		return
	}

	header := files[0]
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = utils.GuessMimeType(header.Filename)
	}

	media := models.Media{
		EntityID:   entityID,
		EntityType: entityType,
		Caption:    r.FormValue("caption"),
		CreatorID:  requestingUserID,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		MimeType:   mimeType,
	}

	if entityType == "event" {
		media.MediaID = "e" + utils.GenerateRandomString(16)
	} else {
		media.MediaID = "p" + utils.GenerateRandomString(16)
	}

	var picType filemgr.PictureType
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		media.Type = "image"
		picType = filemgr.PicPhoto
	case strings.HasPrefix(mimeType, "video/"):
		media.Type = "video"
		picType = filemgr.PicVideo
	default:
		http.Error(w, "Unsupported media type: "+mimeType, http.StatusUnsupportedMediaType)
		return
	}

	filename, err := filemgr.SaveFormFile(r.MultipartForm, "media", filemgr.EntityMedia, picType, true)
	if err != nil {
		http.Error(w, "Media upload failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	media.URL = filename
	media.FileSize = header.Size

	fullPath := filepath.Join(filemgr.ResolvePath(filemgr.EntityMedia, picType), filename)

	if media.Type == "video" {
		// Video duration: attempt ffprobe, but continue with defaults on failure
		if dur, err := getVideoDuration(fullPath); err == nil && dur > 0 {
			media.Duration = dur
		} else {
			// leave duration 0 or set default behaviour downstream
			media.Duration = 0
		}

		// Create poster file name with extension
		posterDir := filemgr.ResolvePath(filemgr.EntityMedia, filemgr.PicThumb)
		posterFilename := media.MediaID + ".jpg"
		posterPath := filepath.Join(posterDir, posterFilename)

		if err := CreatePoster(fullPath, posterPath); err != nil {
			// log but do not fail the upload
			fmt.Printf("Poster creation failed: %v\n", err)
		} else {
			// store poster path or filename if your model tracks it
			// if models.Media has a field for Poster or Thumb you can set it:
			// media.Poster = posterFilename
			fmt.Printf("Poster created at %s\n", posterPath)
		}
	}

	_, err = db.MediaCollection.InsertOne(ctx, media)
	if err != nil {
		http.Error(w, "Error saving media to database: "+err.Error(), http.StatusInternalServerError)
		return
	}

	userdata.SetUserData("media", media.MediaID, requestingUserID, entityType, entityID)

	go mq.Emit(ctx, "media-created", models.Index{
		EntityType: "media",
		EntityId:   media.MediaID,
		Method:     "POST",
		ItemType:   entityType,
		ItemId:     entityID,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(media)
}

func getVideoDuration(path string) (float64, error) {
	// ffprobe returns JSON with format.duration as string
	args := []string{"-v", "error", "-show_entries", "format=duration", "-of", "json", path}
	stdout, stderr, err := cmdRunner.Run(ffprobeTimeout, "ffprobe", args...)
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w (stderr=%s)", err, stderr)
	}

	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return 0, fmt.Errorf("ffprobe returned empty output")
	}

	var result struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		return 0, fmt.Errorf("json unmarshal failed: %w (stdout=%s)", err, stdout)
	}

	if result.Format.Duration == "" {
		return 0, fmt.Errorf("duration not found in ffprobe output (stdout=%s)", stdout)
	}

	dur, err := strconv.ParseFloat(strings.TrimSpace(result.Format.Duration), 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration failed: %w (duration=%s)", err, result.Format.Duration)
	}
	return dur, nil
}

func EditMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")
	mediaID := ps.ByName("id")
	cacheKey := fmt.Sprintf("media:%s:%s", entityID, mediaID)

	// Check the cache first
	cachedMedia, err := rdx.RdxGet(cacheKey)
	if err == nil && cachedMedia != "" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cachedMedia))
		return
	}

	// Fetch the media from MongoDB
	var media models.Media
	err = db.MediaCollection.FindOne(ctx, bson.M{
		"entityid":   entityID,
		"entitytype": entityType,
		"mediaid":    mediaID,
	}).Decode(&media)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Media not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Cache the result
	mediaJSON, _ := json.Marshal(media)
	rdx.RdxSet(cacheKey, string(mediaJSON))

	m := models.Index{EntityType: "media", EntityId: mediaID, Method: "{PUT}", ItemType: entityType, ItemId: entityID}
	go mq.Emit(ctx, "media-edited", m)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(media)
}

func GetMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")
	mediaID := ps.ByName("id")

	var media models.Media
	err := db.MediaCollection.FindOne(r.Context(), bson.M{
		"entityid":   entityID,
		"entitytype": entityType,
		"mediaid":    mediaID,
	}).Decode(&media)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Media not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(media)
}

func DeleteMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	entityType := ps.ByName("entitytype")
	entityID := ps.ByName("entityid")
	mediaID := ps.ByName("id")

	// Retrieve requesting user
	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok || requestingUserID == "" {
		http.Error(w, "Invalid user", http.StatusUnauthorized)
		return
	}

	// Fetch the media to check owner & paths
	var media models.Media
	err := db.MediaCollection.FindOne(ctx, bson.M{
		"entityid":   entityID,
		"entitytype": entityType,
		"mediaid":    mediaID,
	}).Decode(&media)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Media not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Basic authorization: only creator can delete (or you can add admin role check)
	if media.CreatorID != requestingUserID {
		// if you have admin role logic replace with proper role check
		http.Error(w, "Not authorized to delete this media", http.StatusForbidden)
		return
	}

	// Delete DB document
	_, err = db.MediaCollection.DeleteOne(ctx, bson.M{
		"entityid":   entityID,
		"entitytype": entityType,
		"mediaid":    mediaID,
	})
	if err != nil {
		http.Error(w, "Failed to delete media", http.StatusInternalServerError)
		return
	}

	// Attempt to remove files (media file and poster/thumb). Best-effort: log errors but do not fail response.
	// media.URL is expected to be a filename returned by filemgr.SaveFormFile
	if media.URL != "" {
		mediaPath := filepath.Join(filemgr.ResolvePath(filemgr.EntityMedia, filemgr.PicPhoto), media.URL)
		if err := os.Remove(mediaPath); err != nil && !os.IsNotExist(err) {
			fmt.Printf("Warning: failed to remove media file %s: %v\n", mediaPath, err)
		}
	}

	// remove poster (mediaID + .jpg) from thumbnail path (best-effort)
	thumbPath := filepath.Join(filemgr.ResolvePath(filemgr.EntityMedia, filemgr.PicThumb), media.MediaID+".jpg")
	if err := os.Remove(thumbPath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: failed to remove thumb %s: %v\n", thumbPath, err)
	}

	userdata.DelUserData("media", mediaID, requestingUserID)

	m := models.Index{EntityType: "media", EntityId: mediaID, Method: "DELETE", ItemType: entityType, ItemId: entityID}
	go mq.Emit(ctx, "media-deleted", m)

	// Respond with success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := map[string]any{
		"success": true,
		"message": "Media deleted successfully",
	}
	json.NewEncoder(w).Encode(response)
}

// Media
func GetMedias(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	filter := bson.M{"entityid": ps.ByName("entityid"), "entitytype": ps.ByName("entitytype")}
	medias, err := utils.FindAndDecode[models.Media](ctx, db.MediaCollection, filter)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve media")
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, medias)
}

func ensureDir(dir string) error {
	return os.MkdirAll(dir, os.ModePerm)
}

func CreatePoster(videoPath, posterPath string) error {
	// ensure directory exists
	if err := ensureDir(filepath.Dir(posterPath)); err != nil {
		return err
	}

	duration, err := getVideoDuration(videoPath)
	if err != nil || duration <= 0 {
		// fallback: pick a short offset
		duration = 3.0
	}

	// choose a timestamp safely within duration
	t := duration * 0.25
	if t < 1.0 {
		t = 1.0
	}
	if t > duration-0.5 {
		t = math.Max(0.0, duration-0.5)
	}
	// small jitter but bounded
	jitter := (rand.Float64() * 0.2)
	t = t + jitter
	if t < 0 {
		t = 0
	}

	timestamp := formatTimestamp(t)

	// ensure posterPath has extension
	ext := filepath.Ext(posterPath)
	if ext == "" {
		posterPath = posterPath + ".jpg"
	}

	args := []string{
		"-y",
		"-ss", timestamp,
		"-i", videoPath,
		"-vframes", "1",
		"-q:v", "2",
		posterPath,
	}

	stdout, stderr, err := cmdRunner.Run(posterTimeout, "ffmpeg", args...)
	if err != nil {
		return fmt.Errorf("poster creation failed for %s: %w (stdout=%s, stderr=%s)", videoPath, err, stdout, stderr)
	}
	return nil
}

func formatTimestamp(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	// produce "HH:MM:SS.mmm" which ffmpeg accepts
	totalMs := int(seconds * 1000)
	h := totalMs / 3600000
	m := (totalMs % 3600000) / 60000
	s := (totalMs % 60000) / 1000
	ms := totalMs % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}

// package media

// import (
// 	"bytes"
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"math"
// 	"math/rand/v2"
// 	"naevis/db"
// 	"naevis/filemgr"
// 	"naevis/globals"
// 	"naevis/models"
// 	"naevis/mq"
// 	"naevis/rdx"
// 	"naevis/userdata"
// 	"naevis/utils"
// 	"net/http"
// 	"os"
// 	"os/exec"
// 	"path/filepath"
// 	"strconv"
// 	"strings"
// 	"time"

// 	"github.com/julienschmidt/httprouter"
// 	"go.mongodb.org/mongo-driver/bson"
// 	"go.mongodb.org/mongo-driver/mongo"
// )

// const (
// 	ffprobeTimeout = 30 * time.Second
// 	posterTimeout  = 45 * time.Second
// )

// // ------------------- Command Runner -------------------

// type Runner interface {
// 	Run(timeout time.Duration, name string, args ...string) (stdout string, stderr string, err error)
// }

// type realRunner struct{}

// func (realRunner) Run(timeout time.Duration, name string, args ...string) (string, string, error) {
// 	ctx, cancel := context.WithTimeout(context.Background(), timeout)
// 	defer cancel()

// 	cmd := exec.CommandContext(ctx, name, args...)
// 	var out, errb bytes.Buffer
// 	cmd.Stdout = &out
// 	cmd.Stderr = &errb

// 	err := cmd.Run()
// 	if ctx.Err() == context.DeadlineExceeded {
// 		return out.String(), errb.String(), fmt.Errorf("%s timed out after %s", name, timeout)
// 	}
// 	return out.String(), errb.String(), err
// }

// // Global runner, can be mocked in tests
// var cmdRunner Runner = realRunner{}

// func AddMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	ctx := r.Context()
// 	entityType := ps.ByName("entitytype")
// 	entityID := ps.ByName("entityid")
// 	if entityID == "" {
// 		http.Error(w, "Entity ID is required", http.StatusBadRequest)
// 		return
// 	}

// 	err := r.ParseMultipartForm(50 << 20)
// 	if err != nil {
// 		http.Error(w, "Unable to parse form: "+err.Error(), http.StatusBadRequest)
// 		return
// 	}

// 	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
// 	if !ok || requestingUserID == "" {
// 		http.Error(w, "Invalid or missing user ID", http.StatusUnauthorized)
// 		return
// 	}

// 	media := models.Media{
// 		EntityID:   entityID,
// 		EntityType: entityType,
// 		Caption:    r.FormValue("caption"),
// 		CreatorID:  requestingUserID,
// 		CreatedAt:  time.Now(),
// 		UpdatedAt:  time.Now(),
// 	}

// 	if entityType == "event" {
// 		media.MediaID = "e" + utils.GenerateRandomString(16)
// 	} else {
// 		media.MediaID = "p" + utils.GenerateRandomString(16)
// 	}

// 	files := r.MultipartForm.File["media"]
// 	if len(files) == 0 {
// 		http.Error(w, "No media file provided", http.StatusBadRequest)
// 		return
// 	}

// 	header := files[0]
// 	mimeType := header.Header.Get("Content-Type")
// 	if mimeType == "" || mimeType == "application/octet-stream" {
// 		mimeType = utils.GuessMimeType(header.Filename)
// 	}
// 	media.MimeType = mimeType

// 	var picType filemgr.PictureType
// 	switch {
// 	case strings.HasPrefix(mimeType, "image/"):
// 		media.Type = "image"
// 		picType = filemgr.PicPhoto
// 	case strings.HasPrefix(mimeType, "video/"):
// 		media.Type = "video"
// 		picType = filemgr.PicVideo
// 	default:
// 		http.Error(w, "Unsupported media type: "+mimeType, http.StatusUnsupportedMediaType)
// 		return
// 	}

// 	filename, err := filemgr.SaveFormFile(r.MultipartForm, "media", filemgr.EntityMedia, picType, true)
// 	if err != nil {
// 		http.Error(w, "Media upload failed: "+err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// 	media.URL = filename

// 	fullPath := filepath.Join(filemgr.ResolvePath(filemgr.EntityMedia, picType), filename)

// 	if media.Type == "video" {
// 		media.FileSize = header.Size
// 		media.Duration, _ = getVideoDuration(fullPath)
// 		posterPath := filepath.Join(filemgr.ResolvePath(filemgr.EntityMedia, filemgr.PicThumb), media.MediaID)
// 		if err := CreatePoster(fullPath, posterPath); err != nil {
// 			fmt.Printf("Poster creation failed: %v\n", err)
// 		} else {
// 			fmt.Printf("Poster created at %s\n", posterPath)
// 		}
// 	}

// 	_, err = db.MediaCollection.InsertOne(r.Context(), media)
// 	if err != nil {
// 		http.Error(w, "Error saving media to database: "+err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	userdata.SetUserData("media", media.MediaID, requestingUserID, entityType, entityID)

// 	go mq.Emit(ctx, "media-created", models.Index{
// 		EntityType: "media",
// 		EntityId:   media.MediaID,
// 		Method:     "POST",
// 		ItemType:   entityType,
// 		ItemId:     entityID,
// 	})

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(media)
// }

// func getVideoDuration(path string) (float64, error) {
// 	args := []string{"-v", "error", "-show_entries", "format=duration", "-of", "json", path}
// 	stdout, stderr, err := cmdRunner.Run(ffprobeTimeout, "ffprobe", args...)
// 	if err != nil {
// 		return 0, fmt.Errorf("ffprobe failed: %w (stderr=%s)", err, stderr)
// 	}

// 	var result struct {
// 		Format struct {
// 			Duration string `json:"duration"`
// 		} `json:"format"`
// 	}
// 	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
// 		return 0, fmt.Errorf("json unmarshal failed: %w (stdout=%s)", err, stdout)
// 	}

// 	dur, err := strconv.ParseFloat(result.Format.Duration, 64)
// 	if err != nil {
// 		return 0, fmt.Errorf("parse duration failed: %w", err)
// 	}
// 	return dur, nil
// }

// func EditMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	ctx := r.Context()
// 	entityType := ps.ByName("entitytype")
// 	entityID := ps.ByName("entityid")
// 	mediaID := ps.ByName("id")
// 	cacheKey := fmt.Sprintf("media:%s:%s", entityID, mediaID)

// 	// Check the cache first
// 	cachedMedia, err := rdx.RdxGet(cacheKey)
// 	if err == nil && cachedMedia != "" {
// 		w.Header().Set("Content-Type", "application/json")
// 		w.Write([]byte(cachedMedia))
// 		return
// 	}

// 	// Fetch the media from MongoDB
// 	var media models.Media
// 	err = db.MediaCollection.FindOne(r.Context(), bson.M{
// 		"entityid":   entityID,
// 		"entitytype": entityType,
// 		"mediaid":    mediaID,
// 	}).Decode(&media)
// 	if err != nil {
// 		if err == mongo.ErrNoDocuments {
// 			http.Error(w, "Media not found", http.StatusNotFound)
// 			return
// 		}
// 		http.Error(w, "Database error", http.StatusInternalServerError)
// 		return
// 	}

// 	// Cache the result
// 	mediaJSON, _ := json.Marshal(media)
// 	rdx.RdxSet(cacheKey, string(mediaJSON))

// 	m := models.Index{EntityType: "media", EntityId: mediaID, Method: "{PUT}", ItemType: entityType, ItemId: entityID}
// 	go mq.Emit(ctx, "media-edited", m)

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(media)
// }

// func GetMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	entityType := ps.ByName("entitytype")
// 	entityID := ps.ByName("entityid")
// 	mediaID := ps.ByName("id")

// 	var media models.Media
// 	err := db.MediaCollection.FindOne(r.Context(), bson.M{
// 		"entityid":   entityID,
// 		"entitytype": entityType,
// 		"mediaid":    mediaID,
// 	}).Decode(&media)
// 	if err != nil {
// 		if err == mongo.ErrNoDocuments {
// 			http.Error(w, "Media not found", http.StatusNotFound)
// 			return
// 		}
// 		http.Error(w, "Database error", http.StatusInternalServerError)
// 		return
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(media)
// }

// func DeleteMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	ctx := r.Context()
// 	entityType := ps.ByName("entitytype")
// 	entityID := ps.ByName("entityid")
// 	mediaID := ps.ByName("id")

// 	// Retrieve the ID of the requesting user from the context
// 	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
// 	if !ok {
// 		http.Error(w, "Invalid user", http.StatusBadRequest)
// 		return
// 	}

// 	_, err := db.MediaCollection.DeleteOne(r.Context(), bson.M{
// 		"entityid":   entityID,
// 		"entitytype": entityType,
// 		"mediaid":    mediaID,
// 	})
// 	if err != nil {
// 		http.Error(w, "Failed to delete media", http.StatusInternalServerError)
// 		return
// 	}

// 	userdata.DelUserData("media", mediaID, requestingUserID)

// 	m := models.Index{EntityType: "media", EntityId: mediaID, Method: "DELETE", ItemType: entityType, ItemId: entityID}
// 	go mq.Emit(ctx, "media-deleted", m)

// 	// Respond with success
// 	w.WriteHeader(http.StatusOK)
// 	response := map[string]any{
// 		"status":  http.StatusNoContent,
// 		"message": "Media deleted successfully",
// 	}
// 	json.NewEncoder(w).Encode(response)
// }

// // Media
// func GetMedias(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
// 	defer cancel()

// 	filter := bson.M{"entityid": ps.ByName("entityid"), "entitytype": ps.ByName("entitytype")}
// 	medias, err := utils.FindAndDecode[models.Media](ctx, db.MediaCollection, filter)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve media")
// 		return
// 	}
// 	utils.RespondWithJSON(w, http.StatusOK, medias)
// }

// func ensureDir(dir string) error {
// 	return os.MkdirAll(dir, os.ModePerm)
// }

// func CreatePoster(videoPath, posterPath string) error {
// 	if err := ensureDir(filepath.Dir(posterPath)); err != nil {
// 		return err
// 	}

// 	duration, err := getVideoDuration(videoPath)
// 	if err != nil || duration <= 0 {
// 		duration = 3.0
// 	}
// 	t := duration * 0.25
// 	if t < 1.0 {
// 		t = 1.0
// 	}
// 	if t > duration-0.5 {
// 		t = math.Max(0.0, duration-0.5)
// 	}
// 	t += math.Mod(rand.Float64()*0.2, 0.2)
// 	timestamp := formatTimestamp(t)

// 	args := []string{
// 		"-y",
// 		"-ss", timestamp,
// 		"-i", videoPath,
// 		"-vframes", "1",
// 		"-q:v", "2",
// 		posterPath,
// 	}

// 	stdout, stderr, err := cmdRunner.Run(posterTimeout, "ffmpeg", args...)
// 	if err != nil {
// 		return fmt.Errorf("poster creation failed for %s: %w (stdout=%s, stderr=%s)", videoPath, err, stdout, stderr)
// 	}
// 	return nil
// }

// func formatTimestamp(seconds float64) string {
// 	if seconds < 0 {
// 		seconds = 0
// 	}
// 	totalMs := int(seconds * 1000)
// 	h := totalMs / 3600000
// 	m := (totalMs % 3600000) / 60000
// 	s := (totalMs % 60000) / 1000
// 	ms := totalMs % 1000
// 	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
// }
