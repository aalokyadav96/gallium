package userdata

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"naevis/db"
	"naevis/middleware"
	"naevis/structs"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

var ValidEntityTypes = map[string]bool{
	"userhome":   true,
	"place":      true,
	"event":      true,
	"feedpost":   true,
	"media":      true,
	"ticket":     true,
	"merch":      true,
	"review":     true,
	"comment":    true,
	"like":       true,
	"favourite":  true,
	"booking":    true,
	"blogpost":   true,
	"collection": true,
}

func IsValidEntityType(entityType string) bool {
	return ValidEntityTypes[entityType]
}
func SetUserData(dataType, dataId, userId, itemType, itemId string) {
	fmt.Println("set dataType : ", dataType)
	fmt.Println("set dataId : ", dataId)
	fmt.Println("set userId : ", userId)
	AddUserData(dataType, dataId, userId, itemType, itemId)
}

func DelUserData(dataType string, dataId string, userId string) {
	fmt.Println("del dataType : ", dataType)
	fmt.Println("del dataId : ", dataId)
	fmt.Println("del userId : ", userId)
	RemUserData(dataType, dataId, userId)
}

func AddUserData(entityType, entityId, userId, itemType, itemId string) {
	var content structs.UserData
	content.EntityID = entityId
	content.EntityType = entityType
	content.ItemID = itemId
	content.ItemType = itemType
	content.UserID = userId
	content.CreatedAt = time.Now().Format(time.RFC3339)
	// Insert the content into MongoDB
	_, err := db.UserDataCollection.InsertOne(context.TODO(), content)
	if err != nil {
		log.Printf("Error inserting content: %v", err)
		return
	}
}

func RemUserData(entityType, entityId, userId string) {
	// Delete the content from MongoDB
	_, err := db.UserDataCollection.DeleteOne(context.TODO(), bson.M{"entity_id": entityId, "entity_type": entityType, "userid": userId})
	if err != nil {
		log.Printf("Error deleting content: %v", err)
		return
	}
}

func GetUserProfileData(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	username := ps.ByName("username")

	tokenString := r.Header.Get("Authorization")
	claims, err := middleware.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if username != claims.UserID {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	// Parse the query parameter for the entity type
	entityType := r.URL.Query().Get("entity_type")
	if entityType == "" {
		http.Error(w, "Entity type is required", http.StatusBadRequest)
		return
	}

	// Validate the entity type
	if !IsValidEntityType(entityType) {
		http.Error(w, "Invalid entity type", http.StatusBadRequest)
		return
	}

	// Fetch user data from MongoDB
	filter := bson.M{"entity_type": entityType, "userid": username}
	cursor, err := db.UserDataCollection.Find(context.TODO(), filter)
	if err != nil {
		http.Error(w, "Failed to fetch user data", http.StatusInternalServerError)
		log.Printf("Error fetching user data: %v", err)
		return
	}
	defer cursor.Close(context.TODO())

	var results []structs.UserData
	if err := cursor.All(context.TODO(), &results); err != nil {
		http.Error(w, "Failed to decode user data", http.StatusInternalServerError)
		log.Printf("Error decoding user data: %v", err)
		return
	}
	fmt.Println(results)
	if len(results) == 0 {
		results = []structs.UserData{}
	}

	// Respond with the results
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		log.Printf("Error encoding response: %v", err)
		return
	}
}

func AddUserDataBatch(docs []structs.UserData) {
	var toInsert []interface{}
	for _, doc := range docs {
		toInsert = append(toInsert, doc)
	}
	if len(toInsert) == 0 {
		return
	}
	_, err := db.UserDataCollection.InsertMany(context.TODO(), toInsert)
	if err != nil {
		log.Printf("Error inserting batch user data: %v", err)
	}
}

// func GetOtherUserProfileData(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	username := ps.ByName("username")

