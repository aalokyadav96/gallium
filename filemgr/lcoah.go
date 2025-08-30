package filemgr

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"naevis/mq"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
)

// SaveFile saves a file with validation, size limit and virus scan.
// Returns the saved filename (base name).
func SaveFile(
	reader io.Reader,
	header *multipart.FileHeader,
	destDir string,
	maxSize int64,
	customNameFn func(original string) string,
) (string, error) {

	ext := strings.ToLower(filepath.Ext(header.Filename))
	picType := detectPicType(destDir)
	if picType == "" {
		return "", fmt.Errorf("unknown picture type for folder: %s", destDir)
	}

	if !isExtensionAllowed(ext, picType) {
		return "", fmt.Errorf("%w: %s for %s", ErrInvalidExtension, ext, picType)
	}

	// Peek first 512 bytes for MIME detection
	buf := make([]byte, 512)
	n, err := io.ReadFull(io.LimitReader(reader, 512), buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", fmt.Errorf("read header: %w", err)
	}

	mimeType := strings.ToLower(http.DetectContentType(buf[:n]))
	if mimeType == "application/octet-stream" {
		formMime := strings.ToLower(header.Header.Get("Content-Type"))
		if formMime != "" && isMIMEAllowed(formMime, picType) {
			mimeType = formMime
		}
	}

	if !isMIMEAllowed(mimeType, picType) {
		return "", fmt.Errorf("%w: %s for %s", ErrInvalidMIME, mimeType, picType)
	}

	if !extMatchesMIME(ext, mimeType, picType) {
		return "", fmt.Errorf("extension %s does not match MIME type %s for %s", ext, mimeType, picType)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", destDir, err)
	}

	filename := getSafeFilename(header.Filename, ext, customNameFn)
	fullPath := filepath.Join(destDir, filename)

	out, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", fullPath, err)
	}
	defer out.Close()

	// write initial bytes we already peeked
	if _, err := out.Write(buf[:n]); err != nil {
		return "", fmt.Errorf("write header: %w", err)
	}

	written, err := io.Copy(out, io.LimitReader(reader, maxSize-int64(n)))
	if err != nil {
		return "", fmt.Errorf("write body: %w", err)
	}

	totalWritten := written + int64(n)
	if maxSize > 0 && totalWritten > maxSize {
		_ = os.Remove(fullPath)
		return "", ErrFileTooLarge
	}

	// Virus scan after full file present
	if err := ScanForViruses(fullPath); err != nil {
		_ = os.Remove(fullPath)
		return "", fmt.Errorf("virus scan failed: %w", err)
	}

	// Log via LogFunc if present
	if LogFunc != nil {
		LogFunc(filename, totalWritten, mimeType)
	}

	return filename, nil
}

// Convenience functions for saving form files
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
		return saved, fmt.Errorf("errors saving files: %s", strings.Join(errs, "; "))
	}
	return saved, nil
}

func SaveFormFilesByKeys(form *multipart.Form, keys []string, entityType EntityType, pictureType PictureType, required bool) ([]string, error) {
	var urls []string
	var errs []string
	for _, key := range keys {
		partial, err := SaveFormFiles(form, key, entityType, pictureType, required)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", key, err))
		}
		urls = append(urls, partial...)
	}
	if len(errs) > 0 {
		return urls, fmt.Errorf("errors: %s", strings.Join(errs, "; "))
	}
	return urls, nil
}

