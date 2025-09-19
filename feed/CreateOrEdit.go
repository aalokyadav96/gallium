package feed

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"naevis/db"
	"naevis/middleware"
	"naevis/models"
	"naevis/mq"
	"naevis/userdata"
	"naevis/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type PostAction string

const (
	ActionCreate PostAction = "create"
	ActionEdit   PostAction = "edit"
)

// shared request payload for both create & edit
type PostPayload struct {
	PostID      string   `json:"postid,omitempty"`
	Type        string   `json:"type,omitempty"`
	Text        string   `json:"text,omitempty"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// core reusable function
func CreateOrEditPost(ctx context.Context, claims *middleware.Claims, payload PostPayload, r *http.Request, action PostAction) (models.FeedPost, error) {
	var post models.FeedPost

	switch action {
	case ActionCreate:
		if payload.Type == "" {
			payload.Type = "text"
		}
		payload.Type = utils.SanitizeText(payload.Type)
		payload.Text = utils.SanitizeText(payload.Text)

		validPostTypes := map[string]bool{
			"text": true, "image": true, "video": true, "audio": true,
			"blog": true, "merchandise": true,
		}
		if !validPostTypes[payload.Type] {
			return post, errors.New("invalid post type")
		}

		newPost := models.FeedPost{
			PostID:      utils.GenerateRandomString(12),
			Username:    claims.Username,
			UserID:      claims.UserID,
			Text:        payload.Text,
			Title:       payload.Title,
			Description: payload.Description,
			Tags:        sanitizeTags(payload.Tags), // ✅ sanitize here
			Timestamp:   time.Now().Format(time.RFC3339),
			Likes:       0,
			Type:        payload.Type,
			Subtitles:   make(map[string]string),
			Resolutions: []int{},
			MediaURL:    []string{},
			Media:       []string{},
		}

		var (
			mediaPaths []string
			mediaNames []string
			mediaRes   []int
			err        error
		)

		switch payload.Type {
		case "image":
			mediaNames, err = saveUploadedFiles(r, "images", "photo")
		case "video":
			mediaRes, mediaPaths, mediaNames, err = saveUploadedVideoFile(r, "video")
		case "audio":
			mediaRes, mediaPaths, mediaNames, err = saveUploadedAudioFile(r, "audio")
		}
		if err != nil {
			return post, err
		}
		if payload.Type != "text" && len(mediaPaths) == 0 && len(mediaNames) == 0 {
			return post, errors.New("no media uploaded")
		}

		newPost.Resolutions = mediaRes
		newPost.MediaURL = mediaNames
		newPost.Media = mediaPaths

		if _, err := db.PostsCollection.InsertOne(ctx, newPost); err != nil {
			return post, err
		}

		// store mapping
		userdata.SetUserData("feedpost", newPost.PostID, claims.UserID, "", "")

		go mq.EmitHashtagEvent("feedpost", newPost.PostID, newPost.Tags)

		// notify async
		go mq.Emit(ctx, "post-created", models.Index{
			EntityType: "feedpost",
			EntityId:   newPost.PostID,
			Method:     "POST",
		})

		return newPost, nil

	case ActionEdit:
		if payload.PostID == "" {
			return post, errors.New("missing postid")
		}

		update := bson.M{}
		if payload.Text != "" {
			update["text"] = payload.Text
		}
		if payload.Title != "" {
			update["title"] = payload.Title
		}
		if payload.Description != "" {
			update["description"] = payload.Description
		}
		if len(payload.Tags) > 0 {
			update["tags"] = sanitizeTags(payload.Tags)
		}

		if len(update) == 0 {
			return post, errors.New("nothing to update")
		}

		opts := options.FindOneAndUpdate().SetReturnDocument(options.After) // ✅ get updated doc back
		res := db.PostsCollection.FindOneAndUpdate(
			ctx,
			bson.M{"postid": payload.PostID, "userid": claims.UserID},
			bson.M{"$set": update},
			opts,
		)
		if res.Err() != nil {
			return post, res.Err()
		}
		if err := res.Decode(&post); err != nil {
			log.Printf("decode updated post: %v", err)
		}

		go mq.Emit(ctx, "post-edited", models.Index{
			EntityType: "feedpost",
			EntityId:   payload.PostID,
			Method:     "PUT",
		})

		return post, nil
	}

	return post, errors.New("unsupported action")
}

// helper to sanitize tags
func sanitizeTags(tags []string) []string {
	seen := make(map[string]bool)
	clean := make([]string, 0, len(tags))

	for _, tag := range tags {
		t := utils.SanitizeText(tag) // clean dangerous input
		if t == "" {
			continue
		}
		if !seen[t] {
			clean = append(clean, t)
			seen[t] = true
		}
	}
	return clean
}
