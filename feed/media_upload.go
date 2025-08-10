package feed

import (
	"fmt"
	"mime/multipart"
	"naevis/filemgr"
	"naevis/models"
	"naevis/mq"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type MediaType string

const (
	Video MediaType = "video"
	Audio MediaType = "audio"
)

func processMediaUpload(r *http.Request, formKey string, mediaType MediaType) ([]int, []string, []string, error) {
	file, err := getUploadedFile(r, formKey)
	if err != nil || file == nil {
		return nil, nil, nil, fmt.Errorf("no file uploaded: %w", err)
	}

	src, err := file.Open()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("cannot open uploaded file: %w", err)
	}
	defer src.Close()

	var picType filemgr.PictureType
	switch mediaType {
	case Video:
		picType = filemgr.PicVideo
	case Audio:
		picType = filemgr.PicAudio
	default:
		return nil, nil, nil, fmt.Errorf("unsupported media type: %s", mediaType)
	}

	// Save to secure location via filemgr (virus scan, EXIF strip, etc.)
	savedName, err := filemgr.SaveFileForEntity(src, file, filemgr.EntityFeed, picType)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("file save failed: %w", err)
	}

	uploadDir := filemgr.ResolvePath(filemgr.EntityFeed, picType)
	savedPath := filepath.Join(uploadDir, savedName)
	uniqueID := strings.TrimSuffix(savedName, filepath.Ext(savedName))

	var (
		resolutions []int
		outputPath  string
	)

	switch mediaType {
	case Video:
		width, height, err := getVideoDimensions(savedPath)
		if err != nil {
			os.Remove(savedPath)
			return nil, nil, nil, fmt.Errorf("failed to get video dimensions: %w", err)
		}

		posterDir := filepath.Join(filemgr.ResolvePath(filemgr.EntityFeed, filemgr.PicPoster), uniqueID)
		resolutions, outputPath = processVideoResolutions(savedPath, uploadDir, uniqueID, width, height)

		// if err := createDefaultPoster(savedPath, uploadDir, uniqueID); err != nil {
		if err := CreatePoster(savedPath, posterDir); err != nil {
			return nil, nil, nil, fmt.Errorf("poster creation failed: %w", err)
		}

		go createSubtitleFile(uniqueID)
		mq.Notify("postpics-uploaded", models.Index{})

	case Audio:
		resolutions, outputPath = processAudioResolutions(savedPath, uploadDir, uniqueID)
		go createSubtitleFile(uniqueID)
		mq.Notify("postaudio-uploaded", models.Index{})
	}

	return resolutions, []string{outputPath}, []string{uniqueID}, nil
}

func getUploadedFile(r *http.Request, formKey string) (*multipart.FileHeader, error) {
	if r.MultipartForm == nil {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return nil, fmt.Errorf("failed to parse form: %w", err)
		}
	}
	files := r.MultipartForm.File[formKey]
	if len(files) == 0 {
		return nil, nil
	}
	return files[0], nil
}

func saveUploadedVideoFile(r *http.Request, formKey string) ([]int, []string, []string, error) {
	return processMediaUpload(r, formKey, Video)
}

func saveUploadedAudioFile(r *http.Request, formKey string) ([]int, []string, []string, error) {
	return processMediaUpload(r, formKey, Audio)
}

// processVideoResolutions creates versions of the video at multiple resolutions.
func processVideoResolutions(originalFilePath, uploadDir, uniqueID string, origWidth, origHeight int) ([]int, string) {
	resolutions := []struct {
		Label  string
		Width  int
		Height int
	}{
		{"4320p", 7680, 4320}, {"2160p", 3840, 2160}, {"1440p", 2560, 1440},
		{"1080p", 1920, 1080}, {"720p", 1280, 720}, {"480p", 854, 480},
		{"360p", 640, 360}, {"240p", 426, 240}, {"144p", 256, 144},
	}

	var availableResolutions []int
	var highestResolutionPath string

	for _, res := range resolutions {
		newWidth, newHeight := fitResolution(origWidth, origHeight, res.Width, res.Height)
		if newWidth > origWidth || newHeight > origHeight {
			continue
		}

		outputFilePath := generateFilePath(uploadDir, uniqueID+"-"+res.Label, "mp4")

		if err := processVideoResolution(originalFilePath, outputFilePath, fmt.Sprintf("%dx%d", newWidth, newHeight)); err != nil {
			fmt.Printf("Skipping %s due to error: %v\n", res.Label, err)
			continue
		}

		highestResolutionPath = "/" + filepath.ToSlash(outputFilePath)
		availableResolutions = append(availableResolutions, newHeight)
	}

	return availableResolutions, highestResolutionPath
}

