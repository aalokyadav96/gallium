package filemgr

import "errors"

type EntityType string
type PictureType string

const (
	EntityArtist  EntityType = "artist"
	EntityUser    EntityType = "user"
	EntityBaito   EntityType = "baito"
	EntitySong    EntityType = "song"
	EntityPost    EntityType = "post"
	EntityChat    EntityType = "chat"
	EntityEvent   EntityType = "event"
	EntityFarm    EntityType = "farm"
	EntityCrop    EntityType = "crop"
	EntityPlace   EntityType = "place"
	EntityMedia   EntityType = "media"
	EntityFeed    EntityType = "feed"
	EntityProduct EntityType = "product"

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
