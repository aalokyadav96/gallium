package recipes

import (
	"context"
	"encoding/json"
	"naevis/db"
	"naevis/filemgr"
	"naevis/models"
	"naevis/mq"
	"naevis/utils"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// --- Helpers ---
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
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func getSafe(arr []string, index int) string {
	if index < len(arr) {
		return arr[index]
	}
	return ""
}

// Recipes
func GetRecipes(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	filter := bson.M{}
	if search := r.URL.Query().Get("search"); search != "" {
		filter["$or"] = []bson.M{
			utils.RegexFilter("title", search),
			utils.RegexFilter("description", search),
		}
	}
	if ing := r.URL.Query().Get("ingredient"); ing != "" {
		filter["ingredients.name"] = bson.M{"$regex": regexp.QuoteMeta(ing), "$options": "i"}
	}

	skip, limit := utils.ParsePagination(r, 10, 100)
	sort := utils.ParseSort(r.URL.Query().Get("sort"), bson.D{{Key: "createdAt", Value: -1}}, map[string]bson.D{
		"oldest":  {{Key: "createdAt", Value: 1}},
		"popular": {{Key: "views", Value: -1}},
	})

	opts := options.Find().SetSort(sort).SetSkip(skip).SetLimit(limit)
	recipes, err := utils.FindAndDecode[models.Recipe](ctx, db.RecipeCollection, filter, opts)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "Failed to fetch recipes")
		return
	}

	utils.JSON(w, http.StatusOK, recipes)
}

// // --- Handlers ---
// func GetRecipes(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
// 	defer cancel()

// 	filter := bson.M{}
// 	search := r.URL.Query().Get("search")
// 	ingredient := r.URL.Query().Get("ingredient")

// 	if search != "" {
// 		filter["$or"] = []bson.M{
// 			utils.RegexFilter("title", search),
// 			utils.RegexFilter("description", search),
// 		}
// 	}
// 	if ingredient != "" {
// 		filter["ingredients.name"] = bson.M{"$regex": regexp.QuoteMeta(ingredient), "$options": "i"}
// 	}

// 	skip, limit := utils.ParsePagination(r, 10, 100)
// 	sort := utils.ParseSort(r.URL.Query().Get("sort"),
// 		bson.D{{"createdAt", -1}},
// 		map[string]bson.D{
// 			"oldest":  {{"createdAt", 1}},
// 			"popular": {{"views", -1}},
// 		})

// 	opts := options.Find().SetSort(sort).SetSkip(skip).SetLimit(limit)
// 	recipes, err := utils.FindAndDecode[models.Recipe](ctx, db.RecipeCollection, filter, opts)
// 	if err != nil {
// 		utils.Error(w, http.StatusInternalServerError, "Failed to fetch recipes")
// 		return
// 	}

// 	utils.JSON(w, http.StatusOK, recipes)
// }

// // func GetRecipes(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// // 	ctx := context.TODO()
// // 	query := bson.M{}

// // 	search := r.URL.Query().Get("search")
// // 	ingredient := r.URL.Query().Get("ingredient")
// // 	sortParam := r.URL.Query().Get("sort")
// // 	offsetStr := r.URL.Query().Get("offset")
// // 	limitStr := r.URL.Query().Get("limit")

// // 	if search != "" {
// // 		query["$or"] = []bson.M{
// // 			{"title": bson.M{"$regex": search, "$options": "i"}},
// // 			{"description": bson.M{"$regex": search, "$options": "i"}},
// // 		}
// // 	}
// // 	if ingredient != "" {
// // 		query["ingredients.name"] = bson.M{"$regex": ingredient, "$options": "i"}
// // 	}

// // 	offset, _ := strconv.Atoi(offsetStr)
// // 	if offset < 0 {
// // 		offset = 0
// // 	}
// // 	limit, _ := strconv.Atoi(limitStr)
// // 	if limit <= 0 {
// // 		limit = 10
// // 	}

