package filemgr

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

func ScanForViruses(fileName string) error {
	if strings.Contains(fileName, "virus") {
		return fmt.Errorf("virus signature matched")
	}
	return nil
}

func stripEXIF(img image.Image) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 90})
	return buf, err
}

func ExtractImageMetadata(img image.Image, size int) error {
	b := img.Bounds()
	fmt.Printf("Image metadata - Width: %d, Height: %d, Size: %d bytes\n", b.Dx(), b.Dy(), size)
	return nil
}

func detectPicType(destDir string) PictureType {
	parts := strings.Split(destDir, string(os.PathSeparator))
	if len(parts) == 0 {
		return ""
	}
	last := strings.ToLower(parts[len(parts)-1])
	for picType, folder := range PictureSubfolders {
		if folder == last {
			return picType
		}
	}
	return ""
}

func ensureSafeFilename(name, ext string) string {
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "_")
	reg := regexp.MustCompile(`[^a-zA-Z0-9_\-]`)
	name = reg.ReplaceAllString(name, "")
	return name + ext
}

func isExtensionAllowed(ext string, picType PictureType) bool {
	allowed := AllowedExtensions[picType]
	for _, a := range allowed {
		if ext == a {
			return true
		}
	}
	return false
}

func isMIMEAllowed(mimeType string, picType PictureType) bool {
	allowed := AllowedMIMEs[picType]
	for _, a := range allowed {
		if mimeType == a {
			return true
		}
	}
	return false
}

func ResolvePath(entity EntityType, picType PictureType) string {
	subfolder := PictureSubfolders[picType]
	if subfolder == "" {
		subfolder = "misc"
	}
	return filepath.Join("static", "uploads", strings.ToLower(string(entity)), subfolder)
}

func isImageType(picType PictureType) bool {
	switch picType {
	case PicBanner, PicPhoto, PicMember, PicPoster, PicSeating:
		return true
	default:
		return false
	}
}

func ValidateImageDimensions(img image.Image, maxWidth, maxHeight int) error {
	bounds := img.Bounds()
	if bounds.Dx() > maxWidth || bounds.Dy() > maxHeight {
		return fmt.Errorf("image dimensions %dx%d exceed max %dx%d", bounds.Dx(), bounds.Dy(), maxWidth, maxHeight)
	}
	return nil
}

func getSafeFilename(original, ext string, fn func(string) string) string {
	name := ""
	if fn != nil {
		name = strings.TrimSpace(fn(original))
	}
	if name == "" {
		name = uuid.New().String() + ext
	} else {
		name = ensureSafeFilename(name, ext)
	}
	return name
}