func generateFilePath(baseDir, uniqueID, extension string) string {
	return filepath.Join(baseDir, fmt.Sprintf("%s.%s", uniqueID, extension))
}

func saveUploadedFiles(r *http.Request, formKey, fileType string) ([]string, error) {
	files := r.MultipartForm.File[formKey]
	if len(files) == 0 {
		return nil, fmt.Errorf("no %s files uploaded", fileType)
	}

	var ids []string
	entity := filemgr.EntityFeed
	picType := filemgr.PictureType(fileType)

	for _, file := range files {
		origName, err := processSingleImageUpload(file, entity, picType)
		if err != nil {
			return nil, fmt.Errorf("failed to process %s: %w", fileType, err)
		}
		ids = append(ids, origName)
	}

	mq.Notify("postpics-uploaded", models.Index{})
	mq.Notify("thumbnail-created", models.Index{})

	return ids, nil
}

func processSingleImageUpload(file *multipart.FileHeader, entity filemgr.EntityType, picType filemgr.PictureType) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("cannot open image: %w", err)
	}
	defer src.Close()

	origName, err := filemgr.SaveFileForEntity(src, file, entity, picType)
	if err != nil {
		return "", fmt.Errorf("saving image failed: %w", err)
	}

	// id := strings.TrimSuffix(filepath.Base(origName), filepath.Ext(origName))
	// path := "/" + filepath.ToSlash(origName)

	return origName, nil
}

// package feed

// import (
// 	"fmt"
// 	"mime/multipart"
// 	"naevis/filemgr"
// 	"naevis/mq"
// 	"net/http"
// 	"os"
// 	"path/filepath"
// 	"strings"
// )

// type MediaType string

// const (
// 	Video MediaType = "video"
// 	Audio MediaType = "audio"
// )

// func processUpload(r *http.Request, formKey, postID, userID string, mediaType MediaType) ([]int, []string, []string, error) {
// 	file, err := getUploadedFile(r, formKey)
// 	if err != nil || file == nil {
// 		return nil, nil, nil, fmt.Errorf("no file uploaded: %w", err)
// 	}

// 	src, err := file.Open()
// 	if err != nil {
// 		return nil, nil, nil, fmt.Errorf("cannot open uploaded file: %w", err)
// 	}
// 	defer src.Close()

// 	var picType filemgr.PictureType
// 	switch mediaType {
// 	case Video:
// 		picType = filemgr.PicVideo
// 	case Audio:
// 		picType = filemgr.PicAudio
// 	default:
// 		return nil, nil, nil, fmt.Errorf("unsupported media type: %s", mediaType)
// 	}

// 	// Save to validated location via filemgr (with virus scan, EXIF strip, mime check, etc.)
// 	savedName, err := filemgr.SaveFileForEntity(src, file, filemgr.EntityFeed, picType)
// 	if err != nil {
// 		return nil, nil, nil, fmt.Errorf("file save failed: %w", err)
// 	}

// 	uploadDir := filemgr.ResolvePath(filemgr.EntityFeed, picType)
// 	savedPath := filepath.Join(uploadDir, savedName)
// 	uniqueID := strings.TrimSuffix(savedName, filepath.Ext(savedName))

// 	// Trigger your upload logic (e.g. DB entry, audit log)
// 	UploadFile(src, savedPath, userID, postID)

// 	var (
// 		resolutions []int
// 		outputPath  string
// 	)

// 	switch mediaType {
// 	case Video:
// 		width, height, err := getVideoDimensions(savedPath)
// 		if err != nil {
// 			os.Remove(savedPath)
// 			return nil, nil, nil, fmt.Errorf("failed to get video dimensions: %w", err)
// 		}

// 		resolutions, outputPath = processVideoResolutions(savedPath, filepath.Dir(savedPath), uniqueID, width, height)

