package feed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// fitResolution scales original dimensions to fit within max bounds while maintaining aspect ratio
func fitResolution(origW, origH, maxW, maxH int) (int, int) {
	ratio := math.Min(float64(maxW)/float64(origW), float64(maxH)/float64(origH))
	return int(float64(origW) * ratio), int(float64(origH) * ratio)
}

// getVideoDimensions returns width and height of a video using ffprobe
func getVideoDimensions(videoPath string) (int, int, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=p=0",
		videoPath,
	)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return 0, 0, fmt.Errorf("ffprobe execution failed: %w", err)
	}

	parts := strings.Split(strings.TrimSpace(out.String()), ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected ffprobe output: %s", out.String())
	}

	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse width: %w", err)
	}

	height, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse height: %w", err)
	}

	return width, height, nil
}

// processVideoResolution transcodes a video to a specific resolution (e.g., 1280x720)
func processVideoResolution(inputPath, outputPath, resolution string) error {
	cmd := exec.Command(
		"ffmpeg", "-i", inputPath,
		"-vf", fmt.Sprintf("scale=%s", resolution),
		"-c:v", "libx264", "-crf", "23",
		"-preset", "veryfast",
		"-c:a", "aac", "-b:a", "128k",
		"-movflags", "+faststart",
		outputPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg transcoding failed: %w - %s", err, stderr.String())
	}

	return nil
}

func CreatePoster(videoPath, posterPath string) error {
	if err := os.MkdirAll(filepath.Dir(posterPath), 0755); err != nil {
		return fmt.Errorf("failed to create poster directory: %w", err)
	}

	posterExt := filepath.ToSlash(posterPath + ".jpg")
	log.Println(videoPath, posterExt)

	duration, err := getVideoDuration(videoPath)
	if err != nil {
		return fmt.Errorf("could not determine video duration: %w", err)
	}
	log.Println(duration)

	if duration < 3.0 {
		duration = 3.0
	}

	randomTime := rand.Float64() * (duration - 1.5)
	timestamp := formatTimestamp(randomTime)

	cmd := exec.Command(
		"ffmpeg",
		"-ss", timestamp,
		"-i", videoPath,
		"-vframes", "1",
		"-q:v", "2",
		posterExt,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("poster creation failed: %w - %s", err, stderr.String())
	}

	return nil
}

// getVideoDuration returns the video duration in seconds using ffprobe.
func getVideoDuration(path string) (float64, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "json",
		path,
	)

	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	var result struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}

	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		return 0, fmt.Errorf("unmarshal duration: %w", err)
	}

	return strconv.ParseFloat(result.Format.Duration, 64)
}

// formatTimestamp converts seconds (e.g. 12.345) to "hh:mm:ss.SSS"
func formatTimestamp(seconds float64) string {
	totalMs := int(seconds * 1000)
	h := totalMs / 3600000
	m := (totalMs % 3600000) / 60000
	s := (totalMs % 60000) / 1000
	ms := totalMs % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}

func ExtractVideoDuration(videoPath string) float64 {
	res, err := getVideoDuration(videoPath)
	if err != nil {
		return 0.0
	}
	return res
}

// Normalize and process audio, returning a standard resolution (bitrate)
func processAudioResolutions(originalFilePath, uploadDir, uniqueID string) ([]int, string) {
	outputPath := filepath.Join(uploadDir, uniqueID+".mp3")

	cmd := exec.Command(
		"ffmpeg", "-i", originalFilePath,
		"-vn",                // no video
		"-c:a", "libmp3lame", // convert to mp3
		"-b:a", "128k", // standard audio bitrate
		"-y", // overwrite
		outputPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("audio processing failed: %v\nstderr: %s\n", err, stderr.String())
		return []int{}, originalFilePath
	}

	return []int{128}, outputPath
}

// // Extracts a default poster frame at 1 second into the video
// func createDefaultPoster(originalFilePath, uploadDir, uniqueID string) error {
// 	posterPath := generateFilePath(uploadDir, uniqueID, "jpg")
// 	return CreatePoster(originalFilePath, posterPath)
// }
