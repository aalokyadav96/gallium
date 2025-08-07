package posts

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"naevis/db"
	"naevis/filemgr"
	"naevis/globals"
	"naevis/models"
)

func GetPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	objID, err := primitive.ObjectIDFromHex(ps.ByName("id"))
	if err != nil {
		http.Error(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	var post models.Post
	if err := db.BlogPostsCollection.FindOne(context.TODO(), bson.M{"_id": objID}).Decode(&post); err != nil {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(post)
}

func GetAllPosts(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	cur, err := db.BlogPostsCollection.Find(context.TODO(), bson.M{})
	if err != nil {
		http.Error(w, "Failed to fetch posts", http.StatusInternalServerError)
		return
	}
	defer cur.Close(context.TODO())

	var posts []models.Post
	for cur.Next(context.TODO()) {
		var post models.Post
		if err := cur.Decode(&post); err == nil {
			post.Content = ""
			posts = append(posts, post)
		}
	}

	if len(posts) == 0 {
		posts = []models.Post{}
	}

	json.NewEncoder(w).Encode(posts)
}
func CreatePost(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
	if !ok {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	content := strings.TrimSpace(r.FormValue("content"))
	category := r.FormValue("category")
	subcategory := r.FormValue("subcategory")
	referenceID := strings.TrimSpace(r.FormValue("referenceId"))

	if title == "" || content == "" || category == "" || subcategory == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	var refPtr *string
	if category == "Review" && (subcategory == "Product" || subcategory == "Place" || subcategory == "Event") {
		if referenceID == "" {
			http.Error(w, "Reference ID required for this subcategory", http.StatusBadRequest)
			return
		}
		refPtr = &referenceID
	}

	var imagePaths []string
	for key, files := range r.MultipartForm.File {
		if !strings.HasPrefix(key, "images_") {
			continue
		}
		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				http.Error(w, "Image upload failed", http.StatusInternalServerError)
				return
			}
			path, err := filemgr.SaveFileForEntity(file, fileHeader, filemgr.EntityPost, filemgr.PicPhoto)
			if err != nil {
				http.Error(w, "Image upload failed", http.StatusInternalServerError)
				return
			}
			imagePaths = append(imagePaths, path)
		}
	}

	post := models.Post{
		ID:          primitive.NewObjectID(),
		Title:       title,
		Content:     content,
		Category:    category,
		Subcategory: subcategory,
		ImagePaths:  imagePaths,
		ReferenceID: refPtr,
		CreatedBy:   requestingUserID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	res, err := db.BlogPostsCollection.InsertOne(context.TODO(), post)
	if err != nil {
		http.Error(w, "Failed to create post", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"postid":  res.InsertedID.(primitive.ObjectID).Hex(),
	})
}

func UpdatePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	idStr := ps.ByName("id")
	objID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	update := bson.M{}
	if v := strings.TrimSpace(r.FormValue("title")); v != "" {
		update["title"] = v
	}
	if v := strings.TrimSpace(r.FormValue("content")); v != "" {
		update["content"] = v
	}
	if v := r.FormValue("category"); v != "" {
		update["category"] = v
	}
	if v := r.FormValue("subcategory"); v != "" {
		update["subcategory"] = v
	}

	var imagePaths []string
	for key, files := range r.MultipartForm.File {
		if !strings.HasPrefix(key, "images_") {
			continue
		}
		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				http.Error(w, "Image upload failed", http.StatusInternalServerError)
				return
			}
			path, err := filemgr.SaveFileForEntity(file, fileHeader, filemgr.EntityPost, filemgr.PicPhoto)
			if err != nil {
				http.Error(w, "Image upload failed", http.StatusInternalServerError)
				return
			}
			imagePaths = append(imagePaths, path)
		}
	}
	if len(imagePaths) > 0 {
		update["imagePaths"] = imagePaths
	}

	if len(update) == 0 {
		http.Error(w, "No fields to update", http.StatusBadRequest)
		return
	}

	update["updatedAt"] = time.Now()

	_, err = db.BlogPostsCollection.UpdateOne(
		context.TODO(),
		bson.M{"_id": objID},
		bson.M{"$set": update},
	)
	if err != nil {
		http.Error(w, "Failed to update post", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"postid":  idStr,
	})
}

// func CreatePost(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		http.Error(w, "Invalid form data", http.StatusBadRequest)
// 		return
// 	}