// SaveImageWithThumb saves an image, validates dimensions and creates a thumbnail; returns image name and thumbnail name (if created).
func SaveImageWithThumb(file multipart.File, header *multipart.FileHeader, entity EntityType, picType PictureType, thumbWidth int, userid string) (string, string, error) {
	defer file.Close()

	origPath := ResolvePath(entity, picType)
	origName, err := SaveFile(file, header, origPath, 10<<20, nil)
	if err != nil {
		return "", "", fmt.Errorf("save original: %w", err)
	}

	fullPath := filepath.Join(origPath, origName)

	f, err := os.Open(fullPath)
	if err != nil {
		return origName, "", fmt.Errorf("open for decode: %w", err)
	}
	img, _, err := image.Decode(f)
	_ = f.Close()
	if err != nil {
		return origName, "", fmt.Errorf("decode %q: %w", header.Filename, err)
	}

	ext := strings.ToLower(filepath.Ext(fullPath))
	// Re-encode as PNG if not PNG
	if ext != ".png" {
		pngPath := strings.TrimSuffix(fullPath, ext) + ".png"
		out, err := os.Create(pngPath)
		if err != nil {
			return origName, "", fmt.Errorf("create png %s: %w", pngPath, err)
		}
		if err := png.Encode(out, img); err != nil {
			_ = out.Close()
			_ = os.Remove(pngPath)
			return origName, "", fmt.Errorf("encode png: %w", err)
		}
		_ = out.Close()
		_ = os.Remove(fullPath)
		fullPath = pngPath
		origName = filepath.Base(pngPath)
	}

	if err := ValidateImageDimensions(img, 3000, 3000); err != nil {
		return origName, "", fmt.Errorf("invalid image %q: %w", header.Filename, err)
	}

	// Notify MQ (best-effort)
	go func(p, ent, name, pt, uid string) {
		_ = mq.NotifyImageSaved(p, ent, name, pt, uid)
	}(fullPath, string(entity), origName, string(picType), userid)

	// Thumbnail → create with unique name to avoid collisions
	if img.Bounds().Dx() > thumbWidth || img.Bounds().Dy() > thumbWidth {
		thumbImg := imaging.Resize(img, thumbWidth, 0, imaging.Lanczos)
		thumbName := userid + ".jpg"
		thumbDir := ResolvePath(entity, PicThumb)

		if err := os.MkdirAll(thumbDir, 0o755); err != nil {
			return origName, "", fmt.Errorf("mkdir %q: %w", thumbDir, err)
		}
		thumbPath := filepath.Join(thumbDir, thumbName)

		out, err := os.Create(thumbPath)
		if err != nil {
			return origName, "", fmt.Errorf("create thumb %q: %w", thumbPath, err)
		}
		if err := jpeg.Encode(out, thumbImg, &jpeg.Options{Quality: 85}); err != nil {
			_ = out.Close()
			_ = os.Remove(thumbPath)
			return origName, "", fmt.Errorf("encode thumb: %w", err)
		}
		_ = out.Close()

		if LogFunc != nil {
			LogFunc(thumbPath, 0, "image/jpeg")
		}
		return origName, thumbName, nil
	}

	if LogFunc != nil {
		LogFunc(origName, 0, "image/png")
	}
	return origName, "", nil
}

// package filemgr

// import (
// 	"fmt"
// 	"image"
// 	_ "image/gif"
// 	"image/jpeg"
// 	"image/png"
// 	"io"
// 	"mime/multipart"
// 	"net/http"
// 	"os"
// 	"os/exec"
// 	"path/filepath"
// 	"strconv"
// 	"strings"
// 	"time"

// 	"naevis/mq"

// 	"github.com/disintegration/imaging"
// 	_ "golang.org/x/image/webp"
// )

// const defaultThumbWidth = 200

// // SaveFile saves a file with validation, size limit and virus scan.
// // Returns the saved filename (base name).
// func SaveFile(
// 	reader io.Reader,
// 	header *multipart.FileHeader,
// 	destDir string,
// 	maxSize int64,
// 	customNameFn func(original string) string,
// ) (string, error) {

// 	ext := strings.ToLower(filepath.Ext(header.Filename))
// 	picType := detectPicType(destDir)
// 	if picType == "" {
// 		return "", fmt.Errorf("unknown picture type for folder: %s", destDir)
// 	}

// 	if !isExtensionAllowed(ext, picType) {
// 		return "", fmt.Errorf("%w: %s for %s", ErrInvalidExtension, ext, picType)
// 	}

// 	// Peek first 512 bytes for MIME detection
// 	buf := make([]byte, 512)
// 	n, err := io.ReadFull(io.LimitReader(reader, 512), buf)
// 	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
// 		return "", fmt.Errorf("read header: %w", err)
// 	}

// 	mimeType := strings.ToLower(http.DetectContentType(buf[:n]))
// 	if mimeType == "application/octet-stream" {
// 		formMime := strings.ToLower(header.Header.Get("Content-Type"))
// 		if formMime != "" && isMIMEAllowed(formMime, picType) {
// 			mimeType = formMime
// 		}
// 	}

