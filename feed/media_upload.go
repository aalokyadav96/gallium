package feed

import (
	"fmt"
	"mime/multipart"
	"naevis/filemgr"
	"naevis/models"
	"naevis/mq"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type MediaType string

const (
	Video MediaType = "video"
	Audio MediaType = "audio"
)

// processMediaUpload handles both video and audio uploads.
// - Streams upload to disk via filemgr.SaveFileForEntity (virus scan, validations there).
// - For video: probes dimensions, runs parallel transcode to multiple resolutions, creates poster.
// - For audio: delegates to processAudioResolutions (external).
// Returns:
//   - []int: list of available heights (e.g., [1080, 720, 480])
//   - []string: list of output paths (normalized with leading "/" and forward slashes)
//   - []string: list of uniqueIDs (one item per uploaded file in this function)
//   - error
func processMediaUpload(r *http.Request, formKey string, mediaType MediaType) ([]int, []string, []string, error) {
	file, err := getUploadedFile(r, formKey)
	if err != nil || file == nil {
		return nil, nil, nil, fmt.Errorf("no file uploaded: %w", err)
	}

	src, err := file.Open()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("cannot open uploaded file: %w", err)
	}
	defer src.Close()

	var picType filemgr.PictureType
	switch mediaType {
	case Video:
		picType = filemgr.PicVideo
	case Audio:
		picType = filemgr.PicAudio
	default:
		return nil, nil, nil, fmt.Errorf("unsupported media type: %s", mediaType)
	}

	// Save to secure location via filemgr (virus scan, validations there).
	savedName, err := filemgr.SaveFileForEntity(src, file, filemgr.EntityFeed, picType)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("file save failed: %w", err)
	}

	uploadDir := filemgr.ResolvePath(filemgr.EntityFeed, picType)
	savedPath := filepath.Join(uploadDir, savedName)
	uniqueID := strings.TrimSuffix(savedName, filepath.Ext(savedName))

	switch mediaType {
	case Video:
		// Probe
		width, height, err := getVideoDimensions(savedPath)
		if err != nil {
			_ = os.Remove(savedPath)
			return nil, nil, nil, fmt.Errorf("failed to get video dimensions: %w", err)
		}

		// Parallel transcodes
		resolutions, outputPaths := processVideoResolutionsParallel(savedPath, uploadDir, uniqueID, width, height, 3)
		if len(outputPaths) == 0 {
			// All failed → cleanup original
			_ = os.Remove(savedPath)
			return nil, nil, nil, fmt.Errorf("video transcoding failed for all resolutions")
		}

		// Poster
		posterDir := filepath.Join(filemgr.ResolvePath(filemgr.EntityFeed, filemgr.PicPoster), uniqueID)
		if err := CreatePoster(savedPath, posterDir); err != nil {
			// Poster failure is considered fatal for this flow → cleanup outputs and original
			for _, out := range outputPaths {
				_ = os.Remove(strings.TrimPrefix(filepath.FromSlash(out), string(filepath.Separator)))
			}
			_ = os.Remove(savedPath)
			return nil, nil, nil, fmt.Errorf("poster creation failed: %w", err)
		}

		// Fire-and-forget
		go createSubtitleFile(uniqueID)
		mq.Notify("postpics-uploaded", models.Index{})

		return resolutions, outputPaths, []string{uniqueID}, nil

	case Audio:
		// Keep audio processing as your external pipeline
		resolutions, outputPath := processAudioResolutions(savedPath, uploadDir, uniqueID)

		// Fire-and-forget
		go createSubtitleFile(uniqueID)
		mq.Notify("postaudio-uploaded", models.Index{})

		paths := []string{}
		if outputPath != "" {
			paths = []string{outputPath}
			// Normalize to / and forward slashes if not already
			if !strings.HasPrefix(paths[0], "/") {
				paths[0] = "/" + filepath.ToSlash(paths[0])
			}
		}
		return resolutions, paths, []string{uniqueID}, nil
	}
	return []int{}, []string{}, []string{}, nil
	// Should be unreachable due to mediaType switch
}

// getUploadedFile fetches the first file for formKey.
func getUploadedFile(r *http.Request, formKey string) (*multipart.FileHeader, error) {
	if r.MultipartForm == nil {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return nil, fmt.Errorf("failed to parse form: %w", err)
		}
	}
	files := r.MultipartForm.File[formKey]
	if len(files) == 0 {
		return nil, nil
	}
	return files[0], nil
}

func saveUploadedVideoFile(r *http.Request, formKey string) ([]int, []string, []string, error) {
	return processMediaUpload(r, formKey, Video)
}

func saveUploadedAudioFile(r *http.Request, formKey string) ([]int, []string, []string, error) {
	return processMediaUpload(r, formKey, Audio)
}