// 	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
// 	if !ok {
// 		http.Error(w, "Invalid user", http.StatusBadRequest)
// 		return
// 	}

// 	title := strings.TrimSpace(r.FormValue("title"))
// 	content := strings.TrimSpace(r.FormValue("content"))
// 	category := r.FormValue("category")
// 	subcategory := r.FormValue("subcategory")
// 	referenceID := strings.TrimSpace(r.FormValue("referenceId"))

// 	if title == "" || content == "" || category == "" || subcategory == "" {
// 		http.Error(w, "Missing required fields", http.StatusBadRequest)
// 		return
// 	}

// 	var refPtr *string
// 	if category == "Review" && (subcategory == "Product" || subcategory == "Place" || subcategory == "Event") {
// 		if referenceID == "" {
// 			http.Error(w, "Reference ID required for this subcategory", http.StatusBadRequest)
// 			return
// 		}
// 		refPtr = &referenceID
// 	}

// 	// ⬇️ Replaced saveUploadedImages
// 	files, err := filemgr.SaveFormFiles(r, "images", "./static/uploads", false)
// 	if err != nil {
// 		http.Error(w, "Image upload failed", http.StatusInternalServerError)
// 		return
// 	}
// 	var imagePaths []string
// 	for _, f := range files {
// 		imagePaths = append(imagePaths, "/uploads/"+f)
// 	}

// 	post := models.Post{
// 		ID:          primitive.NewObjectID(),
// 		Title:       title,
// 		Content:     content,
// 		Category:    category,
// 		Subcategory: subcategory,
// 		ImagePaths:  imagePaths,
// 		ReferenceID: refPtr,
// 		CreatedBy:   requestingUserID,
// 		CreatedAt:   time.Now(),
// 		UpdatedAt:   time.Now(),
// 	}

// 	res, err := db.BlogPostsCollection.InsertOne(context.TODO(), post)
// 	if err != nil {
// 		http.Error(w, "Failed to create post", http.StatusInternalServerError)
// 		return
// 	}

// 	json.NewEncoder(w).Encode(map[string]any{
// 		"success": true,
// 		"postid":  res.InsertedID.(primitive.ObjectID).Hex(),
// 	})
// }

// func UpdatePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	idStr := ps.ByName("id")
// 	objID, err := primitive.ObjectIDFromHex(idStr)
// 	if err != nil {
// 		http.Error(w, "Invalid post ID", http.StatusBadRequest)
// 		return
// 	}

// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		http.Error(w, "Invalid form data", http.StatusBadRequest)
// 		return
// 	}

// 	update := bson.M{}
// 	if v := strings.TrimSpace(r.FormValue("title")); v != "" {
// 		update["title"] = v
// 	}
// 	if v := strings.TrimSpace(r.FormValue("content")); v != "" {
// 		update["content"] = v
// 	}
// 	if v := r.FormValue("category"); v != "" {
// 		update["category"] = v
// 	}
// 	if v := r.FormValue("subcategory"); v != "" {
// 		update["subcategory"] = v
// 	}

// 	// ⬇️ Replaced saveUploadedImages
// 	files, err := filemgr.SaveFormFiles(r, "images", "./static/uploads", false)
// 	if err != nil {
// 		http.Error(w, "Image upload failed", http.StatusInternalServerError)
// 		return
// 	}
// 	if len(files) > 0 {
// 		var imagePaths []string
// 		for _, f := range files {
// 			imagePaths = append(imagePaths, "/uploads/"+f)
// 		}
// 		update["imagePaths"] = imagePaths
// 	}

// 	if len(update) == 0 {
// 		http.Error(w, "No fields to update", http.StatusBadRequest)
// 		return
// 	}

// 	update["updatedAt"] = time.Now()

// 	_, err = db.BlogPostsCollection.UpdateOne(
// 		context.TODO(),
// 		bson.M{"_id": objID},
// 		bson.M{"$set": update},
// 	)
// 	if err != nil {
// 		http.Error(w, "Failed to update post", http.StatusInternalServerError)
// 		return
// 	}

// 	json.NewEncoder(w).Encode(map[string]any{
// 		"success": true,
// 		"postid":  idStr,
// 	})
// }

// func CreatePost(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		http.Error(w, "Invalid form data", http.StatusBadRequest)
// 		return
// 	}

