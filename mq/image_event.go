package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"naevis/globals"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type ImageEvent struct {
	LocalPath string `json:"localPath"`
	Entity    string `json:"entity"`
	FileName  string `json:"fileName"`
	PicType   string `json:"picType"`
	Userid    string `json:"userid"`
}

// Config cache so we don't keep re-reading env vars
var (
	publicBaseURL     string
	publicStripPrefix string
)

func init() {
	publicBaseURL = strings.TrimRight(os.Getenv("PUBLIC_BASE_URL"), "/")
	if publicBaseURL == "" {
		publicBaseURL = "http://localhost:4000"
	}
	publicStripPrefix = filepath.ToSlash(strings.TrimRight(os.Getenv("PUBLIC_STRIP_PREFIX"), "/"))
}

// ToPublicURL converts a local path into an accessible HTTP URL.
func ToPublicURL(p string) string {
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p
	}
	p = filepath.ToSlash(p)

	if publicStripPrefix != "" {
		p = strings.TrimPrefix(p, publicStripPrefix)
	}

	// Use `path.Clean` for URL-safe paths (not filepath.Clean)
	return publicBaseURL + path.Clean("/"+p)
}

// NewImageEvent builds an ImageEvent with normalized URL
func NewImageEvent(localPath, entity, fileName, picType, userid string) ImageEvent {
	return ImageEvent{
		LocalPath: ToPublicURL(localPath),
		Entity:    entity,
		FileName:  fileName,
		PicType:   picType,
		Userid:    userid,
	}
}

// NotifyImageSaved publishes an ImageEvent to Redis.
func NotifyImageSaved(localPath, entity, fileName, picType, userid string) error {
	event := NewImageEvent(localPath, entity, fileName, picType, userid)

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal image event: %w", err)
	}

	if err := globals.RedisClient.Publish(context.Background(), "getting-images", data).Err(); err != nil {
		return fmt.Errorf("publish to redis: %w", err)
	}

	log.Printf("[NotifyImageSaved] Published image event: %+v", event)
	return nil
}

// // mq/image_event.go
// package mq

// import (
// 	"context"
// 	"encoding/json"
// 	"log"
// 	"naevis/globals"
// 	"os"
// 	"path"
// 	"path/filepath"
// 	"strings"
// )

// // ImageEvent is unchanged
// type ImageEvent struct {
// 	LocalPath string `json:"localPath"`
// 	Entity    string `json:"entity"`
// 	FileName  string `json:"fileName"`
// 	PicType   string `json:"picType"`
// }

// func publicBaseURL() string {
// 	if v := os.Getenv("PUBLIC_BASE_URL"); v != "" {
// 		return strings.TrimRight(v, "/")
// 	}
// 	return "http://localhost:4000"
// }

// // toPublicURL normalizes local/FS paths to a proper HTTP URL
// func toPublicURL(p string) string {
// 	// Already a URL? pass through
// 	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
// 		return p
// 	}

// 	// Windows -> URL slashes
// 	p = filepath.ToSlash(p)

// 	// Optionally strip a known filesystem prefix (e.g., "C:/app/public")
// 	if strip := os.Getenv("PUBLIC_STRIP_PREFIX"); strip != "" {
// 		strip = filepath.ToSlash(strip)
// 		strip = strings.TrimRight(strip, "/")
// 		p = strings.TrimPrefix(p, strip)
// 	}

// 	// Ensure single leading slash and clean URL-style path (use path, not filepath)
// 	p = path.Clean("/" + p)

// 	return publicBaseURL() + p
// }

// // NotifyImageSaved publishes an event to Redis when an image is saved.
// func NotifyImageSaved(localPath, entity, fileName, picType string) {
// 	log.Printf("[NotifyImageSaved] START localPath=%q entity=%s fileName=%s picType=%s", localPath, entity, fileName, picType)

// 	// Always emit a clean URL
// 	urlpath := toPublicURL(localPath)

// 	content := ImageEvent{
// 		LocalPath: urlpath,
// 		Entity:    entity,
// 		FileName:  fileName,
// 		PicType:   picType,
// 	}

// 	data, err := json.Marshal(content)
// 	if err != nil {
// 		log.Printf("[NotifyImageSaved] Failed to marshal event content: %v", err)
// 		return
// 	}

// 	if err := globals.RedisClient.Publish(context.Background(), "getting-images", data).Err(); err != nil {
// 		log.Printf("[NotifyImageSaved] Failed to publish event to Redis: %v", err)
// 		return
// 	}

// 	log.Printf("[NotifyImageSaved] Published LocalPath URL: %s", urlpath)
// 	log.Printf("[NotifyImageSaved] Event published to channel 'getting-images'")
// 	log.Printf("[NotifyImageSaved] END")
// }

// // // mq/image_event.go
// // package mq

// // import (
// // 	"context"
// // 	"encoding/json"
// // 	"fmt"
// // 	"log"
// // 	"naevis/globals"
// // )

// // type ImageEvent struct {
// // 	LocalPath string `json:"localPath"`
// // 	Entity    string `json:"entity"`
// // 	FileName  string `json:"fileName"`
// // 	PicType   string `json:"picType"`
// // }

// // // NotifyImageSaved publishes an event to Redis when an image is saved.
// // func NotifyImageSaved(localPath, entity, fileName, picType string) {
// // 	log.Printf("[NotifyImageSaved] START localPath=%s entity=%s fileName=%s picType=%s", localPath, entity, fileName, picType)
// // 	urlpath := fmt.Sprintf("http://localhost:4000/%s", localPath)
// // 	content := ImageEvent{
// // 		LocalPath: urlpath,
// // 		Entity:    entity,
// // 		FileName:  fileName,
// // 		PicType:   picType,
// // 	}

// // 	data, err := json.Marshal(content)
// // 	if err != nil {
// // 		log.Printf("[NotifyImageSaved] Failed to marshal event content: %v", err)
// // 		return
// // 	}

// // 	if err := globals.RedisClient.Publish(context.Background(), "getting-images", data).Err(); err != nil {
// // 		log.Printf("[NotifyImageSaved] Failed to publish event to Redis: %v", err)
// // 		return
// // 	}

// // 	log.Printf("[NotifyImageSaved] Event published to channel 'getting-images'")
// // 	log.Printf("[NotifyImageSaved] END")
// // }
