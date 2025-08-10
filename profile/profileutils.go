package profile

import (
	"fmt"
	"naevis/filemgr"
	"naevis/middleware"
	"naevis/models"
	"naevis/mq"
	"net/http"

	"go.mongodb.org/mongo-driver/bson"
)

// // createThumbnail creates a thumbnail of the image at inputPath and saves it at outputPath.
// func createThumbnail(inputPath, outputPath string, width, height int) error {
// 	// width := 100
// 	// height := 100

// 	img, err := imaging.Open(inputPath)
// 	if err != nil {
// 		return err
// 	}
// 	resizedImg := imaging.Resize(img, width, height, imaging.Lanczos)

// 	m := models.Index{}
// 	mq.Notify("thumbnail-created", m)

// 	return imaging.Save(resizedImg, outputPath)
// }

func uploadBannerHandler(_ http.ResponseWriter, r *http.Request, _ *middleware.Claims) (bson.M, error) {
	update := bson.M{}

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		return nil, fmt.Errorf("error parsing form data: %w", err)
	}

	file, header, err := r.FormFile("banner_picture")
	if err != nil {
		return nil, fmt.Errorf("banner upload failed: %w", err)
	}
	defer file.Close()

	origName, _, err := filemgr.SaveImageWithThumb(file, header, filemgr.EntityUser, filemgr.PicBanner, 300, "")
	if err != nil {
		return nil, fmt.Errorf("save image with thumb failed: %w", err)
	}

	update["banner_picture"] = origName
	mq.Notify("banner-uploaded", models.Index{})

	return update, nil
}

func updateProfilePictures(_ http.ResponseWriter, r *http.Request, claims *middleware.Claims) (bson.M, error) {
	update := bson.M{}
	_ = claims
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		return nil, fmt.Errorf("error parsing form data: %w", err)
	}

	file, header, err := r.FormFile("avatar_picture")
	if err != nil {
		return nil, fmt.Errorf("avatar upload failed: %w", err)
	}
	defer file.Close()

	origName, thumbName, err := filemgr.SaveImageWithThumb(file, header, filemgr.EntityUser, filemgr.PicPhoto, 100, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("save image with thumb failed: %w", err)
	}

	update["profile_picture"] = origName
	update["profile_thumb"] = thumbName

	mq.Notify("avatar-uploaded", models.Index{})

	return update, nil
}

// func uploadBannerHandler(_ http.ResponseWriter, r *http.Request, _ *middleware.Claims) (bson.M, error) {
// 	update := bson.M{}
// 	// Parse form data
// 	err := r.ParseMultipartForm(10 << 20) // Limit to 10MB
// 	if err != nil {
// 		// http.Error(w, "Error parsing form data", http.StatusBadRequest)
// 		return nil, fmt.Errorf("error parsing form data: %w", err)
// 	}

// 	// Retrieve the file
// 	file, _, err := r.FormFile("banner_picture")
// 	if err != nil {
// 		// http.Error(w, "Error retrieving the file", http.StatusBadRequest)
// 		return nil, fmt.Errorf("error retrieving the file: %w", err)
// 	}
// 	defer file.Close()

// 	// bannerFileName := claims.Username
// 	bannerFileName := utils.GenerateID(12)

// 	// Save the file
// 	// filePath := filepath.Join("./userpic/banner", handler.Filename)
// 	// filePath := "./userpic/banner/" + bannerFileName + ".jpg"
// 	filePath := "./static/userpic/banner/" + bannerFileName + ".webp"
// 	outFile, err := os.Create(filePath)
// 	if err != nil {
// 		// http.Error(w, "Error saving the file", http.StatusInternalServerError)
// 		return nil, fmt.Errorf("error saving the file: %w", err)
// 	}
// 	defer outFile.Close()

// 	// Write the file content
// 	_, err = outFile.ReadFrom(file)
// 	if err != nil {
// 		// http.Error(w, "Error writing the file", http.StatusInternalServerError)
// 		return nil, fmt.Errorf("error writing the file: %w", err)
// 	}

// 	// update["banner_picture"] = bannerFileName + ".jpg"
// 	update["banner_picture"] = bannerFileName + ".webp"

// 	m := models.Index{}
// 	mq.Notify("banner-uploaded", m)

// 	return update, nil

// 	// // Respond with success
// 	// w.Header().Set("Content-Type", "application/json")
// 	// w.WriteHeader(http.StatusOK)
// 	// fmt.Fprintf(w, `{"message": "Thumbnail uploaded successfully", "ok": true, "file": "%s"}`, handler.Filename)
// }

// func updateProfilePictures(w http.ResponseWriter, r *http.Request, claims *middleware.Claims) (bson.M, error) {
// 	_ = w
// 	update := bson.M{}
// 	// Parse form data
// 	err := r.ParseMultipartForm(10 << 20) // Limit to 10MB
// 	if err != nil {
// 		// http.Error(w, "Error parsing form data", http.StatusBadRequest)
// 		return nil, fmt.Errorf("error parsing form data: %w", err)
// 	}

// 	// Retrieve the file
// 	file, _, err := r.FormFile("avatar_picture")
// 	if err != nil {
// 		// http.Error(w, "Error retrieving the file", http.StatusBadRequest)
// 		return nil, fmt.Errorf("error retrieving the file: %w", err)
// 	}
// 	defer file.Close()

// 	profileFileName := utils.GenerateID(12)

// 	// Save the file
// 	// filePath := filepath.Join("./userpic", handler.Filename)
// 	filePath := "./static/userpic/" + profileFileName + ".jpg"
// 	outFile, err := os.Create(filePath)
// 	if err != nil {
// 		// http.Error(w, "Error saving the file", http.StatusInternalServerError)
// 		return nil, fmt.Errorf("error saving the file: %w", err)
// 	}
// 	defer outFile.Close()

// 	// Write the file content
// 	_, err = outFile.ReadFrom(file)
// 	if err != nil {
// 		// http.Error(w, "Error writing the file", http.StatusInternalServerError)
// 		return nil, fmt.Errorf("error writing the file: %w", err)
// 	}

// 	thumbPath := "./static/userpic/thumb/" + claims.UserID + ".jpg"
// 	createThumbnail(filePath, thumbPath, 100, 100)

// 	update["profile_picture"] = profileFileName + ".jpg"

// 	m := models.Index{}
// 	mq.Notify("avatar-uploaded", m)

// 	return update, nil

// 	// // Respond with success
// 	// w.Header().Set("Content-Type", "application/json")
// 	// w.WriteHeader(http.StatusOK)
// 	// fmt.Fprintf(w, `{"message": "Thumbnail uploaded successfully", "ok": true, "file": "%s"}`, handler.Filename)
// }
