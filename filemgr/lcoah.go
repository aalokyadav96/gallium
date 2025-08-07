package filemgr

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
)

type EntityType string
type PictureType string

const (
	EntityArtist EntityType = "artist"
	EntityUser   EntityType = "user"
	EntityBaito  EntityType = "baito"
	EntitySong   EntityType = "song"
	EntityPost   EntityType = "post"
	EntityChat   EntityType = "chat"
	EntityEvent  EntityType = "event"
	EntityFarm   EntityType = "farm"
	EntityCrop   EntityType = "crop"
	EntityPlace  EntityType = "place"
	EntityMedia  EntityType = "media"
	EntityFeed   EntityType = "feed"

	PicBanner   PictureType = "banner"
	PicPhoto    PictureType = "photo"
	PicPoster   PictureType = "poster"
	PicSeating  PictureType = "seating"
	PicMember   PictureType = "member"
	PicThumb    PictureType = "thumb"
	PicAudio    PictureType = "audio"
	PicVideo    PictureType = "video"
	PicDocument PictureType = "document"
	PicFile     PictureType = "file"
)

var (
	AllowedExtensions = map[PictureType][]string{
		PicPhoto:    {".jpg", ".jpeg", ".png", ".gif", ".webp"},
		PicThumb:    {".jpg"},
		PicPoster:   {".jpg", ".jpeg", ".png"},
		PicBanner:   {".jpg", ".jpeg", ".png"},
		PicMember:   {".jpg", ".jpeg", ".png"},
		PicSeating:  {".jpg", ".jpeg", ".png"},
		PicAudio:    {".mp3", ".wav", ".aac"},
		PicVideo:    {".mp4", ".mov", ".avi", ".webm"},
		PicDocument: {".pdf", ".doc", ".docx", ".txt"},
		PicFile:     {".pdf", ".doc", ".docx", ".jpg", ".jpeg", ".png", ".mp3", ".mp4", ".mov", ".avi", ".webm"},
	}

	AllowedMIMEs = map[PictureType][]string{
		PicPhoto:   {"image/jpeg", "image/png", "image/gif", "image/webp"},
		PicThumb:   {"image/jpeg"},
		PicPoster:  {"image/jpeg", "image/png"},
		PicBanner:  {"image/jpeg", "image/png"},
		PicMember:  {"image/jpeg", "image/png"},
		PicSeating: {"image/jpeg", "image/png"},
		PicAudio:   {"audio/mpeg", "audio/wav", "audio/aac"},
		PicVideo:   {"video/mp4", "video/quicktime", "video/x-msvideo", "video/webm"},
		PicDocument: {
			"application/pdf", "application/msword",
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			"text/plain",
		},
		PicFile: {
			"application/pdf", "application/msword",
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			"image/jpeg", "image/png", "image/gif", "image/webp",
			"audio/mpeg", "audio/wav", "video/mp4", "video/webm", "video/quicktime", "video/x-msvideo",
		},
	}

	PictureSubfolders = map[PictureType]string{
		PicBanner:   "banner",
		PicPhoto:    "photo",
		PicPoster:   "poster",
		PicSeating:  "seating",
		PicMember:   "member",
		PicThumb:    "thumb",
		PicAudio:    "audio",
		PicVideo:    "videos",
		PicDocument: "docs",
		PicFile:     "files",
	}

	ErrInvalidExtension = errors.New("invalid file extension")
	ErrInvalidMIME      = errors.New("invalid MIME type")
	ErrFileTooLarge     = errors.New("file size exceeds limit")

	LogFunc func(path string, size int64, mimeType string)
)

