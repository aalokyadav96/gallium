package profile

import (
	"encoding/json"
	"fmt"
	"naevis/filemgr"
	"naevis/middleware"
	"naevis/models"
	"naevis/mq"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
)

// Edit profile picture
func EditProfilePic(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	claims, err := middleware.ValidateJWT(r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	pictureUpdates, err := updateProfilePictures(w, r, claims)
	if err != nil {
		http.Error(w, "Failed to update profile picture", http.StatusInternalServerError)
		return
	}

	if err := ApplyProfileUpdates(claims.UserID, pictureUpdates); err != nil {
		http.Error(w, "Failed to update profile picture", http.StatusInternalServerError)
		return
	}

	mq.Notify("profilepic-updated", models.Index{})
	InvalidateCachedProfile(claims.Username)

	// Return only the new image name as JSON
	origName, ok := pictureUpdates["profile_picture"].(string)
	if !ok {
		http.Error(w, "Failed to get updated image name", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"profile_picture": origName})
}

// Edit banner picture
func EditProfileBanner(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	claims, err := middleware.ValidateJWT(r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	bannerUpdates, err := uploadBannerHandler(w, r, claims)
	if err != nil {
		http.Error(w, "Failed to update banner picture", http.StatusInternalServerError)
		return
	}

	if err := ApplyProfileUpdates(claims.UserID, bannerUpdates); err != nil {
		http.Error(w, "Failed to update banner picture", http.StatusInternalServerError)
		return
	}

	mq.Notify("bannerpic-updated", models.Index{})
	InvalidateCachedProfile(claims.Username)

	// Return only the new image name as JSON
	origName, ok := bannerUpdates["banner_picture"].(string)
	if !ok {
		http.Error(w, "Failed to get updated banner image name", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"banner_picture": origName})
}

// // Update profile picture
// func EditProfilePic(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	// Validate JWT token
// 	claims, err := middleware.ValidateJWT(r.Header.Get("Authorization"))
// 	if err != nil {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}

// 	// Parse the multipart form
// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		http.Error(w, "Unable to parse form", http.StatusBadRequest)
// 		return
// 	}

// 	// Update profile picture
// 	pictureUpdates, err := updateProfilePictures(w, r, claims)
// 	if err != nil {
// 		http.Error(w, "Failed to update profile picture", http.StatusInternalServerError)
// 		return
// 	}

// 	// Save updated profile picture to the database
// 	if err := ApplyProfileUpdates(claims.UserID, pictureUpdates); err != nil {
// 		http.Error(w, "Failed to update profile picture", http.StatusInternalServerError)
// 		return
// 	}

// 	m := models.Index{}
// 	mq.Notify("profilepic-updated", m)

// 	InvalidateCachedProfile(claims.Username)

// 	// Respond with the updated profile
// 	if err := RespondWithUserProfile(w, claims.UserID); err != nil {
// 		http.Error(w, "Internal server error", http.StatusInternalServerError)
// 	}
// }

// // Update banner picture
// func EditProfileBanner(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	// Validate JWT token
// 	claims, err := middleware.ValidateJWT(r.Header.Get("Authorization"))
// 	if err != nil {
// 		http.Error(w, "Unauthorized", http.StatusUnauthorized)
// 		return
// 	}

// 	// Parse the multipart form
// 	if err := r.ParseMultipartForm(10 << 20); err != nil {
// 		http.Error(w, "Unable to parse form", http.StatusBadRequest)
// 		return
// 	}

// 	// Update banner picture
// 	bannerUpdates, err := uploadBannerHandler(w, r, claims)
// 	if err != nil {
// 		http.Error(w, "Failed to update banner picture", http.StatusInternalServerError)
// 		return
// 	}

// 	// Save updated banner picture to the database
// 	if err := ApplyProfileUpdates(claims.UserID, bannerUpdates); err != nil {
// 		http.Error(w, "Failed to update banner picture", http.StatusInternalServerError)
// 		return
// 	}

// 	m := models.Index{}
// 	mq.Notify("bannerpic-updated", m)

// 	InvalidateCachedProfile(claims.Username)

//		// Respond with the updated profile
//		if err := RespondWithUserProfile(w, claims.UserID); err != nil {
//			http.Error(w, "Internal server error", http.StatusInternalServerError)
//		}
//	}

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