// // 	sort := bson.D{{Key: "createdAt", Value: -1}}
// // 	switch sortParam {
// // 	case "oldest":
// // 		sort = bson.D{{Key: "createdAt", Value: 1}}
// // 	case "popular":
// // 		sort = bson.D{{Key: "views", Value: -1}}
// // 	}

// // 	opts := options.Find().SetSort(sort).SetSkip(int64(offset)).SetLimit(int64(limit))
// // 	cursor, err := db.RecipeCollection.Find(ctx, query, opts)
// // 	if err != nil {
// // 		http.Error(w, err.Error(), http.StatusInternalServerError)
// // 		return
// // 	}
// // 	defer cursor.Close(ctx)

// // 	var recipes []models.Recipe
// // 	if err = cursor.All(ctx, &recipes); err != nil {
// // 		http.Error(w, err.Error(), http.StatusInternalServerError)
// // 		return
// // 	}

// // 	if recipes == nil {
// // 		recipes = []models.Recipe{}
// // 	}
// // 	json.NewEncoder(w).Encode(recipes)
// // }

func GetRecipe(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	var recipe models.Recipe
	err := db.RecipeCollection.FindOne(context.TODO(), bson.M{"recipeid": id}).Decode(&recipe)
	if err != nil {
		http.Error(w, "Recipe not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(recipe)
}

func CreateRecipe(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
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

	imagePaths, err := filemgr.SaveFormFiles(r.MultipartForm, "imageUrls", filemgr.EntityType("recipe"), filemgr.PicPhoto, false)
	if err != nil {
		http.Error(w, "Image upload failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	newID := utils.GenerateRandomString(12)
	recipe := models.Recipe{
		RecipeId:    newID,
		UserID:      userID,
		Title:       title,
		Description: description,
		PrepTime:    prepTime,
		Tags:        tags,
		ImageURLs:   imagePaths,
		Ingredients: ingredients,
		Steps:       steps,
		Difficulty:  difficulty,
		Servings:    servings,
		CreatedAt:   time.Now().Unix(),
		Views:       0,
	}

	_, err = db.RecipeCollection.InsertOne(context.TODO(), recipe)
	if err != nil {
		http.Error(w, "DB insert failed", http.StatusInternalServerError)
		return
	}

	go mq.Emit(ctx, "recipe-created", models.Index{
		EntityType: "recipe",
		EntityId:   recipe.RecipeId,
		Method:     "POST",
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(recipe)
}

func UpdateRecipe(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	id := ps.ByName("id")

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
	}

	// --- Ingredient parsing ---
	names := r.MultipartForm.Value["ingredientName[]"]
	itemIDs := r.MultipartForm.Value["ingredientItemId[]"]
	types := r.MultipartForm.Value["ingredientType[]"]
	quantities := r.MultipartForm.Value["ingredientQuantity[]"]
	units := r.MultipartForm.Value["ingredientUnit[]"]
	rawAlts := r.MultipartForm.Value["ingredientAlternatives[]"]

	if len(names) > 0 {
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
		updates["ingredients"] = ingredients
	}

	// --- Images ---
	files := r.MultipartForm.File["imageUrls"]
	if len(files) > 0 {
		imagePaths, err := filemgr.SaveFormFiles(r.MultipartForm, "imageUrls", filemgr.EntityType("recipe"), filemgr.PicPhoto, false)
		if err != nil {
			http.Error(w, "Image upload failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		updates["imageUrls"] = imagePaths
	}

	_, err := db.RecipeCollection.UpdateOne(
		context.TODO(),
		bson.M{"recipeid": id},
		bson.M{"$set": updates},
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go mq.Emit(ctx, "recipe-updated", models.Index{
		EntityType: "recipe",
		EntityId:   id,
		Method:     "PUT",
	})

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"updated"}`))
}

// func UpdateRecipe(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	ctx := r.Context()
// 	id := ps.ByName("id")

// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		http.Error(w, "Failed to parse form", http.StatusBadRequest)
// 		return
// 	}

// 	updates := bson.M{
// 		"title":       r.FormValue("title"),
// 		"description": r.FormValue("description"),
// 		"prepTime":    r.FormValue("prepTime"),
// 		"tags":        splitCSV(r.FormValue("tags")),
// 		"steps":       splitLines(r.FormValue("steps")),
// 	}

// 	files := r.MultipartForm.File["imageUrls"]
// 	if len(files) > 0 {
// 		imagePaths, err := filemgr.SaveFormFiles(r.MultipartForm, "imageUrls", filemgr.EntityType("recipe"), filemgr.PicPhoto, false)
// 		if err != nil {
// 			http.Error(w, "Image upload failed: "+err.Error(), http.StatusInternalServerError)
// 			return
// 		}
// 		updates["imageUrls"] = imagePaths
// 	}

// 	_, err := db.RecipeCollection.UpdateOne(
// 		context.TODO(),
// 		bson.M{"recipeid": id},
// 		bson.M{"$set": updates},
// 	)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	go mq.Emit(ctx, "recipe-updated", models.Index{
// 		EntityType: "recipe",
// 		EntityId:   id,
// 		Method:     "PUT",
// 	})

// 	w.Header().Set("Content-Type", "application/json")
// 	w.Write([]byte(`{"status":"updated"}`))
// }

func DeleteRecipe(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	id := ps.ByName("id")
	_, err := db.RecipeCollection.DeleteOne(context.TODO(), bson.M{"recipeid": id})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go mq.Emit(ctx, "recipe-deleted", models.Index{
		EntityType: "recipe",
		EntityId:   id,
		Method:     "DELETE",
	})

	w.Write([]byte(`{"status":"deleted"}`))
}

func GetRecipeTags(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := context.TODO()
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

// package recipes

// import (
// 	"context"
// 	"encoding/json"
// 	"naevis/db"
// 	"naevis/filemgr"
// 	"naevis/models"
// 	"naevis/mq"
// 	"naevis/utils"
// 	"net/http"
// 	"strconv"
// 	"strings"
// 	"time"

// 	"github.com/julienschmidt/httprouter"
// 	"go.mongodb.org/mongo-driver/bson"
// 	"go.mongodb.org/mongo-driver/mongo"
// 	"go.mongodb.org/mongo-driver/mongo/options"
// )

// // Get all recipes
// func GetRecipes(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx := context.TODO()
// 	query := bson.M{}

// 	// --- Parse query params ---
// 	search := r.URL.Query().Get("search")
// 	ingredient := r.URL.Query().Get("ingredient")
// 	sortParam := r.URL.Query().Get("sort")
// 	offsetStr := r.URL.Query().Get("offset")
// 	limitStr := r.URL.Query().Get("limit")

// 	// --- Search by title or description (case-insensitive) ---
// 	if search != "" {
// 		query["$or"] = []bson.M{
// 			{"title": bson.M{"$regex": search, "$options": "i"}},
// 			{"description": bson.M{"$regex": search, "$options": "i"}},
// 		}
// 	}

// 	// --- Filter by ingredient ---
// 	if ingredient != "" {
// 		query["ingredients.name"] = bson.M{"$regex": ingredient, "$options": "i"}
// 	}

// 	// --- Pagination ---
// 	offset, err := strconv.Atoi(offsetStr)
// 	if err != nil || offset < 0 {
// 		offset = 0
// 	}
// 	limit, err := strconv.Atoi(limitStr)
// 	if err != nil || limit <= 0 {
// 		limit = 10
// 	}

// 	// --- Sorting ---
// 	sort := bson.D{{Key: "createdAt", Value: -1}} // default: newest
// 	switch sortParam {
// 	case "oldest":
// 		sort = bson.D{{Key: "createdAt", Value: 1}}
// 	case "popular":
// 		sort = bson.D{{Key: "views", Value: -1}}
// 	}

// 	// --- Execute query ---
// 	opts := options.Find().
// 		SetSort(sort).
// 		SetSkip(int64(offset)).
// 		SetLimit(int64(limit))

// 	cursor, err := db.RecipeCollection.Find(ctx, query, opts)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(ctx)

// 	var recipes []models.Recipe
// 	if err = cursor.All(ctx, &recipes); err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	if len(recipes) == 0 {
// 		recipes = []models.Recipe{}
// 	}

// 	json.NewEncoder(w).Encode(recipes)
// }

// // func GetRecipes(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// // 	cursor, err := db.RecipeCollection.Find(context.TODO(), bson.M{})
// // 	if err != nil {
// // 		http.Error(w, err.Error(), http.StatusInternalServerError)
// // 		return
// // 	}
// // 	defer cursor.Close(context.TODO())

// // 	var recipes []models.Recipe
// // 	if err = cursor.All(context.TODO(), &recipes); err != nil {
// // 		http.Error(w, err.Error(), http.StatusInternalServerError)
// // 		return
// // 	}
// // 	json.NewEncoder(w).Encode(recipes)
// // }

// // Get one recipe
// func GetRecipe(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	id := ps.ByName("id")
// 	var recipe models.Recipe
// 	err := db.RecipeCollection.FindOne(context.TODO(), bson.M{"recipeid": id}).Decode(&recipe)
// 	if err != nil {
// 		http.Error(w, "Recipe not found", http.StatusNotFound)
// 		return
// 	}

// 	json.NewEncoder(w).Encode(recipe)
// }

// // Create
// func getSafe(arr []string, index int) string {
// 	if index < len(arr) {
// 		return arr[index]
// 	}
// 	return ""
// }
// func CreateRecipe(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx := r.Context()
// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		http.Error(w, "Failed to parse form", http.StatusBadRequest)
// 		return
// 	}

// 	userID := utils.GetUserIDFromRequest(r)
// 	title := r.FormValue("title")
// 	description := r.FormValue("description")
// 	prepTime := r.FormValue("prepTime")
// 	tags := splitCSV(r.FormValue("tags"))
// 	steps := splitLines(r.FormValue("steps"))
// 	difficulty := r.FormValue("difficulty")

// 	var servings int
// 	if val := r.FormValue("servings"); val != "" {
// 		if parsed, err := strconv.Atoi(val); err == nil {
// 			servings = parsed
// 		}
// 	}

// 	names := r.MultipartForm.Value["ingredientName[]"]
// 	itemIDs := r.MultipartForm.Value["ingredientItemId[]"]
// 	types := r.MultipartForm.Value["ingredientType[]"]
// 	quantities := r.MultipartForm.Value["ingredientQuantity[]"]
// 	units := r.MultipartForm.Value["ingredientUnit[]"]
// 	rawAlts := r.MultipartForm.Value["ingredientAlternatives[]"]

// 	var ingredients []models.Ingredient
// 	for i := range names {
// 		if i >= len(quantities) || i >= len(units) || names[i] == "" {
// 			continue
// 		}
// 		qty, err := strconv.ParseFloat(quantities[i], 64)
// 		if err != nil {
// 			continue
// 		}
// 		ingredient := models.Ingredient{
// 			Name:     names[i],
// 			ItemID:   getSafe(itemIDs, i),
// 			Type:     getSafe(types, i),
// 			Quantity: qty,
// 			Unit:     units[i],
// 		}
// 		if i < len(rawAlts) && rawAlts[i] != "" {
// 			alts := strings.Split(rawAlts[i], ",")
// 			for _, alt := range alts {
// 				parts := strings.Split(alt, "|")
// 				if len(parts) >= 3 {
// 					ingredient.Alternatives = append(ingredient.Alternatives, models.IngredientAlternative{
// 						Name:   parts[0],
// 						ItemID: parts[1],
// 						Type:   parts[2],
// 					})
// 				}
// 			}
// 		}
// 		ingredients = append(ingredients, ingredient)
// 	}

// 	imagePaths, err := filemgr.SaveFormFiles(r.MultipartForm, "imageUrls", filemgr.EntityType("recipe"), filemgr.PicPhoto, false)
// 	if err != nil {
// 		http.Error(w, "Image upload failed: "+err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	recipe := models.Recipe{
// 		UserID:      userID,
// 		Title:       title,
// 		Description: description,
// 		PrepTime:    prepTime,
// 		Tags:        tags,
// 		ImageURLs:   imagePaths,
// 		Ingredients: ingredients,
// 		Steps:       steps,
// 		Difficulty:  difficulty,
// 		Servings:    servings,
// 		CreatedAt:   time.Now().Unix(),
// 		Views:       0,
// 	}

// 	result, err := db.RecipeCollection.InsertOne(context.TODO(), recipe)
// 	if err != nil {
// 		http.Error(w, "DB insert failed", http.StatusInternalServerError)
// 		return
// 	}

// 	m := models.Index{EntityType: "recipe", EntityId: result.InsertedID.(string), Method: "POST"}
// 	go mq.Emit(ctx, "recipe-created", m)

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(result)
// }
// func UpdateRecipe(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	ctx := r.Context()
// 	id := ps.ByName("id")

// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		http.Error(w, "Failed to parse form", http.StatusBadRequest)
// 		return
// 	}

// 	updates := bson.M{
// 		"title":       r.FormValue("title"),
// 		"description": r.FormValue("description"),
// 		"prepTime":    r.FormValue("prepTime"),
// 		"tags":        splitCSV(r.FormValue("tags")),
// 		"steps":       splitLines(r.FormValue("steps")),
// 	}

// 	files := r.MultipartForm.File["imageUrls"]
// 	if len(files) > 0 {
// 		imagePaths, err := filemgr.SaveFormFiles(r.MultipartForm, "imageUrls", filemgr.EntityType("recipe"), filemgr.PicPhoto, false)
// 		if err != nil {
// 			http.Error(w, "Image upload failed: "+err.Error(), http.StatusInternalServerError)
// 			return
// 		}
// 		updates["imageUrls"] = imagePaths
// 	}

// 	_, err := db.RecipeCollection.UpdateOne(
// 		context.TODO(),
// 		bson.M{"recipeid": id},
// 		bson.M{"$set": updates},
// 	)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	m := models.Index{EntityType: "recipe", EntityId: id, Method: "PUT"}
// 	go mq.Emit(ctx, "recipe-updated", m)

// 	w.Header().Set("Content-Type", "application/json")
// 	w.Write([]byte(`{"status":"updated"}`))
// }

// // Delete
// func DeleteRecipe(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	ctx := r.Context()
// 	id := ps.ByName("id")
// 	_, err := db.RecipeCollection.DeleteOne(context.TODO(), bson.M{"recipeid": id})
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	m := models.Index{EntityType: "recipe", EntityId: id, Method: "DELETE"}
// 	go mq.Emit(ctx, "recipe-deleted", m)

// 	w.Write([]byte(`{"status":"deleted"}`))
// }

// func GetRecipeTags(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx := context.TODO()

// 	// Use MongoDB aggregation to extract unique tags
// 	pipeline := mongo.Pipeline{
// 		{{Key: "$unwind", Value: "$tags"}},
// 		{{Key: "$group", Value: bson.M{
// 			"recipeid": nil,
// 			"tags":     bson.M{"$addToSet": "$tags"},
// 		}}},
// 	}

// 	cursor, err := db.RecipeCollection.Aggregate(ctx, pipeline)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// 	defer cursor.Close(ctx)

// 	var result []struct {
// 		Tags []string `bson:"tags"`
// 	}
// 	if err := cursor.All(ctx, &result); err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	if len(result) > 0 {
// 		json.NewEncoder(w).Encode(result[0].Tags)
// 	} else {
// 		json.NewEncoder(w).Encode([]string{})
// 	}
// }

// func splitCSV(s string) []string {
// 	if s == "" {
// 		return nil
// 	}
// 	parts := strings.Split(s, ",")
// 	for i := range parts {
// 		parts[i] = strings.TrimSpace(parts[i])
// 	}
// 	return parts
// }

// func splitLines(s string) []string {
// 	if s == "" {
// 		return nil
// 	}
// 	lines := strings.Split(s, "\n")
// 	var out []string
// 	for _, line := range lines {
// 		trimmed := strings.TrimSpace(line)
// 		if trimmed != "" {
// 			out = append(out, trimmed)
// 		}
// 	}
// 	return out
// }
