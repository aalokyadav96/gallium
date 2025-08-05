package baito

// func UpdateBaito(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// 	if err := r.ParseMultipartForm(20 << 20); err != nil {
// 		http.Error(w, "Invalid form", http.StatusBadRequest)
// 		return
// 	}

// 	baitoId := ps.ByName("id")
// 	objID, err := primitive.ObjectIDFromHex(baitoId)
// 	if err != nil {
// 		http.Error(w, "Invalid baito ID", http.StatusBadRequest)
// 		return
// 	}

// 	update := bson.M{
// 		"$set": bson.M{
// 			"title":        r.FormValue("title"),
// 			"description":  r.FormValue("description"),
// 			"category":     r.FormValue("category"),
// 			"subcategory":  r.FormValue("subcategory"),
// 			"location":     r.FormValue("location"),
// 			"wage":         r.FormValue("wage"),
// 			"phone":        r.FormValue("phone"),
// 			"requirements": r.FormValue("requirements"),
// 			"workHours":    r.FormValue("workHours"),
// 			"updatedAt":    time.Now(),
// 		},
// 	}

// 	if fhArr, ok := r.MultipartForm.File["banner"]; ok && len(fhArr) > 0 {
// 		if url, err := filemgr.SaveFile(fhArr, "baitos"); err == nil {
// 			update["$set"].(bson.M)["banner"] = url
// 		}
// 	}

// 	if images, ok := r.MultipartForm.File["images"]; ok && len(images) > 0 {
// 		imgList, _ := filemgr.SaveMultipleFiles(images, "baitos")
// 		update["$set"].(bson.M)["images"] = imgList
// 	}

// 	filter := bson.M{
// 		"_id":     objID,
// 		"ownerId": utils.GetUserIDFromRequest(r),
// 	}

// 	result, err := db.BaitoCollection.UpdateOne(context.TODO(), filter, update)
// 	if err != nil {
// 		http.Error(w, "Failed to update baito", http.StatusInternalServerError)
// 		return
// 	}
// 	if result.MatchedCount == 0 {
// 		http.Error(w, "Baito not found or unauthorized", http.StatusNotFound)
// 		return
// 	}

// 	utils.RespondWithJSON(w, http.StatusOK, utils.M{
// 		"message": "Baito updated",
// 		"baitoid": baitoId,
// 	})
// }

// // func UpdateBaito(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
// // 	if err := r.ParseMultipartForm(20 << 20); err != nil {
// // 		http.Error(w, "Invalid form", http.StatusBadRequest)
// // 		return
// // 	}

// // 	baitoId := ps.ByName("id")
// // 	objID, err := primitive.ObjectIDFromHex(baitoId)
// // 	if err != nil {
// // 		http.Error(w, "Invalid baito ID", http.StatusBadRequest)
// // 		return
// // 	}

// // 	// Gather and trim
// // 	title := strings.TrimSpace(r.FormValue("title"))
// // 	description := strings.TrimSpace(r.FormValue("description"))
// // 	category := strings.TrimSpace(r.FormValue("category"))
// // 	subcategory := strings.TrimSpace(r.FormValue("subcategory"))
// // 	location := strings.TrimSpace(r.FormValue("location"))
// // 	wage := strings.TrimSpace(r.FormValue("wage"))
// // 	phone := strings.TrimSpace(r.FormValue("phone"))
// // 	requirements := strings.TrimSpace(r.FormValue("requirements"))
// // 	workHours := strings.TrimSpace(r.FormValue("workHours"))

// // 	// Require all
// // 	if title == "" || description == "" || category == "" ||
// // 		subcategory == "" || location == "" || wage == "" ||
// // 		phone == "" || requirements == "" || workHours == "" {
// // 		http.Error(w, "Missing required fields", http.StatusBadRequest)
// // 		return
// // 	}

// // 	update := bson.M{
// // 		"$set": bson.M{
// // 			"title":        title,
// // 			"description":  description,
// // 			"category":     category,
// // 			"subcategory":  subcategory,
// // 			"location":     location,
// // 			"wage":         wage,
// // 			"phone":        phone,
// // 			"requirements": requirements,
// // 			"workHours":    workHours,
// // 			"updatedAt":    time.Now(),
// // 		},
// // 	}

// // 	// Optional: Banner upload
// // 	if fhArr, ok := r.MultipartForm.File["banner"]; ok && len(fhArr) > 0 {
// // 		url, err := saveUploadedFile(fhArr[0])
// // 		if err != nil {
// // 			http.Error(w, "Banner upload failed", http.StatusInternalServerError)
// // 			return
// // 		}
// // 		update["$set"].(bson.M)["banner"] = url
// // 	}

// // 	// Optional: Multiple images
// // 	var imageURLs []string
// // 	if imageFiles, ok := r.MultipartForm.File["images"]; ok {
// // 		for _, fileHeader := range imageFiles {
// // 			url, err := saveUploadedFile(fileHeader)
// // 			if err != nil {
// // 				http.Error(w, "Image upload failed", http.StatusInternalServerError)
// // 				return
// // 			}
// // 			imageURLs = append(imageURLs, url)
// // 		}
// // 	}
// // 	// if len(imageURLs) > 0 {
// // 	// 	update["$push"] = bson.M{
// // 	// 		"images": bson.M{
// // 	// 			"$each": imageURLs,
// // 	// 		},
// // 	// 	}
// // 	// }
// // 	if len(imageURLs) > 0 {
// // 		// Add to $set instead of $push if you want to replace images entirely
// // 		update["$set"].(bson.M)["images"] = imageURLs
// // 	}

// // 	// filter := bson.M{"_id": objID, "ownerID": utils.GetUserIDFromRequest(r)}
// // 	filter := bson.M{
// // 		"_id":     objID,
// // 		"ownerId": utils.GetUserIDFromRequest(r), // use "ownerId" here
// // 	}

// // 	result, err := db.BaitoCollection.UpdateOne(context.TODO(), filter, update)
// // 	if err != nil {
// // 		http.Error(w, "Failed to update baito", http.StatusInternalServerError)
// // 		return
// // 	}
// // 	if result.MatchedCount == 0 {
// // 		http.Error(w, "Baito not found or unauthorized", http.StatusNotFound)
// // 		return
// // 	}

// // 	w.Header().Set("Content-Type", "application/json")
// // 	json.NewEncoder(w).Encode(map[string]string{
// // 		"message": "Baito updated",
// // 		"baitoid": baitoId,
// // 	})
// // }
