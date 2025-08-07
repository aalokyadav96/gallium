package feed

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Subtitle struct {
	Index   int
	Start   string // Format: "hh:mm:ss.mmm"
	End     string // Format: "hh:mm:ss.mmm"
	Content string
}

func createSubtitleFile(uniqueID string) {
	subtitles := []Subtitle{
		{1, "00:00:00.000", "00:00:01.000", "Welcome to the video!"},
		{2, "00:00:01.001", "00:00:02.000", "In this video, we'll learn how to create subtitles in Go."},
		{3, "00:00:02.001", "00:00:03.000", "Let's get started!"},
	}

	if err := writeVTT(uniqueID, "en", subtitles); err != nil {
		fmt.Printf("subtitle creation failed: %v\n", err)
		return
	}

	fmt.Printf("Subtitle file for %s created.\n", uniqueID)
}

func writeVTT(uniqueID, lang string, subtitles []Subtitle) error {
	dir := filepath.Join("static", "uploads", "subtitles", uniqueID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	filePath := filepath.Join(dir, fmt.Sprintf("%s-%s.vtt", uniqueID, lang))
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("create subtitle file: %w", err)
	}
	defer file.Close()

	var b strings.Builder
	b.WriteString("WEBVTT\n\n")

	for _, s := range subtitles {
		if !isValidTimestamp(s.Start) || !isValidTimestamp(s.End) {
			return fmt.Errorf("invalid timestamp format at index %d", s.Index)
		}
		b.WriteString(fmt.Sprintf("%d\n%s --> %s\n%s\n\n", s.Index, s.Start, s.End, s.Content))
	}

	if _, err := file.WriteString(b.String()); err != nil {
		return fmt.Errorf("write subtitles: %w", err)
	}
	return nil
}

func isValidTimestamp(ts string) bool {
	// Basic format validation: "hh:mm:ss.mmm" = len(12), contains exactly 2 ':' and 1 '.'
	if len(ts) != 12 {
		return false
	}
	if strings.Count(ts, ":") != 2 || strings.Count(ts, ".") != 1 {
		return false
	}
	return true
}