// 	requestingUserID, ok := r.Context().Value(globals.UserIDKey).(string)
// 	if !ok {
// 		http.Error(w, "Invalid user", http.StatusBadRequest)
// 		return
// 	}

// 	title := strings.TrimSpace(r.FormValue("title"))
// 	content := strings.TrimSpace(r.FormValue("content"))
// 	category := r.FormValue("category")
// 	subcategory := r.FormValue("subcategory")
// 	referenceID := strings.TrimSpace(r.FormValue("referenceId"))

// 	if title == "" || content == "" || category == "" || subcategory == "" {
// 		http.Error(w, "Missing required fields", http.StatusBadRequest)
// 		return
// 	}

// 	var refPtr *string
// 	if category == "Review" && (subcategory == "Product" || subcategory == "Place" || subcategory == "Event") {
// 		if referenceID == "" {
// 			http.Error(w, "Reference ID required for this subcategory", http.StatusBadRequest)
// 			return
// 		}
// 		refPtr = &referenceID
// 	}

// 	imagePaths, _ := saveUploadedImages(r)

// 	post := models.Post{
// 		ID:          primitive.NewObjectID(),
// 		Title:       title,
// 		Content:     content,
// 		Category:    category,
// 		Subcategory: subcategory,
// 		ImagePaths:  imagePaths,
// 		ReferenceID: refPtr,
// 		CreatedBy:   requestingUserID,
// 		CreatedAt:   time.Now(),
// 		UpdatedAt:   time.Now(),
// 	}

// 	res, err := db.BlogPostsCollection.InsertOne(context.TODO(), post)
// 	if err != nil {
// 		http.Error(w, "Failed to create post", http.StatusInternalServerError)
// 		return
// 	}

// 	json.NewEncoder(w).Encode(map[string]any{
// 		"success": true,
// 		"postid":  res.InsertedID.(primitive.ObjectID).Hex(),
// 	})
// }

// func UpdatePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	idStr := ps.ByName("id")
// 	objID, err := primitive.ObjectIDFromHex(idStr)
// 	if err != nil {
// 		http.Error(w, "Invalid post ID", http.StatusBadRequest)
// 		return
// 	}

// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		http.Error(w, "Invalid form data", http.StatusBadRequest)
// 		return
// 	}

// 	update := bson.M{}
// 	if v := strings.TrimSpace(r.FormValue("title")); v != "" {
// 		update["title"] = v
// 	}
// 	if v := strings.TrimSpace(r.FormValue("content")); v != "" {
// 		update["content"] = v
// 	}
// 	if v := r.FormValue("category"); v != "" {
// 		update["category"] = v
// 	}
// 	if v := r.FormValue("subcategory"); v != "" {
// 		update["subcategory"] = v
// 	}

// 	if imagePaths, _ := saveUploadedImages(r); len(imagePaths) > 0 {
// 		update["imagePaths"] = imagePaths
// 	}

// 	if len(update) == 0 {
// 		http.Error(w, "No fields to update", http.StatusBadRequest)
// 		return
// 	}

// 	update["updatedAt"] = time.Now()

// 	_, err = db.BlogPostsCollection.UpdateOne(
// 		context.TODO(),
// 		bson.M{"_id": objID},
// 		bson.M{"$set": update},
// 	)
// 	if err != nil {
// 		http.Error(w, "Failed to update post", http.StatusInternalServerError)
// 		return
// 	}

// 	json.NewEncoder(w).Encode(map[string]any{
// 		"success": true,
// 		"postid":  idStr,
// 	})
// }

// // helper
// func saveUploadedImages(r *http.Request) ([]string, error) {
// 	var imagePaths []string
// 	files := r.MultipartForm.File["images"]
// 	for _, fileHeader := range files {
// 		file, err := fileHeader.Open()
// 		if err != nil {
// 			continue
// 		}
// 		defer file.Close()

// 		os.MkdirAll("/static/uploads", os.ModePerm)
// 		// filename := primitive.NewObjectID().Hex() + "_" + fileHeader.Filename
// 		filename := utils.GenerateID(14) + fileHeader.Filename
// 		fullPath := filepath.Join("static", "uploads", filename)

// 		dst, err := os.Create(fullPath)
// 		if err != nil {
// 			continue
// 		}
// 		defer dst.Close()

// 		if _, err := io.Copy(dst, file); err == nil {
// 			imagePaths = append(imagePaths, "/uploads/"+filename)
// 		}
// 	}
// 	return imagePaths, nil
// }