// 	if !isMIMEAllowed(mimeType, picType) {
// 		return "", fmt.Errorf("%w: %s for %s", ErrInvalidMIME, mimeType, picType)
// 	}

// 	if !extMatchesMIME(ext, mimeType, picType) {
// 		return "", fmt.Errorf("extension %s does not match MIME type %s for %s", ext, mimeType, picType)
// 	}

// 	if err := os.MkdirAll(destDir, 0o755); err != nil {
// 		return "", fmt.Errorf("mkdir %s: %w", destDir, err)
// 	}

// 	filename := getSafeFilename(header.Filename, ext, customNameFn)
// 	fullPath := filepath.Join(destDir, filename)

// 	out, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
// 	if err != nil {
// 		return "", fmt.Errorf("create %s: %w", fullPath, err)
// 	}
// 	defer out.Close()

// 	// write initial bytes we already peeked
// 	if _, err := out.Write(buf[:n]); err != nil {
// 		return "", fmt.Errorf("write header: %w", err)
// 	}

// 	written, err := io.Copy(out, io.LimitReader(reader, maxSize-int64(n)))
// 	if err != nil {
// 		return "", fmt.Errorf("write body: %w", err)
// 	}

// 	totalWritten := written + int64(n)
// 	if maxSize > 0 && totalWritten > maxSize {
// 		_ = os.Remove(fullPath)
// 		return "", ErrFileTooLarge
// 	}

// 	// Virus scan after full file present
// 	if err := ScanForViruses(fullPath); err != nil {
// 		_ = os.Remove(fullPath)
// 		return "", fmt.Errorf("virus scan failed: %w", err)
// 	}

// 	// Log via LogFunc if present
// 	if LogFunc != nil {
// 		LogFunc(filename, totalWritten, mimeType)
// 	}

// 	return filename, nil
// }

// // SaveFileForEntity saves file to entity/picType path and kicks off processing (thumbnails, posters, metadata).
// // It tries to be safe: only re-encodes when needed, runs heavy tasks asynchronously where acceptable.
// func SaveFileForEntity(file multipart.File, header *multipart.FileHeader, entity EntityType, picType PictureType) (string, error) {
// 	// ensure the incoming multipart.File is closed when we're done (SaveFile will read it)
// 	defer file.Close()

// 	path := ResolvePath(entity, picType)
// 	filename, err := SaveFile(file, header, path, 10<<20, nil)
// 	if err != nil {
// 		return "", err
// 	}

// 	fullPath := filepath.Join(path, filename)
// 	ext := strings.ToLower(filepath.Ext(fullPath))

// 	// If this is an image type -> do image-specific processing
// 	if isImageType(picType) {
// 		f, err := os.Open(fullPath)
// 		if err != nil {
// 			return "", fmt.Errorf("reopen saved file: %w", err)
// 		}
// 		img, _, err := image.Decode(f)
// 		_ = f.Close()
// 		if err != nil {
// 			// if decode fails, keep the uploaded file as-is
// 			if LogFunc != nil {
// 				LogFunc(filename, 0, "unknown")
// 			}
// 			return filename, nil
// 		}

// 		// Re-encode as PNG only if the saved file is not already PNG
// 		if ext != ".png" {
// 			pngPath := strings.TrimSuffix(fullPath, ext) + ".png"
// 			out, err := os.Create(pngPath)
// 			if err != nil {
// 				return "", fmt.Errorf("create png %s: %w", pngPath, err)
// 			}
// 			if err := png.Encode(out, img); err != nil {
// 				_ = out.Close()
// 				_ = os.Remove(pngPath)
// 				return "", fmt.Errorf("encode png: %w", err)
// 			}
// 			_ = out.Close()
// 			// remove original after successful png creation
// 			if err := os.Remove(fullPath); err != nil {
// 				// non-fatal warning
// 				if LogFunc != nil {
// 					LogFunc(fmt.Sprintf("warning: failed to remove original %s", fullPath), 0, "")
// 				}
// 			}
// 			fullPath = pngPath
// 			filename = filepath.Base(pngPath)
// 			ext = ".png"
// 		}