// 		if err := createDefaultPoster(savedPath, filepath.Dir(savedPath), uniqueID); err != nil {
// 			return nil, nil, nil, fmt.Errorf("poster creation failed: %w", err)
// 		}

// 		go createSubtitleFile(uniqueID)
// 		mq.Notify("postpics-uploaded", models.Index{})

// 	case Audio:
// 		resolutions, outputPath = processAudioResolutions(savedPath, filepath.Dir(savedPath), uniqueID)
// 		go createSubtitleFile(uniqueID)
// 		mq.Notify("postaudio-uploaded", models.Index{})
// 	}

// 	return resolutions, []string{outputPath}, []string{uniqueID}, nil
// }

// func getUploadedFile(r *http.Request, formKey string) (*multipart.FileHeader, error) {
// 	if r.MultipartForm == nil {
// 		if err := r.ParseMultipartForm(32 << 20); err != nil {
// 			return nil, fmt.Errorf("failed to parse form: %w", err)
// 		}
// 	}
// 	files := r.MultipartForm.File[formKey]
// 	if len(files) == 0 {
// 		return nil, nil
// 	}
// 	return files[0], nil
// }

// func saveUploadedVideoFile(r *http.Request, formKey, postID, userID string) ([]int, []string, []string, error) {
// 	return processUpload(r, formKey, postID, userID, Video)
// }

// func saveUploadedAudioFile(r *http.Request, formKey, postID, userID string) ([]int, []string, []string, error) {
// 	return processUpload(r, formKey, postID, userID, Audio)
// }

// // processVideoResolutions creates versions of the video at multiple resolutions.
// // It returns a slice of available resolution heights and the path of the highest resolution file.
// func processVideoResolutions(originalFilePath, uploadDir, uniqueID string, origWidth, origHeight int) ([]int, string) {
// 	// Define available resolutions.
// 	resolutions := []struct {
// 		Label  string
// 		Width  int
// 		Height int
// 	}{
// 		{"4320p", 7680, 4320}, {"2160p", 3840, 2160}, {"1440p", 2560, 1440},
// 		{"1080p", 1920, 1080}, {"720p", 1280, 720}, {"480p", 854, 480},
// 		{"360p", 640, 360}, {"240p", 426, 240}, {"144p", 256, 144},
// 	}

// 	var availableResolutions []int
// 	var highestResolutionPath string

// 	for _, res := range resolutions {
// 		newWidth, newHeight := fitResolution(origWidth, origHeight, res.Width, res.Height)
// 		// Only downscale (avoid upscaling)
// 		if newWidth > origWidth || newHeight > origHeight {
// 			continue
// 		}

// 		outputFilePath := generateFilePath(uploadDir, uniqueID+"-"+res.Label, "mp4")
// 		outputPosterPath := generateFilePath(uploadDir, uniqueID, "jpg")

// 		// processVideoResolution is assumed to perform the resolution conversion.
// 		if err := processVideoResolution(originalFilePath, outputFilePath, fmt.Sprintf("%dx%d", newWidth, newHeight)); err != nil {
// 			fmt.Printf("Skipping %s due to error: %v\n", res.Label, err)
// 			continue
// 		}

// 		// Create a poster image for this resolution.
// 		if err := CreatePoster(originalFilePath, outputPosterPath); err != nil {
// 			fmt.Printf("Skipping %s poster due to error: %v\n", res.Label, err)
// 			continue
// 		}

// 		highestResolutionPath = "/postpic/" + uniqueID + "/" + filepath.Base(outputFilePath)
// 		availableResolutions = append(availableResolutions, newHeight)
// 	}

// 	return availableResolutions, highestResolutionPath
// }

// func ensureDirExists(dir string) error {
// 	return os.MkdirAll(dir, 0755)
// }

// func generateFilePath(baseDir, uniqueID, extension string) string {
// 	return filepath.Join(baseDir, fmt.Sprintf("%s.%s", uniqueID, extension))
// }

// func saveUploadedFiles(r *http.Request, formKey, fileType string) ([]string, []string, error) {
// 	files := r.MultipartForm.File[formKey]
// 	if len(files) == 0 {
// 		return nil, nil, fmt.Errorf("no %s files uploaded", fileType)
// 	}

