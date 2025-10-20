package feed

import (
	"context"
	"errors"
	"log"
	"naevis/db"
	"naevis/middleware"
	"naevis/models"
	"naevis/mq"
	"naevis/userdata"
	"naevis/utils"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// PostAction defines if this is create or edit
type PostAction string

const (
	ActionCreate PostAction = "create"
	ActionEdit   PostAction = "edit"
)

// media reference from /filedrop
type MediaRef struct {
	Filename string `json:"filename"`
	Extn     string `json:"extn"`
}

// PostPayload is shared between create & edit
type PostPayload struct {
	PostID      string     `json:"postid,omitempty"`
	Type        string     `json:"type,omitempty"`
	Text        string     `json:"text,omitempty"`
	Title       string     `json:"title,omitempty"`
	Description string     `json:"description,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	Caption     string     `json:"caption,omitempty"`
	Images      []MediaRef `json:"images,omitempty"`
	Video       *MediaRef  `json:"video,omitempty"`
	Thumbnail   *MediaRef  `json:"thumbnail,omitempty"`
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
	res := db.PostsCollection.FindOneAndUpdate(ctx,
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

func CreateOrEditPost(ctx context.Context, claims *middleware.Claims, payload PostPayload, action PostAction) (models.FeedPost, error) {
	var post models.FeedPost

	payload, err := preparePostPayload(payload)
	if err != nil {
		return post, err
	}

	switch action {
	case ActionCreate:
		return insertNewPost(ctx, claims, payload)
	case ActionEdit:
		return editExistingPost(ctx, claims, payload)
	default:
		return post, errors.New("unsupported action")
	}
}

func insertNewPost(ctx context.Context, claims *middleware.Claims, payload PostPayload) (models.FeedPost, error) {
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
	}

	switch payload.Type {
	case "image":
		if len(payload.Images) == 0 {
			return post, errors.New("no images attached")
		}
		if len(payload.Images) > 6 {
			return post, errors.New("cannot attach more than 6 images")
		}
		for _, img := range payload.Images {
			post.MediaURL = append(post.MediaURL, img.Filename)
			// post.Media = append(post.Media, img.Key+"/"+img.Filename+img.Extn)
			post.Media = append(post.Media, img.Filename+img.Extn)
		}
		post.Caption = payload.Caption

	case "video":
		if payload.Video == nil {
			return post, errors.New("missing video file")
		}
		post.MediaURL = []string{payload.Video.Filename}
		// post.Media = []string{payload.Video.Key + "/" + payload.Video.Filename + payload.Video.Extn}
		post.Media = []string{payload.Video.Filename + payload.Video.Extn}
		if payload.Thumbnail != nil {
			// post.Thumbnail = payload.Thumbnail.Key + "/" + payload.Thumbnail.Filename + payload.Thumbnail.Extn
			post.Thumbnail = payload.Thumbnail.Filename + payload.Thumbnail.Extn
			post.Resolutions = []int{}
		}

	case "text":
		// text only

	default:
		return post, errors.New("unsupported post type")
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

	if len([]rune(payload.Text)) > 500 {
		return payload, errors.New("post text exceeds 500 characters")
	}

	validPostTypes := map[string]bool{
		"text": true, "image": true, "video": true,
	}
	if !validPostTypes[payload.Type] {
		return payload, errors.New("invalid post type")
	}

	if err := checkTextContent(payload.Text); err != nil {
		return payload, err
	}
	payload.Tags = sanitizeTags(payload.Tags)
	return payload, nil
}

func sanitizeTags(tags []string) []string {
	seen := make(map[string]bool)
	clean := make([]string, 0, len(tags))
	for _, tag := range tags {
		t := utils.SanitizeText(tag)
		if t != "" && !seen[t] {
			seen[t] = true
			clean = append(clean, t)
		}
	}
	return clean
}

func checkTextContent(text string) error {
	mentions := extractMentions(text)
	hashtags := extractHashtags(text)
	urls := extractURLs(text)

	banned := []string{"spamword1", "offensiveword", "bannedtopic"}
	lowered := strings.ToLower(text)
	for _, bad := range banned {
		if strings.Contains(lowered, bad) {
			return errors.New("post contains prohibited content")
		}
	}

	if len(mentions) > 0 {
		log.Printf("mentions found: %v", mentions)
	}
	if len(hashtags) > 0 {
		log.Printf("hashtags found: %v", hashtags)
	}
	if len(urls) > 0 {
		log.Printf("urls found: %v", urls)
	}
	return nil
}

// regex helpers
var (
	mentionRegex = regexp.MustCompile(`@([a-zA-Z0-9_]{1,15})`)
	hashtagRegex = regexp.MustCompile(`#(\w+)`)
	urlRegex     = regexp.MustCompile(`https?://[^\s]+`)
)

func extractMentions(text string) []string {
	matches := mentionRegex.FindAllStringSubmatch(text, -1)
	out := []string{}
	for _, m := range matches {
		if len(m) > 1 {
			out = append(out, m[1])
		}
	}
	return out
}

func extractHashtags(text string) []string {
	matches := hashtagRegex.FindAllStringSubmatch(text, -1)
	out := []string{}
	for _, m := range matches {
		if len(m) > 1 {
			out = append(out, m[1])
		}
	}
	return out
}

func extractURLs(text string) []string {
	return urlRegex.FindAllString(text, -1)
}
