package feed

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"naevis/db"
	"naevis/filemgr"
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

func editExistingPost(ctx context.Context, claims *middleware.Claims, payload PostPayload) (models.FeedPost, error) {
	var post models.FeedPost
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
		update["tags"] = payload.Tags
	}

	if len(update) == 0 {
		return post, errors.New("nothing to update")
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	res := db.PostsCollection.FindOneAndUpdate(ctx, bson.M{"postid": payload.PostID, "userid": claims.UserID}, bson.M{"$set": update}, opts)
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

func CreateOrEditPost(ctx context.Context, claims *middleware.Claims, payload PostPayload, r *http.Request, action PostAction) (models.FeedPost, error) {
	var post models.FeedPost
	var err error

	entitytype := filemgr.EntityFeed

	payload, err = preparePostPayload(payload)
	if err != nil {
		return post, err
	}

	switch action {
	case ActionCreate:
		paths, names, resolutions, err := HandleMediaUpload(r, payload.Type, entitytype)
		if err != nil {
			return post, err
		}
		if payload.Type != "text" && len(paths) == 0 && len(names) == 0 {
			return post, errors.New("no media uploaded")
		}
		return insertNewPost(ctx, claims, payload, paths, names, resolutions)

	case ActionEdit:
		return editExistingPost(ctx, claims, payload)
	}

	return post, errors.New("unsupported action")
}

func insertNewPost(ctx context.Context, claims *middleware.Claims, payload PostPayload, paths, names []string, resolutions []int) (models.FeedPost, error) {
	post := models.FeedPost{
		PostID:      utils.GenerateRandomString(12),
		Username:    claims.Username,
		UserID:      claims.UserID,
		Text:        payload.Text,
		Title:       payload.Title,
		Description: payload.Description,
		Tags:        payload.Tags,
		Timestamp:   time.Now().Format(time.RFC3339),
		Likes:       0,
		Type:        payload.Type,
		Subtitles:   make(map[string]string),
		Resolutions: resolutions,
		MediaURL:    names,
		Media:       paths,
	}

	if _, err := db.PostsCollection.InsertOne(ctx, post); err != nil {
		return post, err
	}

	userdata.SetUserData("feedpost", post.PostID, claims.UserID, "", "")
	go mq.EmitHashtagEvent("feedpost", post.PostID, post.Tags)
	go mq.Emit(ctx, "post-created", models.Index{
		EntityType: "feedpost",
		EntityId:   post.PostID,
		Method:     "POST",
	})

	return post, nil
}

func preparePostPayload(payload PostPayload) (PostPayload, error) {
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
		return payload, errors.New("invalid post type")
	}

	payload.Tags = sanitizeTags(payload.Tags)
	return payload, nil
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
