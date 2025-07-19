package recipes

import (
	"context"
	"encoding/json"
	"io"
	"naevis/db"
	"naevis/models"
	"naevis/utils"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Get all recipes
func GetRecipes(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := context.TODO()
	query := bson.M{}

	// --- Parse query params ---
	search := r.URL.Query().Get("search")
	ingredient := r.URL.Query().Get("ingredient")
	sortParam := r.URL.Query().Get("sort")
	offsetStr := r.URL.Query().Get("offset")
	limitStr := r.URL.Query().Get("limit")

	// --- Search by title or description (case-insensitive) ---
	if search != "" {
		query["$or"] = []bson.M{
			{"title": bson.M{"$regex": search, "$options": "i"}},
			{"description": bson.M{"$regex": search, "$options": "i"}},
		}
	}

	// --- Filter by ingredient ---
	if ingredient != "" {
		query["ingredients.name"] = bson.M{"$regex": ingredient, "$options": "i"}
	}

	// --- Pagination ---
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}

	// --- Sorting ---
	sort := bson.D{{Key: "createdAt", Value: -1}} // default: newest
	switch sortParam {
	case "oldest":
		sort = bson.D{{Key: "createdAt", Value: 1}}
	case "popular":
		sort = bson.D{{Key: "views", Value: -1}}
	}

	// --- Execute query ---
	opts := options.Find().
		SetSort(sort).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	cursor, err := db.RecipeCollection.Find(ctx, query, opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var recipes []models.Recipe
	if err = cursor.All(ctx, &recipes); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(recipes) == 0 {
		recipes = []models.Recipe{}
	}

	json.NewEncoder(w).Encode(recipes)
}

// func GetRecipes(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	cursor, err := db.RecipeCollection.Find(context.TODO(), bson.M{})
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(context.TODO())

// 	var recipes []models.Recipe
// 	if err = cursor.All(context.TODO(), &recipes); err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// 	json.NewEncoder(w).Encode(recipes)
// }

// Get one recipe
func GetRecipe(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, _ := primitive.ObjectIDFromHex(ps.ByName("id"))
	var recipe models.Recipe
	err := db.RecipeCollection.FindOne(context.TODO(), bson.M{"_id": id}).Decode(&recipe)
	if err != nil {
		http.Error(w, "Recipe not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(recipe)
}

// Create
func getSafe(arr []string, index int) string {
	if index < len(arr) {
		return arr[index]
	}
	return ""
}
func CreateRecipe(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	userID := utils.GetUserIDFromRequest(r)
	title := r.FormValue("title")
	description := r.FormValue("description")
	prepTime := r.FormValue("prepTime")
	tags := splitCSV(r.FormValue("tags"))
	steps := splitLines(r.FormValue("steps"))
	difficulty := r.FormValue("difficulty")

	var servings int
	if val := r.FormValue("servings"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			servings = parsed
		}
	}

	names := r.MultipartForm.Value["ingredientName[]"]
	itemIDs := r.MultipartForm.Value["ingredientItemId[]"]
	types := r.MultipartForm.Value["ingredientType[]"]
	quantities := r.MultipartForm.Value["ingredientQuantity[]"]
	units := r.MultipartForm.Value["ingredientUnit[]"]
	rawAlts := r.MultipartForm.Value["ingredientAlternatives[]"]

	var ingredients []models.Ingredient
	for i := range names {
		if i >= len(quantities) || i >= len(units) || names[i] == "" {
			continue
		}

		qty, err := strconv.ParseFloat(quantities[i], 64)
		if err != nil {
			continue
		}

		ingredient := models.Ingredient{
			Name:     names[i],
			ItemID:   getSafe(itemIDs, i),
			Type:     getSafe(types, i),
			Quantity: qty,
			Unit:     units[i],
		}

		if i < len(rawAlts) && rawAlts[i] != "" {
			alts := strings.Split(rawAlts[i], ",")
			for _, alt := range alts {
				parts := strings.Split(alt, "|")
				if len(parts) >= 3 {
					ingredient.Alternatives = append(ingredient.Alternatives, models.IngredientAlternative{
						Name:   parts[0],
						ItemID: parts[1],
						Type:   parts[2],
					})
				}
			}
		}

		ingredients = append(ingredients, ingredient)
	}

	var imageURLs []string
	uploadFolder := "./static/uploads"
	if r.MultipartForm.File != nil {
		files := r.MultipartForm.File["imageUrls"]
		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				http.Error(w, "Error reading file", http.StatusInternalServerError)
				return
			}
			defer file.Close()

			savedName, err := utils.SaveFile(file, fileHeader, uploadFolder)
			if err != nil {
				http.Error(w, "Error saving file", http.StatusInternalServerError)
				return
			}
			imageURLs = append(imageURLs, savedName)
		}
	}

	recipe := models.Recipe{
		UserID:      userID,
		Title:       title,
		Description: description,
		PrepTime:    prepTime,
		Tags:        tags,
		ImageURLs:   imageURLs,
		Ingredients: ingredients,
		Steps:       steps,
		Difficulty:  difficulty,
		Servings:    servings,
		CreatedAt:   time.Now().Unix(),
		Views:       0,
	}

	result, err := db.RecipeCollection.InsertOne(context.TODO(), recipe)
	if err != nil {
		http.Error(w, "DB insert failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// func CreateRecipe(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	// Parse multipart form (max 10MB)
// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		http.Error(w, "Failed to parse form", http.StatusBadRequest)
// 		return
// 	}

// 	// Parse simple fields
// 	userID := r.FormValue("userId")
// 	title := r.FormValue("title")
// 	description := r.FormValue("description")
// 	prepTime := r.FormValue("prepTime")
// 	tags := splitCSV(r.FormValue("tags"))
// 	steps := splitLines(r.FormValue("steps"))

// 	// Parse servings (optional int)
// 	var servings int
// 	if s := r.FormValue("servings"); s != "" {
// 		if val, err := strconv.Atoi(s); err == nil {
// 			servings = val
// 		}
// 	}

// 	difficulty := r.FormValue("difficulty")

// 	// Parse ingredients
// 	names := r.MultipartForm.Value["ingredientName[]"]
// 	quantities := r.MultipartForm.Value["ingredientQuantity[]"]
// 	units := r.MultipartForm.Value["ingredientUnit[]"]

// 	var ingredients []models.Ingredient
// 	for i := range names {
// 		if i < len(quantities) && i < len(units) {
// 			qty, err := strconv.ParseFloat(quantities[i], 64)
// 			if err != nil {
// 				continue
// 			}
// 			ingredients = append(ingredients, models.Ingredient{
// 				Name:     names[i],
// 				Quantity: qty,
// 				Unit:     units[i],
// 			})
// 		}
// 	}

// 	// Handle image uploads
// 	var imageURLs []string
// 	uploadFolder := "./static/uploads"
// 	files := r.MultipartForm.File["imageUrls"]

// 	for _, fileHeader := range files {
// 		file, err := fileHeader.Open()
// 		if err != nil {
// 			http.Error(w, "Error reading file", http.StatusInternalServerError)
// 			return
// 		}
// 		defer file.Close()

// 		savedName, err := utils.SaveFile(file, fileHeader, uploadFolder)
// 		if err != nil {
// 			http.Error(w, "Error saving file", http.StatusInternalServerError)
// 			return
// 		}
// 		imageURLs = append(imageURLs, savedName)
// 	}

// 	// Build Recipe object
// 	recipe := models.Recipe{
// 		UserID:      userID,
// 		Title:       title,
// 		Description: description,
// 		PrepTime:    prepTime,
// 		Tags:        tags,
// 		ImageURLs:   imageURLs,
// 		Ingredients: ingredients,
// 		Steps:       steps,
// 		CreatedAt:   time.Now().Unix(),
// 		Views:       0,
// 	}

// 	// Save to DB
// 	result, err := db.RecipeCollection.InsertOne(context.TODO(), recipe)
// 	if err != nil {
// 		http.Error(w, "DB insert failed", http.StatusInternalServerError)
// 		return
// 	}

// 	// Return the inserted ID
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(result)
// }

// // func CreateRecipe(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// // 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// // 		http.Error(w, "Failed to parse form", http.StatusBadRequest)
// // 		return
// // 	}

// // 	recipe := models.Recipe{
// // 		UserID:      r.FormValue("userId"),
// // 		Title:       r.FormValue("title"),
// // 		Description: r.FormValue("description"),
// // 		PrepTime:    r.FormValue("prepTime"),
// // 		Tags:        splitCSV(r.FormValue("tags")),
// // 		Steps:       splitLines(r.FormValue("steps")),
// // 		CreatedAt:   time.Now().Unix(),
// // 		Views:       0,
// // 	}

// // 	uploadFolder := "./static/uploads"
// // 	files := r.MultipartForm.File["imageUrls"]
// // 	for _, fileHeader := range files {
// // 		file, err := fileHeader.Open()
// // 		if err != nil {
// // 			http.Error(w, "Error reading file", http.StatusInternalServerError)
// // 			return
// // 		}
// // 		defer file.Close()

// // 		savedName, err := utils.SaveFile(file, fileHeader, uploadFolder)
// // 		if err != nil {
// // 			http.Error(w, "Error saving file", http.StatusInternalServerError)
// // 			return
// // 		}

// // 		recipe.ImageURLs = append(recipe.ImageURLs, savedName)
// // 	}

// // 	result, err := db.RecipeCollection.InsertOne(context.TODO(), recipe)
// // 	if err != nil {
// // 		http.Error(w, "DB insert failed", http.StatusInternalServerError)
// // 		return
// // 	}
// // 	json.NewEncoder(w).Encode(result)
// // }

// Update
func UpdateRecipe(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, _ := primitive.ObjectIDFromHex(ps.ByName("id"))

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	updates := bson.M{
		"title":       r.FormValue("title"),
		"description": r.FormValue("description"),
		"prepTime":    r.FormValue("prepTime"),
		"tags":        splitCSV(r.FormValue("tags")),
		"steps":       splitLines(r.FormValue("steps")),
		// add additional fields as needed
	}

	// Handle new image uploads
	files := r.MultipartForm.File["imageUrls"]
	var imagePaths []string
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, "Error reading file", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		filename := "uploads/" + fileHeader.Filename
		dst, err := os.Create(filename)
		if err != nil {
			http.Error(w, "Error saving file", http.StatusInternalServerError)
			return
		}
		defer dst.Close()
		if _, err := io.Copy(dst, file); err != nil {
			http.Error(w, "Error writing file", http.StatusInternalServerError)
			return
		}

		imagePaths = append(imagePaths, filename)
	}
	if len(imagePaths) > 0 {
		updates["imageUrls"] = imagePaths
	}

	_, err := db.RecipeCollection.UpdateOne(
		context.TODO(),
		bson.M{"_id": id},
		bson.M{"$set": updates},
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write([]byte(`{"status":"updated"}`))
}

// Delete
func DeleteRecipe(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, _ := primitive.ObjectIDFromHex(ps.ByName("id"))
	_, err := db.RecipeCollection.DeleteOne(context.TODO(), bson.M{"_id": id})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write([]byte(`{"status":"deleted"}`))
}

func GetRecipeTags(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := context.TODO()

	// Use MongoDB aggregation to extract unique tags
	pipeline := mongo.Pipeline{
		{{Key: "$unwind", Value: "$tags"}},
		{{Key: "$group", Value: bson.M{
			"_id":  nil,
			"tags": bson.M{"$addToSet": "$tags"},
		}}},
	}

	cursor, err := db.RecipeCollection.Aggregate(ctx, pipeline)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var result []struct {
		Tags []string `bson:"tags"`
	}
	if err := cursor.All(ctx, &result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(result) > 0 {
		json.NewEncoder(w).Encode(result[0].Tags)
	} else {
		json.NewEncoder(w).Encode([]string{})
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
