package utils

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
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