// SaveFile saves a file to disk with validation, virus scan, and optional renaming
func SaveFile(reader io.Reader, header *multipart.FileHeader, destDir string, maxSize int64, customNameFn func(original string) string) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	picType := detectPicType(destDir)
	if picType == "" {
		return "", fmt.Errorf("unknown picture type for folder: %s", destDir)
	}

	if !isExtensionAllowed(ext, picType) {
		return "", fmt.Errorf("%w: %s for %s", ErrInvalidExtension, ext, picType)
	}

	buf := make([]byte, 512)
	n, err := reader.Read(buf)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read header: %w", err)
	}
	mimeType := http.DetectContentType(buf[:n])

	if mimeType == "application/octet-stream" {
		formMime := header.Header.Get("Content-Type")
		if formMime != "" {
			mimeType = formMime
		}
		if !isMIMEAllowed(mimeType, picType) {
			return "", fmt.Errorf("%w: %s for %s", ErrInvalidMIME, mimeType, picType)
		}
	}

	if !isMIMEAllowed(mimeType, picType) {
		return "", fmt.Errorf("%w: %s for %s", ErrInvalidMIME, mimeType, picType)
	}

	// dummy virus scan
	if err := ScanForViruses(header.Filename); err != nil {
		return "", fmt.Errorf("virus scan failed: %w", err)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", destDir, err)
	}

	filename := ""
	if customNameFn != nil {
		filename = strings.TrimSpace(customNameFn(header.Filename))
	}
	if filename == "" {
		filename = uuid.New().String() + ext
	} else {
		filename = ensureSafeFilename(filename, ext)
	}

	fullPath := filepath.Join(destDir, filename)
	out, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", fullPath, err)
	}
	defer out.Close()

	if _, err := out.Write(buf[:n]); err != nil {
		return "", fmt.Errorf("write header: %w", err)
	}

	written, err := io.Copy(out, io.LimitReader(reader, maxSize-int64(n)))
	if err != nil {
		return "", fmt.Errorf("write body: %w", err)
	}
	if maxSize > 0 && written+int64(n) > maxSize {
		return "", ErrFileTooLarge
	}

	if LogFunc != nil {
		LogFunc(fullPath, written+int64(n), mimeType)
	}

	return filename, nil
}

func SaveFileForEntity(file multipart.File, header *multipart.FileHeader, entity EntityType, picType PictureType) (string, error) {
	defer file.Close()
	dest := ResolvePath(entity, picType)

	buf, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	if isImageType(picType) {
		img, _, err := image.Decode(bytes.NewReader(buf))
		if err == nil {
			strip, err := stripEXIF(img)
			if err == nil {
				buf = strip.Bytes()
			}

			// Save after EXIF strip, before thumbnail/meta
			fileName, err := SaveFile(bytes.NewReader(buf), header, dest, 10<<20, nil)
			if err != nil {
				return "", err
			}

			_ = generateThumbnail(img, entity, fileName)
			_ = ExtractImageMetadata(img, len(buf))

			return fileName, nil
		}
		// fallback to normal save if decode fails
	}

	// For non-images or failed decode
	fileName, err := SaveFile(bytes.NewReader(buf), header, dest, 10<<20, nil)
	if err != nil {
		return "", err
	}

	return fileName, nil
}

// func SaveFileForEntity(file multipart.File, header *multipart.FileHeader, entity EntityType, picType PictureType) (string, error) {
// 	defer file.Close()
// 	dest := ResolvePath(entity, picType)

// 	buf, err := io.ReadAll(file)
// 	if err != nil {
// 		return "", fmt.Errorf("read file: %w", err)
// 	}

// 	if isImageType(picType) {
// 		img, _, err := image.Decode(bytes.NewReader(buf))
// 		if err == nil {
// 			strip, err := stripEXIF(img)
// 			if err == nil {
// 				buf = strip.Bytes()
// 			}
// 			_ = generateThumbnail(img, entity, header.Filename)
// 			_ = ExtractImageMetadata(img, len(buf))
// 		}
// 	}

// 	fileName, err := SaveFile(bytes.NewReader(buf), header, dest, 10<<20, nil)
// 	if err != nil {
// 		return "", err
// 	}

// 	return fileName, nil
// }

func ScanForViruses(fileName string) error {
	if strings.Contains(fileName, "virus") {
		return fmt.Errorf("virus signature matched")
	}
	return nil
}

func stripEXIF(img image.Image) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 90})
	if err != nil {
		return nil, err
	}
	return buf, nil
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
	allowed, ok := AllowedExtensions[picType]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if ext == a {
			return true
		}
	}
	return false
}

func isMIMEAllowed(mimeType string, picType PictureType) bool {
	allowed, ok := AllowedMIMEs[picType]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if mimeType == a {
			return true
		}
	}
	return false
}

