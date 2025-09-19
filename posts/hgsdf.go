package posts

import (
	"encoding/json"
	"fmt"
	"html"
	"naevis/db"
	"naevis/filemgr"
	"naevis/globals"
	"naevis/models"
	"naevis/mq"
	"naevis/utils"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func pickThumb(blocks []models.Block) string {
	for _, b := range blocks {
		if b.Type == "image" && b.URL != "" {
			return b.URL
		}
	}
	return ""
}

// image upload endpoint
func UploadImage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid form data")
		return
	}
	fileHeader := r.MultipartForm.File["image"]
	if len(fileHeader) == 0 {
		utils.RespondWithError(w, http.StatusBadRequest, "No image uploaded")
		return
	}
	file, err := fileHeader[0].Open()
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to open image")
		return
	}
	defer file.Close()

	path, err := filemgr.SaveFileForEntity(file, fileHeader[0], filemgr.EntityPost, filemgr.PicPhoto)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Image save failed")
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"url": path})
}

func CreateOrUpdatePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params, isEdit bool) {
	ctx := r.Context()
	userID, ok := ctx.Value(globals.UserIDKey).(string)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Invalid user")
		return
	}

	var postid string
	if isEdit {
		postid = ps.ByName("id")
		if postid == "" {
			utils.RespondWithError(w, http.StatusBadRequest, "Post ID required for update")
			return
		}
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid form data")
		return
	}

	title := html.EscapeString(r.FormValue("title"))
	category := r.FormValue("category")
	subcategory := r.FormValue("subcategory")
	referenceID := strings.TrimSpace(r.FormValue("referenceId"))
	blocksRaw := r.FormValue("blocks")

	if title == "" || category == "" || subcategory == "" || blocksRaw == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Missing required fields")
		return
	}

	var blocks []models.Block
	if err := json.Unmarshal([]byte(blocksRaw), &blocks); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid blocks JSON")
		return
	}

	var refPtr *string
	if category == "Review" && (subcategory == "Product" || subcategory == "Place" || subcategory == "Event") {
		if referenceID == "" {
			utils.RespondWithError(w, http.StatusBadRequest, "Reference ID required")
			return
		}
		refPtr = &referenceID
	}

	if isEdit {
		filter := bson.M{"postid": postid, "createdBy": userID}
		update := bson.M{
			"title":       title,
			"category":    category,
			"subcategory": subcategory,
			"referenceId": refPtr,
			"blocks":      blocks,
			"thumb":       pickThumb(blocks),
			"updatedAt":   time.Now(),
		}
		_, err := db.BlogPostsCollection.UpdateOne(ctx, filter, bson.M{"$set": update})
		if err != nil {
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update post")
			return
		}
		go mq.Emit(ctx, "post-updated", models.Index{EntityType: "blogpost", EntityId: postid, Method: "PUT"})
		utils.RespondWithJSON(w, http.StatusOK, map[string]any{"postid": postid})
		return
	}

	newPost := models.BlogPost{
		PostID:      uuid.NewString(),
		Title:       title,
		Category:    category,
		Subcategory: subcategory,
		ReferenceID: refPtr,
		Blocks:      blocks,
		Thumb:       pickThumb(blocks),
		CreatedBy:   userID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	_, err := db.BlogPostsCollection.InsertOne(ctx, newPost)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create post")
		return
	}
	go mq.Emit(ctx, "post-created", models.Index{EntityType: "blogpost", EntityId: newPost.PostID, Method: "POST"})
	utils.RespondWithJSON(w, http.StatusOK, map[string]any{"postid": newPost.PostID})
}

// --- Thin wrappers ---
func CreatePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	CreateOrUpdatePost(w, r, ps, false)
}
func UpdatePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	CreateOrUpdatePost(w, r, ps, true)
}

// --- Delete post ---
func DeletePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	userID, ok := ctx.Value(globals.UserIDKey).(string)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Invalid user")
		return
	}
	postid := ps.ByName("id")
	if postid == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Post ID required")
		return
	}
	res, err := db.BlogPostsCollection.DeleteOne(ctx, bson.M{"postid": postid, "createdBy": userID})
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to delete post")
		return
	}
	if res.DeletedCount == 0 {
		utils.RespondWithError(w, http.StatusNotFound, "Post not found or unauthorized")
		return
	}
	go mq.Emit(ctx, "post-deleted", models.Index{EntityType: "blogpost", EntityId: postid, Method: "DELETE"})
	utils.RespondWithJSON(w, http.StatusOK, map[string]any{"postid": postid, "deleted": true})
}

// --- Get single post ---
func GetPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	postid := ps.ByName("id")
	var post models.BlogPost
	if err := db.BlogPostsCollection.FindOne(ctx, bson.M{"postid": postid}).Decode(&post); err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "Post not found")
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]any{"post": post})
}

// --- Get all posts ---
func GetAllPosts(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	query := r.URL.Query()
	limit := 20
	page := 1
	if l := query.Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
		if limit > 100 {
			limit = 100
		}
	}
	if p := query.Get("page"); p != "" {
		fmt.Sscanf(p, "%d", &page)
		if page < 1 {
			page = 1
		}
	}
	skip := (page - 1) * limit

	findOpts := options.Find().SetLimit(int64(limit)).SetSkip(int64(skip)).SetSort(bson.M{"createdAt": -1})
	cur, err := db.BlogPostsCollection.Find(ctx, bson.M{}, findOpts)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch posts")
		return
	}
	defer cur.Close(ctx)

	var posts []models.BlogPostResponse
	for cur.Next(ctx) {
		var p models.BlogPost
		if err := cur.Decode(&p); err != nil {
			continue
		}
		posts = append(posts, models.BlogPostResponse{
			PostID:      p.PostID,
			Title:       p.Title,
			Category:    p.Category,
			Subcategory: p.Subcategory,
			ReferenceID: p.ReferenceID,
			Thumb:       pickThumb(p.Blocks),
			CreatedBy:   p.CreatedBy,
			CreatedAt:   p.CreatedAt,
			UpdatedAt:   p.UpdatedAt,
		})
	}
	utils.RespondWithJSON(w, http.StatusOK, map[string]any{"posts": posts})
}
