package utils

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func ParseFloat(s string) float64 {
	val, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return val
}

func ParseInt(s string) int {
	val, _ := strconv.Atoi(strings.TrimSpace(s))
	return val
}

func ParseDate(s string) *time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return &t
}

func SaveUploadedImage(file multipart.File, header *multipart.FileHeader) (string, error) {
	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	dstPath := filepath.Join("static", "uploads", "crops", filename)

	os.MkdirAll(filepath.Dir(dstPath), 0755)
	out, err := os.Create(dstPath)
	if err != nil {
		return "", err
	}
	defer out.Close()
	_, err = io.Copy(out, file)
	return "/uploads/crops/" + filename, err
}