func ResolvePath(entity EntityType, picType PictureType) string {
	subfolder, ok := PictureSubfolders[picType]
	if !ok || subfolder == "" {
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

func generateThumbnail(img image.Image, entity EntityType, baseFilename string) error {
	resized := imaging.Resize(img, 200, 0, imaging.Lanczos) // maintain aspect ratio
	name := strings.TrimSuffix(baseFilename, filepath.Ext(baseFilename)) + ".jpg"
	path := filepath.Join(ResolvePath(entity, PicThumb), name)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}

	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create thumbnail: %w", err)
	}
	defer out.Close()

	if err := jpeg.Encode(out, resized, &jpeg.Options{Quality: 85}); err != nil {
		return fmt.Errorf("encode thumbnail: %w", err)
	}

	if LogFunc != nil {
		LogFunc(path, 0, "image/jpeg")
	}

	return nil
}

func SaveFormFile(r *multipart.Form, formKey string, entity EntityType, picType PictureType, required bool) (string, error) {
	files := r.File[formKey]
	if len(files) == 0 {
		if required {
			return "", fmt.Errorf("missing required file: %s", formKey)
		}
		return "", nil
	}
	file, err := files[0].Open()
	if err != nil {
		return "", fmt.Errorf("open %s: %w", formKey, err)
	}
	return SaveFileForEntity(file, files[0], entity, picType)
}

func SaveFormFiles(form *multipart.Form, formKey string, entity EntityType, picType PictureType, required bool) ([]string, error) {
	files := form.File[formKey]
	if len(files) == 0 {
		if required {
			return nil, fmt.Errorf("missing required files: %s", formKey)
		}
		return nil, nil
	}

	var saved []string
	var errs []string

	for _, hdr := range files {
		file, err := hdr.Open()
		if err != nil {
			errs = append(errs, fmt.Sprintf("open %s: %v", hdr.Filename, err))
			continue
		}
		name, err := SaveFileForEntity(file, hdr, entity, picType)
		if err != nil {
			errs = append(errs, fmt.Sprintf("save %s: %v", hdr.Filename, err))
			continue
		}
		saved = append(saved, name)
	}

	if len(errs) > 0 {
		return saved, fmt.Errorf("one or more errors saving files: %s", strings.Join(errs, "; "))
	}
	return saved, nil
}

func SaveImageWithThumb(file multipart.File, header *multipart.FileHeader, entity EntityType, picType PictureType, thumbWidth int, userid string) (string, string, error) {
	defer file.Close()

	buf, err := io.ReadAll(file)
	if err != nil {
		return "", "", fmt.Errorf("failed to read file: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(buf))
	if err != nil {
		return "", "", fmt.Errorf("failed to decode image %q: %w", header.Filename, err)
	}

	if err := ValidateImageDimensions(img, 3000, 3000); err != nil {
		return "", "", fmt.Errorf("image %q failed dimension validation: %w", header.Filename, err)
	}

	origPath := ResolvePath(entity, picType)
	origName, err := SaveFile(bytes.NewReader(buf), header, origPath, 10<<20, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to save original image to %q: %w", origPath, err)
	}

	var thumbName string
	thumbImg := imaging.Resize(img, thumbWidth, 0, imaging.Lanczos)
	switch picType {
	case PicPhoto:
		thumbName = userid + ".jpg"
	case PicBanner:
		thumbName = origName
	}

	thumbDir := ResolvePath(entity, PicThumb)
	if err := os.MkdirAll(thumbDir, 0o755); err != nil {
		return origName, "", fmt.Errorf("failed to create thumbnail directory %q: %w", thumbDir, err)
	}

	thumbPath := filepath.Join(thumbDir, thumbName)
	out, err := os.Create(thumbPath)
	if err != nil {
		return origName, "", fmt.Errorf("failed to create thumbnail file %q: %w", thumbPath, err)
	}
	defer out.Close()

	if err := jpeg.Encode(out, thumbImg, &jpeg.Options{Quality: 85}); err != nil {
		return origName, "", fmt.Errorf("failed to encode thumbnail JPEG: %w", err)
	}

	if LogFunc != nil {
		LogFunc(thumbPath, 0, "image/jpeg")
	}

	return origName, thumbName, nil
}

func ValidateImageDimensions(img image.Image, maxWidth, maxHeight int) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width > maxWidth || height > maxHeight {
		return fmt.Errorf("image dimensions %dx%d exceed allowed maximum %dx%d", width, height, maxWidth, maxHeight)
	}
	return nil
}
