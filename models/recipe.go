package models

import "go.mongodb.org/mongo-driver/bson/primitive"
type IngredientAlternative struct {
	Name   string `json:"name" bson:"name"`
	ItemID string `json:"itemId" bson:"itemId"`
	Type   string `json:"type" bson:"type"`
}

type Ingredient struct {
	Name         string                 `json:"name" bson:"name"`
	ItemID       string                 `json:"itemId" bson:"itemId"`
	Type         string                 `json:"type" bson:"type"`
	Quantity     float64                `json:"quantity" bson:"quantity"`
	Unit         string                 `json:"unit" bson:"unit"`
	Alternatives []IngredientAlternative `json:"alternatives" bson:"alternatives"`
}

type Recipe struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID      string             `json:"userId" bson:"userId"`
	Title       string             `json:"title" bson:"title"`
	Description string             `json:"description" bson:"description"`
	PrepTime    string             `json:"prepTime" bson:"prepTime"`
	Tags        []string           `json:"tags" bson:"tags"`
	ImageURLs   []string           `json:"imageUrls" bson:"imageUrls"`
	Ingredients []Ingredient       `json:"ingredients" bson:"ingredients"`
	Steps       []string           `json:"steps" bson:"steps"`
	Difficulty  string             `json:"difficulty" bson:"difficulty"`
	Servings    int                `json:"servings" bson:"servings"`
	CreatedAt   int64              `json:"createdAt" bson:"createdAt"`
	Views       int                `json:"views" bson:"views"`
}

//~ type Ingredient struct {
	//~ Name         string  `json:"name"`
	//~ ItemID       string  `json:"itemId"`
	//~ Type         string  `json:"type"`
	//~ Quantity     float64 `json:"quantity"`
	//~ Unit         string  `json:"unit"`
	//~ Alternatives []struct {
		//~ Name   string `json:"name"`
		//~ ItemID string `json:"itemId"`
		//~ Type   string `json:"type"`
	//~ } `json:"alternatives"`
//~ }

//~ type Recipe struct {
	//~ ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	//~ UserID      string             `json:"userId"`
	//~ Title       string             `json:"title"`
	//~ Description string             `json:"description"`
	//~ PrepTime    string             `json:"prepTime"`
	//~ Tags        []string           `json:"tags"`
	//~ ImageURLs   []string           `json:"imageUrls"`
	//~ Ingredients []Ingredient       `json:"ingredients"`
	//~ Steps       []string           `json:"steps"`
	//~ CreatedAt   int64              `json:"createdAt"`
	//~ Views       int                `json:"views"`
//~ }