// 		// Notify MQ in goroutine (non-blocking)
// 		go func(p, ent, fname string, pt string) {
// 			// best-effort notify
// 			_ = mq.NotifyImageSaved(p, ent, fname, pt, "")
// 		}(fullPath, string(entity), filename, string(picType))

// 		// Generate thumbnail asynchronously if image is larger than defaultThumbWidth
// 		if img.Bounds().Dx() > defaultThumbWidth || img.Bounds().Dy() > defaultThumbWidth {
// 			imgCopy := imaging.Clone(img)
// 			go func(img image.Image, ent EntityType, fname string) {
// 				if err := generateThumbnail(img, ent, fname, defaultThumbWidth); err != nil {
// 					if LogFunc != nil {
// 						LogFunc(fmt.Sprintf("warning: thumbnail failed for %s: %v", fname, err), 0, "")
// 					}
// 				}
// 			}(imgCopy, entity, filename)
// 		}

// 		// Extract metadata asynchronously with a stable unique id
// 		go func(img image.Image, uid string) {
// 			if err := ExtractImageMetadata(img, uid); err != nil {
// 				if LogFunc != nil {
// 					LogFunc(fmt.Sprintf("warning: metadata extraction failed for %s: %v", filename, err), 0, "")
// 				}
// 			}
// 		}(imaging.Clone(img), generateUniqueID())

// 		// Logging for the saved file
// 		if LogFunc != nil {
// 			LogFunc(filename, 0, "image/png")
// 		}
// 		return filename, nil
// 	}

// 	// If it's a video by PicType or extension, generate poster asynchronously and return filename
// 	if picType == PicVideo || isVideoExt(ext) {
// 		go func(vpath string, ent EntityType, fname string) {
// 			if thumb, err := generateVideoPoster(vpath, ent, fname); err != nil {
// 				if LogFunc != nil {
// 					LogFunc(fmt.Sprintf("warning: video poster generation failed for %s: %v", fname, err), 0, "")
// 				}
// 			} else {
// 				if LogFunc != nil {
// 					LogFunc(thumb, 0, "image/jpeg")
// 				}
// 			}
// 		}(fullPath, entity, filename)
// 	}

// 	// Generic logging
// 	if LogFunc != nil {
// 		LogFunc(filename, 0, "")
// 	}
// 	return filename, nil
// }

// // SaveFormFile saves a single form file (convenience)
// func SaveFormFile(r *multipart.Form, formKey string, entity EntityType, picType PictureType, required bool) (string, error) {
// 	files := r.File[formKey]
// 	if len(files) == 0 {
// 		if required {
// 			return "", fmt.Errorf("missing required file: %s", formKey)
// 		}
// 		return "", nil
// 	}
// 	file, err := files[0].Open()
// 	if err != nil {
// 		return "", fmt.Errorf("open %s: %w", formKey, err)
// 	}
// 	// SaveFileForEntity will close the file
// 	return SaveFileForEntity(file, files[0], entity, picType)
// }

// // SaveFormFiles saves multiple files under one form key. It returns saved filenames and aggregated error (if any).
// func SaveFormFiles(form *multipart.Form, formKey string, entity EntityType, picType PictureType, required bool) ([]string, error) {
// 	files := form.File[formKey]
// 	if len(files) == 0 {
// 		if required {
// 			return nil, fmt.Errorf("missing required files: %s", formKey)
// 		}
// 		return nil, nil
// 	}
// 	var saved []string
// 	var errs []string
// 	for _, hdr := range files {
// 		file, err := hdr.Open()
// 		if err != nil {
// 			errs = append(errs, fmt.Sprintf("open %s: %v", hdr.Filename, err))
// 			continue
// 		}
// 		name, err := SaveFileForEntity(file, hdr, entity, picType)
// 		if err != nil {
// 			errs = append(errs, fmt.Sprintf("save %s: %v", hdr.Filename, err))
// 			continue
// 		}
// 		saved = append(saved, name)
// 	}
// 	if len(errs) > 0 {
// 		return saved, fmt.Errorf("errors saving files: %s", strings.Join(errs, "; "))
// 	}
// 	return saved, nil
// }