// 	var paths []string
// 	var ids []string
// 	entity := filemgr.EntityFeed
// 	picType := filemgr.PictureType(fileType)

// 	for _, file := range files {
// 		path, id, err := processSingleImageUpload(file, entity, picType)
// 		if err != nil {
// 			return nil, nil, fmt.Errorf("failed to process %s: %w", fileType, err)
// 		}
// 		paths = append(paths, path)
// 		ids = append(ids, id)
// 	}

// 	mq.Notify("postpics-uploaded", models.Index{})
// 	mq.Notify("thumbnail-created", models.Index{})

// 	return paths, ids, nil
// }
// func processSingleImageUpload(file *multipart.FileHeader, entity filemgr.EntityType, picType filemgr.PictureType) (string, string, error) {
// 	src, err := file.Open()
// 	if err != nil {
// 		return "", "", fmt.Errorf("cannot open image: %w", err)
// 	}
// 	defer src.Close()

// 	// use SaveFileForEntity for original image
// 	origName, err := filemgr.SaveFileForEntity(src, file, entity, picType)
// 	if err != nil {
// 		return "", "", fmt.Errorf("saving image failed: %w", err)
// 	}

// 	// extract ID from filename
// 	id := strings.TrimSuffix(filepath.Base(origName), filepath.Ext(origName))
// 	path := "/" + filepath.ToSlash(origName)

// 	return path, id, nil
// }

// // Directory to store uploaded images/videos
// // const feedVideoUploadDir = "./static/postpic/"

// // const feedAudioUploadDir = "./static/postpic/"

// // -------------------------
// // Generic helper functions
// // -------------------------

// // // processVideoResolutions creates versions of the video at multiple resolutions.
// // // It returns a slice of available resolution heights and the path of the highest resolution file.
// // func processVideoResolutions(originalFilePath, uploadDir, uniqueID string, origWidth, origHeight int) ([]int, string) {
// // 	// Define available resolutions.
// // 	resolutions := []struct {
// // 		Label  string
// // 		Width  int
// // 		Height int
// // 	}{
// // 		{"4320p", 7680, 4320}, {"2160p", 3840, 2160}, {"1440p", 2560, 1440},
// // 		{"1080p", 1920, 1080}, {"720p", 1280, 720}, {"480p", 854, 480},
// // 		{"360p", 640, 360}, {"240p", 426, 240}, {"144p", 256, 144},
// // 	}

// // 	var availableResolutions []int
// // 	var highestResolutionPath string

// // 	for _, res := range resolutions {
// // 		newWidth, newHeight := fitResolution(origWidth, origHeight, res.Width, res.Height)
// // 		// Only downscale (avoid upscaling)
// // 		if newWidth > origWidth || newHeight > origHeight {
// // 			continue
// // 		}

// // 		outputFilePath := generateFilePath(uploadDir, uniqueID+"-"+res.Label, "mp4")
// // 		outputPosterPath := generateFilePath(uploadDir, uniqueID, "jpg")

// // 		// processVideoResolution is assumed to perform the resolution conversion.
// // 		if err := processVideoResolution(originalFilePath, outputFilePath, fmt.Sprintf("%dx%d", newWidth, newHeight)); err != nil {
// // 			fmt.Printf("Skipping %s due to error: %v\n", res.Label, err)
// // 			continue
// // 		}

// // 		// Create a poster image for this resolution.
// // 		if err := CreatePoster(originalFilePath, outputPosterPath); err != nil {
// // 			fmt.Printf("Skipping %s poster due to error: %v\n", res.Label, err)
// // 			continue
// // 		}

// // 		highestResolutionPath = "/postpic/" + uniqueID + "/" + filepath.Base(outputFilePath)
// // 		availableResolutions = append(availableResolutions, newHeight)
// // 	}

// // 	return availableResolutions, highestResolutionPath
// // }

// // func ensureDirExists(dir string) error {
// // 	return os.MkdirAll(dir, 0755)
// // }

// // func generateFilePath(baseDir, uniqueID, extension string) string {
// // 	return filepath.Join(baseDir, fmt.Sprintf("%s.%s", uniqueID, extension))
// // }

