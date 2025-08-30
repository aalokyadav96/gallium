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
	"time"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// --- Main CreateOrUpdatePost function ---
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

	if err := r.ParseMultipartForm(50 << 20); err != nil { // 50MB max
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid form data")
		return
	}

	title := html.EscapeString(r.FormValue("title"))
	category := r.FormValue("category")
	subcategory := r.FormValue("subcategory")
	referenceID := r.FormValue("referenceId")
	blocksRaw := r.FormValue("blocks")

	if title == "" || category == "" || subcategory == "" || blocksRaw == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Missing required fields")
		return
	}

	// Parse blocks JSON
	var blocks []models.Block
	if err := json.Unmarshal([]byte(blocksRaw), &blocks); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid blocks JSON")
		return
	}

	// --- Map uploaded files to image blocks ---
	files := r.MultipartForm.File["images[]"]
	imageIndex := 0
	for i := range blocks {
		if blocks[i].Type != "image" {
			continue
		}

		if imageIndex >= len(files) {
			continue // No uploaded file, keep existing URL or placeholder
		}

		fileHeader := files[imageIndex]
		imageIndex++

		file, err := fileHeader.Open()
		if err != nil {
			utils.RespondWithError(w, http.StatusInternalServerError, "Image upload failed")
			return
		}
		defer file.Close()

		path, err := filemgr.SaveFileForEntity(file, fileHeader, filemgr.EntityPost, filemgr.PicPhoto)
		if err != nil {
			utils.RespondWithError(w, http.StatusInternalServerError, "Image save failed")
			return
		}

		blocks[i].URL = path
		blocks[i].Placeholder = ""
	}

	// --- Handle reference ID for reviews ---
	var refPtr *string
	if category == "Review" && (subcategory == "Product" || subcategory == "Place" || subcategory == "Event") {
		if referenceID == "" {
			utils.RespondWithError(w, http.StatusBadRequest, "Reference ID required")
			return
		}
		refPtr = &referenceID
	}

	if isEdit {
		// --- Update existing post ---
		filter := bson.M{"postid": postid, "createdBy": userID}
		update := bson.M{
			"title":       title,
			"category":    category,
			"subcategory": subcategory,
			"referenceId": refPtr,
			"blocks":      blocks,
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

	// --- Create new post ---
	newPost := models.BlogPost{
		PostID:      uuid.NewString(),
		Title:       title,
		Category:    category,
		Subcategory: subcategory,
		ReferenceID: refPtr,
		Blocks:      blocks,
		Thumb:       blocks[0].URL,
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

// --- Thin wrapper for Create ---
func CreatePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	CreateOrUpdatePost(w, r, ps, false)
}

// --- Thin wrapper for Update ---
func UpdatePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	CreateOrUpdatePost(w, r, ps, true)
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

// --- Get all posts with pagination, returning BlogPostResponse with deterministic thumbnail ---
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

		// Deterministic thumbnail: first image block with non-empty URL
		thumb := ""
		for _, b := range p.Blocks {
			if b.Type == "image" && b.URL != "" {
				thumb = b.URL
				break
			}
		}

		posts = append(posts, models.BlogPostResponse{
			PostID:      p.PostID,
			Title:       p.Title,
			Category:    p.Category,
			Subcategory: p.Subcategory,
			ReferenceID: p.ReferenceID,
			Thumb:       thumb,
			CreatedBy:   p.CreatedBy,
			CreatedAt:   p.CreatedAt,
			UpdatedAt:   p.UpdatedAt,
		})
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{"posts": posts})
}

// // --- Get all posts with pagination ---
// func GetAllPosts(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx := r.Context()
// 	query := r.URL.Query()

// 	limit := 20
// 	page := 1
// 	if l := query.Get("limit"); l != "" {
// 		fmt.Sscanf(l, "%d", &limit)
// 		if limit > 100 {
// 			limit = 100
// 		}
// 	}
// 	if p := query.Get("page"); p != "" {
// 		fmt.Sscanf(p, "%d", &page)
// 		if page < 1 {
// 			page = 1
// 		}
// 	}
// 	skip := (page - 1) * limit

// 	findOpts := options.Find().SetLimit(int64(limit)).SetSkip(int64(skip)).SetSort(bson.M{"createdAt": -1})
// 	cur, err := db.BlogPostsCollection.Find(ctx, bson.M{}, findOpts)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch posts")
// 		return
// 	}
// 	defer cur.Close(ctx)

