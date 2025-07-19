package feed

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"naevis/mq"
	"naevis/utils"
)

type MediaType string

const (
	Video MediaType = "video"
	Audio MediaType = "audio"
)

func processUpload(r *http.Request, formKey, postID, userID string, mediaType MediaType) ([]int, []string, []string, error) {
	file, err := getUploadedFile(r, formKey)
	if err != nil || file == nil {
		return nil, nil, nil, err
	}

	uniqueID := utils.GenerateID(16)
	baseDir := getUploadBaseDir(mediaType)
	uploadDir := filepath.Join(baseDir, uniqueID)

	if err := ensureDirExists(uploadDir); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	filePath := generateFilePath(uploadDir, uniqueID, getFileExt(mediaType))
	src, err := file.Open()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	if err := saveUploadedFile(src, filePath); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to save uploaded file: %w", err)
	}

	UploadFile(src, filePath, userID, postID)

	var resolutions []int
	var outputPath string

	if mediaType == Video {
		width, height, err := getVideoDimensions(filePath)
		if err != nil {
			os.RemoveAll(uploadDir)
			return nil, nil, nil, fmt.Errorf("failed to get video dimensions: %w", err)
		}
		resolutions, outputPath = processVideoResolutions(filePath, uploadDir, uniqueID, width, height)

		if err := createDefaultPoster(filePath, uploadDir, uniqueID); err != nil {
			os.RemoveAll(uploadDir)
			return nil, nil, nil, fmt.Errorf("failed to create poster: %w", err)
		}

		go createSubtitleFile(uniqueID)
		mq.Notify("postpics-uploaded", mq.Index{})
	} else {
		resolutions, outputPath = processAudioResolutions(filePath, uploadDir, uniqueID)
		go createSubtitleFile(uniqueID)
		mq.Notify("postaudio-uploaded", mq.Index{})
	}

	return resolutions, []string{outputPath}, []string{uniqueID}, nil
}

func getUploadBaseDir(mediaType MediaType) string {
	switch mediaType {
	case Audio:
		return feedAudioUploadDir
	default:
		return feedVideoUploadDir
	}
}

func getFileExt(mediaType MediaType) string {
	switch mediaType {
	case Audio:
		return "mp3"
	default:
		return "mp4"
	}
}

func saveUploadedVideoFile(r *http.Request, formKey, postID, userID string) ([]int, []string, []string, error) {
	return processUpload(r, formKey, postID, userID, Video)
}

func saveUploadedAudioFile(r *http.Request, formKey, postID, userID string) ([]int, []string, []string, error) {
	return processUpload(r, formKey, postID, userID, Audio)
}