// // SaveImageWithThumb saves an image, validates dimensions and creates a thumbnail; returns image name and thumbnail name (if created).
// func SaveImageWithThumb(file multipart.File, header *multipart.FileHeader, entity EntityType, picType PictureType, thumbWidth int, userid string) (string, string, error) {
// 	defer file.Close()

// 	origPath := ResolvePath(entity, picType)
// 	origName, err := SaveFile(file, header, origPath, 10<<20, nil)
// 	if err != nil {
// 		return "", "", fmt.Errorf("save original: %w", err)
// 	}

// 	fullPath := filepath.Join(origPath, origName)

// 	f, err := os.Open(fullPath)
// 	if err != nil {
// 		return origName, "", fmt.Errorf("open for decode: %w", err)
// 	}
// 	img, _, err := image.Decode(f)
// 	_ = f.Close()
// 	if err != nil {
// 		return origName, "", fmt.Errorf("decode %q: %w", header.Filename, err)
// 	}

// 	ext := strings.ToLower(filepath.Ext(fullPath))
// 	// Re-encode as PNG if not PNG
// 	if ext != ".png" {
// 		pngPath := strings.TrimSuffix(fullPath, ext) + ".png"
// 		out, err := os.Create(pngPath)
// 		if err != nil {
// 			return origName, "", fmt.Errorf("create png %s: %w", pngPath, err)
// 		}
// 		if err := png.Encode(out, img); err != nil {
// 			_ = out.Close()
// 			_ = os.Remove(pngPath)
// 			return origName, "", fmt.Errorf("encode png: %w", err)
// 		}
// 		_ = out.Close()
// 		_ = os.Remove(fullPath)
// 		fullPath = pngPath
// 		origName = filepath.Base(pngPath)
// 	}

// 	if err := ValidateImageDimensions(img, 3000, 3000); err != nil {
// 		return origName, "", fmt.Errorf("invalid image %q: %w", header.Filename, err)
// 	}

// 	// Notify MQ (best-effort)
// 	go func(p, ent, name, pt, uid string) {
// 		_ = mq.NotifyImageSaved(p, ent, name, pt, uid)
// 	}(fullPath, string(entity), origName, string(picType), userid)

// 	// Thumbnail → create with unique name to avoid collisions
// 	if img.Bounds().Dx() > thumbWidth || img.Bounds().Dy() > thumbWidth {
// 		thumbImg := imaging.Resize(img, thumbWidth, 0, imaging.Lanczos)
// 		thumbName := userid + ".jpg"
// 		thumbDir := ResolvePath(entity, PicThumb)

// 		if err := os.MkdirAll(thumbDir, 0o755); err != nil {
// 			return origName, "", fmt.Errorf("mkdir %q: %w", thumbDir, err)
// 		}
// 		thumbPath := filepath.Join(thumbDir, thumbName)

// 		out, err := os.Create(thumbPath)
// 		if err != nil {
// 			return origName, "", fmt.Errorf("create thumb %q: %w", thumbPath, err)
// 		}
// 		if err := jpeg.Encode(out, thumbImg, &jpeg.Options{Quality: 85}); err != nil {
// 			_ = out.Close()
// 			_ = os.Remove(thumbPath)
// 			return origName, "", fmt.Errorf("encode thumb: %w", err)
// 		}
// 		_ = out.Close()

// 		if LogFunc != nil {
// 			LogFunc(thumbPath, 0, "image/jpeg")
// 		}
// 		return origName, thumbName, nil
// 	}

// 	if LogFunc != nil {
// 		LogFunc(origName, 0, "image/png")
// 	}
// 	return origName, "", nil
// }