// 	var posts []models.BlogPostResponse
// 	for cur.Next(ctx) {
// 		var p models.BlogPost
// 		if err := cur.Decode(&p); err != nil {
// 			continue // optional: log decoding error
// 		}

// 		thumb := ""
// 		for _, b := range p.Blocks {
// 			if b.Type == "image" && b.URL != "" {
// 				thumb = b.URL
// 				break
// 			}
// 		}

// 		posts = append(posts, models.BlogPostResponse{
// 			PostID:      p.PostID,
// 			Title:       p.Title,
// 			Category:    p.Category,
// 			Subcategory: p.Subcategory,
// 			ReferenceID: p.ReferenceID,
// 			Thumb:       thumb,
// 			CreatedBy:   p.CreatedBy,
// 			CreatedAt:   p.CreatedAt,
// 			UpdatedAt:   p.UpdatedAt,
// 		})
// 	}

// 	utils.RespondWithJSON(w, http.StatusOK, map[string]any{"posts": posts})
// }

// package posts

// import (
// 	"encoding/json"
// 	"fmt"
// 	"html"
// 	"mime/multipart"
// 	"naevis/db"
// 	"naevis/filemgr"
// 	"naevis/globals"
// 	"naevis/models"
// 	"naevis/mq"
// 	"naevis/utils"
// 	"net/http"
// 	"time"

// 	"github.com/google/uuid"
// 	"github.com/julienschmidt/httprouter"
// 	"go.mongodb.org/mongo-driver/bson"
// 	"go.mongodb.org/mongo-driver/mongo/options"
// )

// // --- Main CreateOrUpdatePost function ---
// func CreateOrUpdatePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params, isEdit bool) {
// 	ctx := r.Context()
// 	userID, ok := ctx.Value(globals.UserIDKey).(string)
// 	if !ok {
// 		utils.RespondWithError(w, http.StatusUnauthorized, "Invalid user")
// 		return
// 	}

// 	var postid string
// 	if isEdit {
// 		postid = ps.ByName("id")
// 		if postid == "" {
// 			utils.RespondWithError(w, http.StatusBadRequest, "Post ID required for update")
// 			return
// 		}
// 	}

// 	if err := r.ParseMultipartForm(50 << 20); err != nil { // 50MB max
// 		utils.RespondWithError(w, http.StatusBadRequest, "Invalid form data")
// 		return
// 	}

// 	title := html.EscapeString(r.FormValue("title"))
// 	category := r.FormValue("category")
// 	subcategory := r.FormValue("subcategory")
// 	referenceID := r.FormValue("referenceId")
// 	blocksRaw := r.FormValue("blocks")

// 	if title == "" || category == "" || subcategory == "" || blocksRaw == "" {
// 		utils.RespondWithError(w, http.StatusBadRequest, "Missing required fields")
// 		return
// 	}

// 	// Parse blocks JSON
// 	var blocks []models.Block
// 	if err := json.Unmarshal([]byte(blocksRaw), &blocks); err != nil {
// 		utils.RespondWithError(w, http.StatusBadRequest, "Invalid blocks JSON")
// 		return
// 	}

// 	// Map uploaded files in order to image blocks
// 	files := r.MultipartForm.File["images[]"]
// 	imageIndex := 0
// 	for i := range blocks {
// 		if blocks[i].Type == "image" {
// 			var fileHeader *multipart.FileHeader
// 			if imageIndex < len(files) {
// 				fileHeader = files[imageIndex]
// 				imageIndex++
// 			}

// 			if fileHeader != nil {
// 				file, err := fileHeader.Open()
// 				if err != nil {
// 					utils.RespondWithError(w, http.StatusInternalServerError, "Image upload failed")
// 					return
// 				}

// 				path, err := filemgr.SaveFileForEntity(file, fileHeader, filemgr.EntityPost, filemgr.PicPhoto)
// 				file.Close()
// 				if err != nil {
// 					utils.RespondWithError(w, http.StatusInternalServerError, "Image save failed")
// 					return
// 				}

// 				blocks[i].URL = path
// 				blocks[i].Placeholder = ""
// 			}
// 		}
// 	}

// 	var refPtr *string
// 	if category == "Review" && (subcategory == "Product" || subcategory == "Place" || subcategory == "Event") {
// 		if referenceID == "" {
// 			utils.RespondWithError(w, http.StatusBadRequest, "Reference ID required")
// 			return
// 		}
// 		refPtr = &referenceID
// 	}

