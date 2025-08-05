package baito

// func CreateBaitoUserProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	ctx := context.Background()
// 	userID := utils.GetUserIDFromRequest(r)

// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		http.Error(w, "Invalid form data", http.StatusBadRequest)
// 		return
// 	}

// 	var existing models.BaitoUserProfile
// 	if err := db.BaitoWorkerCollection.FindOne(ctx, bson.M{"user_id": userID}).Decode(&existing); err == nil {
// 		http.Error(w, "Worker profile already exists", http.StatusConflict)
// 		return
// 	} else if err != mongo.ErrNoDocuments {
// 		http.Error(w, "Database error", http.StatusInternalServerError)
// 		return
// 	}

// 	name := r.FormValue("name")
// 	ageStr := r.FormValue("age")
// 	phone := r.FormValue("phone")
// 	location := r.FormValue("location")
// 	roles := r.FormValue("roles")
// 	bio := r.FormValue("bio")

// 	if name == "" || ageStr == "" || phone == "" || location == "" || roles == "" || bio == "" {
// 		http.Error(w, "All fields are required", http.StatusBadRequest)
// 		return
// 	}

// 	age, err := strconv.Atoi(ageStr)
// 	if err != nil || age < 16 {
// 		http.Error(w, "Invalid age", http.StatusBadRequest)
// 		return
// 	}

// 	pic, _ := r.MultipartForm.File["picture"]
// 	picURL, _ := filemgr.SaveFile(pic, "baitos")

// 	profile := models.BaitoUserProfile{
// 		UserID:      userID,
// 		BaitoUserID: utils.GenerateID(12),
// 		Name:        name,
// 		Age:         age,
// 		Phone:       phone,
// 		Location:    location,
// 		Preferred:   roles,
// 		Bio:         bio,
// 		ProfilePic:  picURL,
// 		CreatedAt:   time.Now(),
// 	}

// 	if _, err := db.BaitoWorkerCollection.InsertOne(ctx, profile); err != nil {
// 		http.Error(w, "Failed to save worker profile", http.StatusInternalServerError)
// 		return
// 	}

// 	_, _ = db.UserCollection.UpdateOne(ctx,
// 		bson.M{"userid": userID},
// 		bson.M{
// 			"$addToSet": bson.M{"role": "worker"},
// 			"$set":      bson.M{"updated_at": time.Now()},
// 		},
// 	)

// 	utils.RespondWithJSON(w, http.StatusOK, utils.M{"message": "Worker profile created successfully"})
// }

// // func CreateBaitoUserProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// // 	ctx := context.Background()
// // 	userID := utils.GetUserIDFromRequest(r)

// // 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// // 		http.Error(w, "Invalid form data", http.StatusBadRequest)
// // 		return
// // 	}

// // 	// Prevent duplicate profile
// // 	var existing models.BaitoUserProfile
// // 	if err := db.BaitoWorkerCollection.FindOne(ctx, bson.M{"user_id": userID}).Decode(&existing); err == nil {
// // 		http.Error(w, "Worker profile already exists", http.StatusConflict)
// // 		return
// // 	} else if err != mongo.ErrNoDocuments {
// // 		http.Error(w, "Database error", http.StatusInternalServerError)
// // 		return
// // 	}

// // 	// Parse fields
// // 	name := r.FormValue("name")
// // 	ageStr := r.FormValue("age")
// // 	phone := r.FormValue("phone")
// // 	location := r.FormValue("location")
// // 	roles := r.FormValue("roles")
// // 	bio := r.FormValue("bio")

// // 	if name == "" || ageStr == "" || phone == "" || location == "" || roles == "" || bio == "" {
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
// // 		profilePicPath, err = saveUploadedFile(handler)
// // 		if err != nil {
// // 			http.Error(w, "Failed to upload profile picture", http.StatusInternalServerError)
// // 			return
// // 		}
// // 	}

// // 	profile := models.BaitoUserProfile{
// // 		UserID:      userID,
// // 		BaitoUserID: utils.GenerateID(12),
// // 		Name:        name,
// // 		Age:         age,
// // 		Phone:       phone,
// // 		Location:    location,
// // 		Preferred:   roles,
// // 		Bio:         bio,
// // 		ProfilePic:  profilePicPath,
// // 		CreatedAt:   time.Now(),
// // 	}

// // 	if _, err := db.BaitoWorkerCollection.InsertOne(ctx, profile); err != nil {
// // 		http.Error(w, "Failed to save worker profile", http.StatusInternalServerError)
// // 		return
// // 	}

// // 	_, _ = db.UserCollection.UpdateOne(ctx,
// // 		bson.M{"userid": userID},
// // 		bson.M{
// // 			"$addToSet": bson.M{"role": "worker"},
// // 			"$set":      bson.M{"updated_at": time.Now()},
// // 		},
// // 	)

// // 	w.Header().Set("Content-Type", "application/json")
// // 	json.NewEncoder(w).Encode(map[string]string{"message": "Worker profile created successfully"})
// // }