// // func saveUploadedFiles(r *http.Request, formKey, fileType string) ([]string, []string, error) {
// // 	files := r.MultipartForm.File[formKey]
// // 	if len(files) == 0 {
// // 		return nil, nil, fmt.Errorf("no %s files uploaded", fileType)
// // 	}

// // 	var paths []string
// // 	var ids []string
// // 	entity := filemgr.EntityFeed
// // 	picType := filemgr.PictureType(fileType)

// // 	for _, file := range files {
// // 		path, id, err := processSingleImageUpload(file, entity, picType)
// // 		if err != nil {
// // 			return nil, nil, fmt.Errorf("failed to process %s: %w", fileType, err)
// // 		}
// // 		paths = append(paths, path)
// // 		ids = append(ids, id)
// // 	}

// // 	mq.Notify("postpics-uploaded", models.Index{})
// // 	mq.Notify("thumbnail-created", models.Index{})

// // 	return paths, ids, nil
// // }
// // func processSingleImageUpload(file *multipart.FileHeader, entity filemgr.EntityType, picType filemgr.PictureType) (string, string, error) {
// // 	src, err := file.Open()
// // 	if err != nil {
// // 		return "", "", fmt.Errorf("cannot open image: %w", err)
// // 	}
// // 	defer src.Close()

// // 	// use SaveFileForEntity for original image
// // 	origName, err := filemgr.SaveFileForEntity(src, file, entity, picType)
// // 	if err != nil {
// // 		return "", "", fmt.Errorf("saving image failed: %w", err)
// // 	}

// // 	// extract ID from filename
// // 	id := strings.TrimSuffix(filepath.Base(origName), filepath.Ext(origName))
// // 	path := "/" + filepath.ToSlash(origName)

// // 	return path, id, nil
// // }

// // func saveUploadedFiles(r *http.Request, formKey, fileType, postID, userID string) ([]string, []string, error) {
// // 	files := r.MultipartForm.File[formKey]
// // 	if len(files) == 0 {
// // 		return nil, nil, fmt.Errorf("no %s files uploaded", fileType)
// // 	}

// // 	var paths []string
// // 	var ids []string

// // 	for _, file := range files {
// // 		path, id, err := processSingleImageUpload(file, postID, userID)
// // 		if err != nil {
// // 			return nil, nil, fmt.Errorf("failed to process %s: %w", fileType, err)
// // 		}
// // 		paths = append(paths, path)
// // 		ids = append(ids, id)
// // 	}

// // 	mq.Notify("postpics-uploaded", models.Index{})
// // 	mq.Notify("thumbnail-created", models.Index{})

// // 	return paths, ids, nil
// // }

// // func processSingleImageUpload(file *multipart.FileHeader, postID, userID string) (string, string, error) {
// // 	src, err := file.Open()
// // 	if err != nil {
// // 		return "", "", fmt.Errorf("cannot open image: %w", err)
// // 	}
// // 	defer src.Close()

// // 	img, err := imaging.Decode(src)
// // 	if err != nil {
// // 		return "", "", fmt.Errorf("image decode failed: %w", err)
// // 	}

// // 	uniqueID := utils.GenerateID(16)
// // 	filename := uniqueID + ".jpg"
// // 	originalPath := generateFilePath(feedVideoUploadDir, uniqueID, "jpg")
// // 	thumbDir := filepath.Join(feedVideoUploadDir, "thumb")
// // 	thumbPath := generateFilePath(thumbDir, uniqueID, "jpg")

// // 	if err := ensureDirExists(filepath.Dir(originalPath)); err != nil {
// // 		return "", "", fmt.Errorf("create dir failed: %w", err)
// // 	}
// // 	if err := ensureDirExists(thumbDir); err != nil {
// // 		return "", "", fmt.Errorf("create thumb dir failed: %w", err)
// // 	}

// // 	if err := imaging.Save(img, originalPath); err != nil {
// // 		return "", "", fmt.Errorf("saving original failed: %w", err)
// // 	}

// // 	thumb := imaging.Resize(img, 300, 0, imaging.Lanczos)
// // 	if err := imaging.Save(thumb, thumbPath); err != nil {
// // 		return "", "", fmt.Errorf("saving thumb failed: %w", err)
// // 	}

// // 	UploadFile(src, "/postpic/"+filename, userID, postID)
// // 	return "/postpic/" + filename, uniqueID, nil
// // }
