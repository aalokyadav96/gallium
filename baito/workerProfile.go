package baito

import (
	"context"
	"encoding/json"
	"naevis/db"
	"naevis/models"
	"naevis/utils"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func CreateBaitoUserProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := context.Background()
	userID := utils.GetUserIDFromRequest(r)

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Prevent duplicate profile
	var existing models.BaitoUserProfile
	if err := db.BaitoWorkerCollection.FindOne(ctx, bson.M{"user_id": userID}).Decode(&existing); err == nil {
		http.Error(w, "Worker profile already exists", http.StatusConflict)
		return
	} else if err != mongo.ErrNoDocuments {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Parse fields
	name := r.FormValue("name")
	ageStr := r.FormValue("age")
	phone := r.FormValue("phone")
	location := r.FormValue("location")
	roles := r.FormValue("roles")
	bio := r.FormValue("bio")

	if name == "" || ageStr == "" || phone == "" || location == "" || roles == "" || bio == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	age, err := strconv.Atoi(ageStr)
	if err != nil || age < 16 {
		http.Error(w, "Invalid age", http.StatusBadRequest)
		return
	}

	var profilePicPath string
	file, handler, err := r.FormFile("picture")
	if err == nil {
		defer file.Close()
		profilePicPath, err = saveUploadedFile(handler)
		if err != nil {
			http.Error(w, "Failed to upload profile picture", http.StatusInternalServerError)
			return
		}
	}

	profile := models.BaitoUserProfile{
		UserID:      userID,
		BaitoUserID: utils.GenerateID(12),
		Name:        name,
		Age:         age,
		Phone:       phone,
		Location:    location,
		Preferred:   roles,
		Bio:         bio,
		ProfilePic:  profilePicPath,
		CreatedAt:   time.Now(),
	}

	if _, err := db.BaitoWorkerCollection.InsertOne(ctx, profile); err != nil {
		http.Error(w, "Failed to save worker profile", http.StatusInternalServerError)
		return
	}

	_, _ = db.UserCollection.UpdateOne(ctx,
		bson.M{"userid": userID},
		bson.M{
			"$addToSet": bson.M{"role": "worker"},
			"$set":      bson.M{"updated_at": time.Now()},
		},
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Worker profile created successfully"})
}

// func CreateBaitoUserProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx := context.Background()
// 	// savepath := "/uploads/baitos/userpic"

// 	userID := utils.GetUserIDFromRequest(r)

// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		http.Error(w, "Invalid form data", http.StatusBadRequest)
// 		return
// 	}

// 	// 🔍 Check if profile already exists
// 	existsFilter := bson.M{"user_id": userID}
// 	var existingProfile models.BaitoUserProfile
// 	err := db.BaitoWorkerCollection.FindOne(ctx, existsFilter).Decode(&existingProfile)
// 	if err == nil {
// 		http.Error(w, "Worker profile already exists", http.StatusConflict)
// 		return
// 	} else if err != mongo.ErrNoDocuments {
// 		http.Error(w, "Database error", http.StatusInternalServerError)
// 		return
// 	}

// 	// 📝 Parse and validate fields
// 	name := r.FormValue("name")
// 	ageStr := r.FormValue("age")
// 	phone := r.FormValue("phone")
// 	location := r.FormValue("location")
// 	preferred := r.FormValue("roles")
// 	bio := r.FormValue("bio")

// 	if name == "" || ageStr == "" || phone == "" || location == "" || preferred == "" || bio == "" {
// 		http.Error(w, "All fields are required", http.StatusBadRequest)
// 		return
// 	}

// 	age, err := strconv.Atoi(ageStr)
// 	if err != nil || age < 16 {
// 		http.Error(w, "Invalid age", http.StatusBadRequest)
// 		return
// 	}

// 	var profilePicPath string
// 	file, handler, err := r.FormFile("picture")
// 	if err == nil {
// 		defer file.Close()
// 		// profilePicPath, err = saveUploadedFile(handler, savepath)
// 		profilePicPath, err = saveUploadedFile(handler)
// 		if err != nil {
// 			http.Error(w, "Failed to upload profile picture", http.StatusInternalServerError)
// 			return
// 		}
// 	}

// 	// 🛠 Build profile struct
// 	profile := models.BaitoUserProfile{
// 		UserID:      userID,
// 		BaitoUserID: utils.GenerateID(12),
// 		Name:        name,
// 		Age:         age,
// 		Phone:       phone,
// 		Location:    location,
// 		Preferred:   preferred,
// 		Bio:         bio,
// 		ProfilePic:  profilePicPath,
// 		CreatedAt:   time.Now(),
// 	}

// 	// 💾 Save to BaitoWorkerCollection
// 	_, err = db.BaitoWorkerCollection.InsertOne(ctx, profile)
// 	if err != nil {
// 		http.Error(w, "Failed to save worker profile", http.StatusInternalServerError)
// 		return
// 	}

// 	// 🔄 Update main User record with worker role
// 	filter := bson.M{"userid": userID}
// 	update := bson.M{
// 		"$addToSet": bson.M{
// 			"role": "worker",
// 		},
// 		"$set": bson.M{
// 			"updated_at": time.Now(),
// 		},
// 	}
// 	_, _ = db.UserCollection.UpdateOne(ctx, filter, update)

// 	w.WriteHeader(http.StatusCreated)
// 	fmt.Fprint(w, `{"message":"Worker profile created successfully"}`)
// }

// // func CreateBaitoUserProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// // 	ctx := context.Background()
// // 	savepath := "/uploads/baitos/userpic"

// // 	userID := utils.GetUserIDFromRequest(r)

// // 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// // 		http.Error(w, "Invalid form data", http.StatusBadRequest)
// // 		return
// // 	}

// // 	name := r.FormValue("name")
// // 	ageStr := r.FormValue("age")
// // 	phone := r.FormValue("phone")
// // 	location := r.FormValue("location")
// // 	preferred := r.FormValue("roles")
// // 	bio := r.FormValue("bio")

// // 	if name == "" || ageStr == "" || phone == "" || location == "" || preferred == "" || bio == "" {
// // 		http.Error(w, "All fields are required", http.StatusBadRequest)
// // 		return
// // 	}

// // 	age, err := strconv.Atoi(ageStr)
// // 	if err != nil || age < 16 {
// // 		http.Error(w, "Invalid age", http.StatusBadRequest)
// // 		return
// // 	}

// // 	var profilePicPath string
// // 	file, handler, err := r.FormFile("picture")
// // 	if err == nil {
// // 		defer file.Close()
// // 		profilePicPath, err = saveUploadedFile(handler, savepath)
// // 		if err != nil {
// // 			http.Error(w, "Failed to upload profile picture", http.StatusInternalServerError)
// // 			return
// // 		}
// // 	}

// // 	// Create profile struct
// // 	profile := models.BaitoUserProfile{
// // 		UserID:      userID,
// // 		BaitoUserID: utils.GenerateID(12),
// // 		Name:        name,
// // 		Age:         age,
// // 		Phone:       phone,
// // 		Location:    location,
// // 		Preferred:   preferred,
// // 		Bio:         bio,
// // 		ProfilePic:  profilePicPath,
// // 		CreatedAt:   time.Now(),
// // 	}

// // 	// Save to BaitoWorkerCollection
// // 	_, err = db.BaitoWorkerCollection.InsertOne(ctx, profile)
// // 	if err != nil {
// // 		http.Error(w, "Failed to save worker profile", http.StatusInternalServerError)
// // 		return
// // 	}

// // 	// Also update user record to reflect role = worker
// // 	filter := bson.M{"userid": userID}
// // 	update := bson.M{
// // 		"$addToSet": bson.M{
// // 			"role": "worker",
// // 		},
// // 		"$set": bson.M{
// // 			"updated_at": time.Now(),
// // 		},
// // 	}
// // 	_, _ = db.UserCollection.UpdateOne(ctx, filter, update)

// // 	w.WriteHeader(http.StatusCreated)
// // 	fmt.Fprint(w, `{"message":"Worker profile created successfully"}`)
// // }

// // // //	func CreateBaitoUserProfile(usersCollection *mongo.Collection) httprouter.Handle {
// // // //		return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// // // func CreateBaitoUserProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// // // 	ctx := context.Background()

// // // 	var savepath = "/uploads/baitos/userpics"

// // // 	userID := utils.GetUserIDFromRequest(r)
// // // 	// userID, err := utils.GetUserIDFromRequest(r)
// // // 	// if err != nil {
// // // 	// 	http.Error(w, "Unauthorized", http.StatusUnauthorized)
// // // 	// 	return
// // // 	// }

// // // 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// // // 		http.Error(w, "Invalid form data", http.StatusBadRequest)
// // // 		return
// // // 	}

// // // 	// Required fields
// // // 	name := r.FormValue("name")
// // // 	age := r.FormValue("age")
// // // 	phone := r.FormValue("phone")
// // // 	location := r.FormValue("location")
// // // 	preferred := r.FormValue("roles")
// // // 	bio := r.FormValue("bio")

// // // 	if name == "" || age == "" || phone == "" || location == "" || preferred == "" || bio == "" {
// // // 		http.Error(w, "All fields are required", http.StatusBadRequest)
// // // 		return
// // // 	}

// // // 	// Handle profile picture
// // // 	var profilePicPath string
// // // 	file, handler, err := r.FormFile("picture")
// // // 	if err == nil {
// // // 		defer file.Close()
// // // 		profilePicPath, err = saveUploadedFile(handler, savepath)
// // // 		if err != nil {
// // // 			http.Error(w, "Failed to upload profile picture", http.StatusInternalServerError)
// // // 			return
// // // 		}
// // // 	}

// // // 	// Update user document
// // // 	update := bson.M{
// // // 		"$set": bson.M{
// // // 			"name":            name,
// // // 			"bio":             bio,
// // // 			"phone_number":    phone,
// // // 			"address":         location,
// // // 			"profile_picture": profilePicPath,
// // // 			"updated_at":      time.Now(),
// // // 		},
// // // 		"$addToSet": bson.M{
// // // 			"role": "worker",
// // // 		},
// // // 	}

// // // 	filter := bson.M{"userid": userID}
// // // 	_, err = db.UserCollection.UpdateOne(ctx, filter, update)
// // // 	if err != nil {
// // // 		http.Error(w, "Database update failed", http.StatusInternalServerError)
// // // 		return
// // // 	}

// // // 	w.WriteHeader(http.StatusCreated)
// // // 	fmt.Fprint(w, `{"message":"Profile created"}`)
// // // }

// // // // 	}
// // // // }
