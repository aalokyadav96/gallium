package feed

import (
	"fmt"
	"path/filepath"
)

// Directory to store uploaded images/videos
const feedVideoUploadDir = "./static/postpic/"
const feedAudioUploadDir = "./static/postpic/"

// -------------------------
// Generic helper functions
// -------------------------

// processVideoResolutions creates versions of the video at multiple resolutions.
// It returns a slice of available resolution heights and the path of the highest resolution file.
func processVideoResolutions(originalFilePath, uploadDir, uniqueID string, origWidth, origHeight int) ([]int, string) {
	// Define available resolutions.
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
		// Only downscale (avoid upscaling)
		if newWidth > origWidth || newHeight > origHeight {
			continue
		}

		outputFilePath := generateFilePath(uploadDir, uniqueID+"-"+res.Label, "mp4")
		outputPosterPath := generateFilePath(uploadDir, uniqueID, "jpg")

		// processVideoResolution is assumed to perform the resolution conversion.
		if err := processVideoResolution(originalFilePath, outputFilePath, fmt.Sprintf("%dx%d", newWidth, newHeight)); err != nil {
			fmt.Printf("Skipping %s due to error: %v\n", res.Label, err)
			continue
		}

		// Create a poster image for this resolution.
		// if err := CreatePoster(outputFilePath, outputPosterPath, "00:00:01"); err != nil {
		if err := CreatePoster(originalFilePath, outputPosterPath, "00:00:01.500"); err != nil {
			fmt.Printf("Skipping %s poster due to error: %v\n", res.Label, err)
			continue
		}

		highestResolutionPath = "/postpic/" + uniqueID + "/" + filepath.Base(outputFilePath)
		availableResolutions = append(availableResolutions, newHeight)
	}

	return availableResolutions, highestResolutionPath
}