// 	if isEdit {
// 		// --- Update existing post ---
// 		filter := bson.M{"postid": postid, "createdBy": userID}
// 		update := bson.M{
// 			"title":       title,
// 			"category":    category,
// 			"subcategory": subcategory,
// 			"referenceId": refPtr,
// 			"blocks":      blocks,
// 			"updatedAt":   time.Now(),
// 		}

// 		_, err := db.BlogPostsCollection.UpdateOne(ctx, filter, bson.M{"$set": update})
// 		if err != nil {
// 			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update post")
// 			return
// 		}

// 		go mq.Emit(ctx, "post-updated", models.Index{EntityType: "blogpost", EntityId: postid, Method: "PUT"})
// 		utils.RespondWithJSON(w, 200, map[string]any{"postid": postid})
// 		return
// 	}

// 	// --- Create new post ---
// 	newPost := models.BlogPost{
// 		PostID:      uuid.NewString(),
// 		Title:       title,
// 		Category:    category,
// 		Subcategory: subcategory,
// 		ReferenceID: refPtr,
// 		Blocks:      blocks,
// 		CreatedBy:   userID,
// 		CreatedAt:   time.Now(),
// 		UpdatedAt:   time.Now(),
// 	}

// 	_, err := db.BlogPostsCollection.InsertOne(ctx, newPost)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create post")
// 		return
// 	}

// 	go mq.Emit(ctx, "post-created", models.Index{EntityType: "blogpost", EntityId: newPost.PostID, Method: "POST"})
// 	utils.RespondWithJSON(w, 200, map[string]any{"postid": newPost.PostID})
// }

// // --- Thin wrapper for Create ---
// func CreatePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	CreateOrUpdatePost(w, r, ps, false)
// }

// // --- Thin wrapper for Update ---
// func UpdatePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	CreateOrUpdatePost(w, r, ps, true)
// }

// // --- Get single post ---
// func GetPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	ctx := r.Context()
// 	postid := ps.ByName("id")

// 	var post models.BlogPost
// 	if err := db.BlogPostsCollection.FindOne(ctx, bson.M{"postid": postid}).Decode(&post); err != nil {
// 		utils.RespondWithError(w, http.StatusNotFound, "Post not found")
// 		return
// 	}

// 	utils.RespondWithJSON(w, 200, map[string]any{"post": post})
// }

// // --- Get all posts with pagination, returning BlogPostResponse ---
// func GetAllPosts(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx := r.Context()
// 	query := r.URL.Query()

// 	limit := 20
// 	page := 1
// 	if l := query.Get("limit"); l != "" {
// 		fmt.Sscanf(l, "%d", &limit)
// 		if limit > 100 {
// 			limit = 100
// 		}
// 	}
// 	if p := query.Get("page"); p != "" {
// 		fmt.Sscanf(p, "%d", &page)
// 		if page < 1 {
// 			page = 1
// 		}
// 	}
// 	skip := (page - 1) * limit

// 	findOpts := options.Find().SetLimit(int64(limit)).SetSkip(int64(skip)).SetSort(bson.M{"createdAt": -1})
// 	cur, err := db.BlogPostsCollection.Find(ctx, bson.M{}, findOpts)
// 	if err != nil {
// 		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to fetch posts")
// 		return
// 	}
// 	defer cur.Close(ctx)

// 	var posts []models.BlogPostResponse
// 	for cur.Next(ctx) {
// 		var p models.BlogPost
// 		if err := cur.Decode(&p); err != nil {
// 			continue
// 		}

// 		thumb := ""
// 		for _, b := range p.Blocks {
// 			if b.Type == "image" && b.URL != "" {
// 				thumb = b.URL
// 				break
// 			}
// 		}

// 		posts = append(posts, models.BlogPostResponse{
// 			PostID:      p.PostID,
// 			Title:       p.Title,
// 			Category:    p.Category,
// 			Subcategory: p.Subcategory,
// 			ReferenceID: p.ReferenceID,
// 			Thumb:       thumb,
// 			CreatedBy:   p.CreatedBy,
// 			CreatedAt:   p.CreatedAt,
// 			UpdatedAt:   p.UpdatedAt,
// 		})
// 	}

// 	utils.RespondWithJSON(w, http.StatusOK, map[string]any{"posts": posts})
// }