// processVideoResolutionsParallel transcodes the input video into multiple resolutions in parallel.
// Returns:
//   - availableResolutions: list of heights that were successfully produced (sorted descending)
//   - outputPaths: list of normalized output paths (leading "/" and forward slashes), sorted by height descending.
func processVideoResolutionsParallel(originalFilePath, uploadDir, uniqueID string, origWidth, origHeight int, maxParallel int) ([]int, []string) {
	if maxParallel <= 0 {
		maxParallel = 2
	}

	// Desired ladder
	ladder := []struct {
		Label  string
		Width  int
		Height int
	}{
		{"4320", 7680, 4320}, {"2160", 3840, 2160}, {"1440", 2560, 1440},
		{"1080", 1920, 1080}, {"720", 1280, 720}, {"480", 854, 480},
		{"360", 640, 360}, {"240", 426, 240}, {"144", 256, 144},
	}

	// Build tasks only for resolutions that are <= original dimensions
	type task struct {
		Label       string
		TargetW     int
		TargetH     int
		OutputPath  string
		ScaledParam string
	}
	var tasks []task
	for _, r := range ladder {
		newW, newH := fitResolution(origWidth, origHeight, r.Width, r.Height)
		// Skip upscales
		if newW > origWidth || newH > origHeight {
			continue
		}
		out := generateFilePath(uploadDir, uniqueID+"-"+r.Label, "mp4")
		tasks = append(tasks, task{
			Label:       r.Label,
			TargetW:     newW,
			TargetH:     newH,
			OutputPath:  out,
			ScaledParam: fmt.Sprintf("%dx%d", newW, newH),
		})
	}

	if len(tasks) == 0 {
		return nil, nil
	}

	// Worker pool
	type result struct {
		ok        bool
		height    int
		outputURL string
	}
	taskCh := make(chan task)
	resCh := make(chan result)
	doneCh := make(chan struct{})

	// Spawn workers
	workers := maxParallel
	if workers > len(tasks) {
		workers = len(tasks)
	}
	for i := 0; i < workers; i++ {
		go func() {
			for t := range taskCh {
				err := processVideoResolution(originalFilePath, t.OutputPath, t.ScaledParam)
				if err != nil {
					// Log and mark failed; caller handles aggregate
					fmt.Printf("Skipping %s due to error: %v\n", t.Label, err)
					resCh <- result{ok: false}
					continue
				}
				// Normalize output to web path style (leading "/" and forward slashes)
				urlPath := "/" + filepath.ToSlash(t.OutputPath)
				resCh <- result{
					ok:        true,
					height:    t.TargetH,
					outputURL: urlPath,
				}
			}
		}()
	}

	// Feed tasks
	go func() {
		for _, t := range tasks {
			taskCh <- t
		}
		close(taskCh)
	}()

	// Collect
	var heights []int
	var outputs []string
	count := 0
	for res := range resCh {
		count++
		if res.ok {
			heights = append(heights, res.height)
			outputs = append(outputs, res.outputURL)
		}
		if count == len(tasks) {
			close(doneCh)
			break
		}
	}
	<-doneCh
	close(resCh)

	// Sort by height desc, keep outputs aligned with heights
	type pair struct {
		h int
		p string
	}
	var pairs []pair
	for i := range heights {
		pairs = append(pairs, pair{h: heights[i], p: outputs[i]})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].h > pairs[j].h })
	heights = heights[:0]
	outputs = outputs[:0]
	for _, pr := range pairs {
		heights = append(heights, pr.h)
		outputs = append(outputs, pr.p)
	}

	return heights, outputs
}

func generateFilePath(baseDir, uniqueID, extension string) string {
	return filepath.Join(baseDir, fmt.Sprintf("%s.%s", uniqueID, extension))
}

// saveUploadedFiles retains your existing multi-image upload flow.
// (Unchanged except minor error messages; it delegates to filemgr.)
func saveUploadedFiles(r *http.Request, formKey, fileType string) ([]string, error) {
	files := r.MultipartForm.File[formKey]
	if len(files) == 0 {
		return nil, fmt.Errorf("no %s files uploaded", fileType)
	}

	var ids []string
	entity := filemgr.EntityFeed
	picType := filemgr.PictureType(fileType)

	for _, file := range files {
		origName, err := processSingleImageUpload(file, entity, picType)
		if err != nil {
			return nil, fmt.Errorf("failed to process %s: %w", fileType, err)
		}
		ids = append(ids, origName)
	}

	mq.Notify("postpics-uploaded", models.Index{})
	mq.Notify("thumbnail-created", models.Index{})

	return ids, nil
}

func processSingleImageUpload(file *multipart.FileHeader, entity filemgr.EntityType, picType filemgr.PictureType) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("cannot open image: %w", err)
	}
	defer src.Close()

	origName, err := filemgr.SaveFileForEntity(src, file, entity, picType)
	if err != nil {
		return "", fmt.Errorf("saving image failed: %w", err)
	}
	return origName, nil
}