// 	// tokenString := r.Header.Get("Authorization")
// 	// claims, err := middleware.ValidateJWT(tokenString)
// 	// if err != nil {
// 	// 	http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 	// 	return
// 	// }
// 	// if username != claims.UserID {
// 	// 	http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 	// 	return
// 	// }
// 	// Parse the query parameter for the entity type
// 	entityType := r.URL.Query().Get("entity_type")
// 	if entityType == "" {
// 		http.Error(w, "Entity type is required", http.StatusBadRequest)
// 		return
// 	}

// 	// Validate the entity type
// 	if !IsValidEntityType(entityType) {
// 		http.Error(w, "Invalid entity type", http.StatusBadRequest)
// 		return
// 	}

// 	// Fetch user data from MongoDB
// 	filter := bson.M{"entity_type": entityType, "userid": username}
// 	cursor, err := db.UserDataCollection.Find(context.TODO(), filter)
// 	if err != nil {
// 		http.Error(w, "Failed to fetch user data", http.StatusInternalServerError)
// 		log.Printf("Error fetching user data: %v", err)
// 		return
// 	}
// 	defer cursor.Close(context.TODO())

// 	var results []structs.UserData
// 	if err := cursor.All(context.TODO(), &results); err != nil {
// 		http.Error(w, "Failed to decode user data", http.StatusInternalServerError)
// 		log.Printf("Error decoding user data: %v", err)
// 		return
// 	}
// 	fmt.Println(results)
// 	if len(results) == 0 {
// 		results = []structs.UserData{}
// 	}

// 	// Respond with the results
// 	w.Header().Set("Content-Type", "application/json")
// 	if err := json.NewEncoder(w).Encode(results); err != nil {
// 		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
// 		log.Printf("Error encoding response: %v", err)
// 		return
// 	}
// }

func GetOtherUserProfileData(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// username := ps.ByName("username") // from /user/{username}/data
	// entityType := r.URL.Query().Get("entity_type")

	// if entityType != "feedpost" {
	// 	http.Error(w, "Invalid entity type", http.StatusBadRequest)
	// 	return
	// }

	// // Connect to MongoDB collection "posts"
	// collection := db.PostsCollection

	// filter := bson.M{"userid": username}
	// cursor, err := collection.Find(context.Background(), filter)
	// if err != nil {
	// 	http.Error(w, "DB error", http.StatusInternalServerError)
	// 	return
	// }
	// defer cursor.Close(context.Background())

	// var posts []bson.M
	// if err = cursor.All(context.Background(), &posts); err != nil {
	// 	http.Error(w, "Cursor decode error", http.StatusInternalServerError)
	// 	return
	// }

	// // Convert MongoDB docs to a simplified response
	// var response []map[string]interface{}
	// for _, post := range posts {
	// 	response = append(response, map[string]interface{}{
	// 		"id":         post["_id"],
	// 		"image_url":  post["image_url"],
	// 		"caption":    post["caption"],
	// 		"created_at": post["created_at"],
	// 	})
	// }

	// w.Header().Set("Content-Type", "application/json")
	// json.NewEncoder(w).Encode(response)

	const res string = `[
		{
		  "id": "post_001",
		  "image_url": "https://i.pinimg.com/736x/a2/1c/98/a21c9856d0ea455ded16bfbcfe6a1104.jpg",
		  "caption": "Sunset in Bali",
		  "created_at": "2025-05-19T12:34:56Z"
		},
		{
		  "id": "post_002",
		  "image_url": "https://i.pinimg.com/736x/a4/11/00/a41100397e2cd32eeaaad432e88cbd14.jpg",
		  "caption": "Coffee time",
		  "created_at": "2025-05-18T08:12:44Z"
		},
		{
		  "id": "post_003",
		  "image_url": "https://i.pinimg.com/736x/37/44/ed/3744edbb1c6bef3b90ea5dd7e212c42e.jpg",
		  "caption": "City lights",
		  "created_at": "2025-05-17T21:09:13Z"
		}
	  ]
	  `
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "%s", res)
}