// // generateThumbnail writes a .jpg thumbnail preserving aspect ratio.
// func generateThumbnail(img image.Image, entity EntityType, baseFilename string, thumbWidth int) error {
// 	resized := imaging.Resize(img, thumbWidth, 0, imaging.Lanczos)
// 	name := strings.TrimSuffix(baseFilename, filepath.Ext(baseFilename)) + ".jpg"
// 	path := filepath.Join(ResolvePath(entity, PicThumb), name)
// 	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
// 		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
// 	}
// 	out, err := os.Create(path)
// 	if err != nil {
// 		return fmt.Errorf("create thumbnail: %w", err)
// 	}
// 	if err := jpeg.Encode(out, resized, &jpeg.Options{Quality: 85}); err != nil {
// 		_ = out.Close()
// 		_ = os.Remove(path)
// 		return fmt.Errorf("encode thumbnail: %w", err)
// 	}
// 	_ = out.Close()
// 	if LogFunc != nil {
// 		LogFunc(path, 0, "image/jpeg")
// 	}
// 	return nil
// }

// // generateVideoPoster creates a JPEG poster for the video. It probes duration and chooses a safe timestamp.
// // Returns the poster base filename (not full path).
// func generateVideoPoster(videoPath string, entity EntityType, baseFilename string) (string, error) {
// 	thumbName := strings.TrimSuffix(baseFilename, filepath.Ext(baseFilename)) + ".jpg"
// 	thumbDir := ResolvePath(entity, PicThumb)
// 	thumbPath := filepath.Join(thumbDir, thumbName)

// 	if err := os.MkdirAll(thumbDir, 0o755); err != nil {
// 		return "", fmt.Errorf("mkdir %s: %w", thumbDir, err)
// 	}

// 	// Probe duration with ffprobe (best-effort)
// 	var ts float64 = 0.5 // default timestamp in seconds
// 	cmdProbe := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", videoPath)
// 	if out, err := cmdProbe.Output(); err == nil {
// 		s := strings.TrimSpace(string(out))
// 		if s != "" {
// 			if d, err := strconv.ParseFloat(s, 64); err == nil && d > 0 {
// 				// pick middle frame if video reasonably long, otherwise pick 0
// 				if d >= 2.0 {
// 					ts = d / 2.0
// 				} else if d >= 0.5 {
// 					ts = d / 2.0
// 				} else {
// 					ts = 0.0
// 				}
// 			}
// 		}
// 	}

// 	// use "%.3f" formatting so ffmpeg accepts decimal seconds
// 	ss := fmt.Sprintf("%.3f", ts)
// 	cmd := exec.Command(
// 		"ffmpeg",
// 		"-y",
// 		"-i", videoPath,
// 		"-ss", ss,
// 		"-vframes", "1",
// 		thumbPath,
// 	)
// 	if err := cmd.Run(); err != nil {
// 		// try fallback: grab first frame (ts = 0)
// 		fallback := exec.Command("ffmpeg", "-y", "-i", videoPath, "-ss", "0", "-vframes", "1", thumbPath)
// 		if ferr := fallback.Run(); ferr != nil {
// 			return "", fmt.Errorf("ffmpeg poster generation failed (primary: %v, fallback: %v)", err, ferr)
// 		}
// 	}

// 	if LogFunc != nil {
// 		LogFunc(thumbPath, 0, "image/jpeg")
// 	}
// 	return thumbName, nil
// }

// // generateUniqueID returns a monotonic unique string using unixnano timestamp.
// func generateUniqueID() string {
// 	return fmt.Sprintf("%d", time.Now().UnixNano())
// }

// // isVideoExt checks common video extensions
// func isVideoExt(ext string) bool {
// 	switch strings.ToLower(ext) {
// 	case ".mp4", ".mov", ".mkv", ".webm", ".avi", ".flv", ".m4v":
// 		return true
// 	default:
// 		return false
// 	}
// }

// // SaveFormFilesByKeys saves files for multiple form keys and returns aggregated list.
// func SaveFormFilesByKeys(form *multipart.Form, keys []string, entityType EntityType, pictureType PictureType, required bool) ([]string, error) {
// 	var urls []string
// 	var errs []string
// 	for _, key := range keys {
// 		partial, err := SaveFormFiles(form, key, entityType, pictureType, required)
// 		if err != nil {
// 			// collect errors but continue to gather others
// 			errs = append(errs, fmt.Sprintf("%s: %v", key, err))
// 		}
// 		urls = append(urls, partial...)
// 	}
// 	if len(errs) > 0 {
// 		return urls, fmt.Errorf("errors: %s", strings.Join(errs, "; "))
// 	}
// 	return urls, nil
// }
