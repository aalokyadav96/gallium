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
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// --- Helpers ---
func splitCSV(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func splitLines(s string) []string {
	if s == "" {
		return []string{}
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

// normalizeRecipeSlices ensures that all slices in a recipe are non-nil
func normalizeRecipeSlices(r *models.Recipe) {
	if r.Dietary == nil {
		r.Dietary = []string{}
	}
	if r.Tags == nil {
		r.Tags = []string{}
	}
	if r.Steps == nil {
		r.Steps = []string{}
	}
	if r.Images == nil {
		r.Images = []string{}
	}
	if r.Ingredients == nil {
		r.Ingredients = []models.Ingredient{}
	}
	// Normalize alternatives in ingredients
	for i := range r.Ingredients {
		if r.Ingredients[i].Alternatives == nil {
			r.Ingredients[i].Alternatives = []models.IngredientAlternative{}
		}
	}
}

// --- Handlers ---

func GetRecipe(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	var recipe models.Recipe
	err := db.RecipeCollection.FindOne(context.TODO(), bson.M{"recipeid": id}).Decode(&recipe)
	if err != nil {
		http.Error(w, "Recipe not found", http.StatusNotFound)
		return
	}

	normalizeRecipeSlices(&recipe)

	w.Header().Set("Content-Type", "application/json")
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
	cookTime := r.FormValue("cookTime")
	cuisine := r.FormValue("cuisine")
	dietary := splitCSV(r.FormValue("dietary"))
	portionSize := r.FormValue("portionSize")
	season := r.FormValue("season")
	tags := splitCSV(r.FormValue("tags"))
	steps := splitLines(r.FormValue("steps"))
	difficulty := r.FormValue("difficulty")
	videoUrl := r.FormValue("videoUrl")
	notes := r.FormValue("notes")

	var servings int
	if val := r.FormValue("servings"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			servings = parsed
		}
	}

	// --- Ingredients ---
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
			Name:         names[i],
			ItemID:       getSafe(itemIDs, i),
			Type:         getSafe(types, i),
			Quantity:     qty,
			Unit:         units[i],
			Alternatives: []models.IngredientAlternative{},
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
		CookTime:    cookTime,
		Cuisine:     cuisine,
		Dietary:     dietary,
		PortionSize: portionSize,
		Season:      season,
		Tags:        tags,
		Steps:       steps,
		Ingredients: ingredients,
		Difficulty:  difficulty,
		Servings:    servings,
		VideoURL:    videoUrl,
		Notes:       notes,
		Images:      imagePaths,
		CreatedAt:   time.Now().Unix(),
		Views:       0,
	}

	normalizeRecipeSlices(&recipe)

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
		"cookTime":    r.FormValue("cookTime"),
		"cuisine":     r.FormValue("cuisine"),
		"dietary":     splitCSV(r.FormValue("dietary")),
		"portionSize": r.FormValue("portionSize"),
		"season":      r.FormValue("season"),
		"tags":        splitCSV(r.FormValue("tags")),
		"steps":       splitLines(r.FormValue("steps")),
		"difficulty":  r.FormValue("difficulty"),
		"servings":    func() int { v, _ := strconv.Atoi(r.FormValue("servings")); return v }(),
		"videoUrl":    r.FormValue("videoUrl"),
		"notes":       r.FormValue("notes"),
	}

	// --- Ingredients ---
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
			Name:         names[i],
			ItemID:       getSafe(itemIDs, i),
			Type:         getSafe(types, i),
			Quantity:     qty,
			Unit:         units[i],
			Alternatives: []models.IngredientAlternative{},
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
