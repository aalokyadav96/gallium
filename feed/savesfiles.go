package feed

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

func ensureDirExists(dir string) error {
	return os.MkdirAll(dir, 0755)
}

func generateFilePath(baseDir, uniqueID, extension string) string {
	return filepath.Join(baseDir, fmt.Sprintf("%s.%s", uniqueID, extension))
}

func saveUploadedFile(src io.Reader, destPath string) error {
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, src)
	return err
}

func getUploadedFile(r *http.Request, formKey string) (*multipart.FileHeader, error) {
	files := r.MultipartForm.File[formKey]
	if len(files) == 0 {
		return nil, nil
	}
	return files[0], nil
}
