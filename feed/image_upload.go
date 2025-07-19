package feed

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"

	"naevis/mq"
	"naevis/utils"

	"github.com/disintegration/imaging"
)

func processSingleImageUpload(file *multipart.FileHeader, postID, userID string) (string, string, error) {
	src, err := file.Open()
	if err != nil {
		return "", "", fmt.Errorf("failed to open image file: %w", err)
	}
	defer src.Close()

	img, err := imaging.Decode(src)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode image: %w", err)
	}

	uniqueID := utils.GenerateID(16)
	fileName := uniqueID + ".jpg"

	originalPath := generateFilePath(feedVideoUploadDir, uniqueID, "jpg")
	thumbDir := filepath.Join(feedVideoUploadDir, "thumb")
	thumbnailPath := generateFilePath(thumbDir, uniqueID, "jpg")

	if err := ensureDirExists(filepath.Dir(originalPath)); err != nil {
		return "", "", fmt.Errorf("failed to create upload directory: %w", err)
	}
	if err := ensureDirExists(thumbDir); err != nil {
		return "", "", fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	if err := imaging.Save(img, originalPath); err != nil {
		return "", "", fmt.Errorf("failed to save original image: %w", err)
	}

	thumbImg := imaging.Resize(img, 300, 0, imaging.Lanczos)
	if err := imaging.Save(thumbImg, thumbnailPath); err != nil {
		return "", "", fmt.Errorf("failed to save thumbnail: %w", err)
	}

	UploadFile(src, "/postpic/"+fileName, userID, postID)

	return "/postpic/" + fileName, uniqueID, nil
}

func saveUploadedFiles(r *http.Request, formKey, fileType, postID, userID string) ([]string, []string, error) {
	files := r.MultipartForm.File[formKey]
	if len(files) == 0 {
		return nil, nil, nil
	}

	var savedPaths, savedNames []string

	for _, file := range files {
		thumbPath, uniqueID, err := processSingleImageUpload(file, postID, userID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process %s file: %w", fileType, err)
		}
		savedPaths = append(savedPaths, thumbPath)
		savedNames = append(savedNames, uniqueID)
	}

	m := mq.Index{}
	mq.Notify("postpics-uploaded", m)
	mq.Notify("thumbnail-created", m)

	return savedPaths, savedNames, nil
}
