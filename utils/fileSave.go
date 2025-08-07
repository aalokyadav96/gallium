package utils

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
)

func SaveFile(file multipart.File, header *multipart.FileHeader, folder string) (string, error) {
	filename := fmt.Sprintf("%s%s", GenerateID(12), filepath.Ext(header.Filename))
	filePath := fmt.Sprintf("%s/%s", folder, filename)
	fmt.Println("_____________", filePath)
	out, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		return "", err
	}

	return filename, nil
}

func GuessMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	default:
		return "application/octet-stream"
	}
}
