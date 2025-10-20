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

	pictureUpdates, err := updateAvatars(w, r, claims)
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
	origName, ok := pictureUpdates["avatar"].(string)
	if !ok {
		http.Error(w, "Failed to get updated image name", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"avatar": origName})
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
	origName, ok := bannerUpdates["banner"].(string)
	if !ok {
		http.Error(w, "Failed to get updated banner image name", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"banner": origName})
}

func uploadBannerHandler(_ http.ResponseWriter, r *http.Request, _ *middleware.Claims) (bson.M, error) {
	update := bson.M{}

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		return nil, fmt.Errorf("error parsing form data: %w", err)
	}

	file, header, err := r.FormFile("banner")
	if err != nil {
		return nil, fmt.Errorf("banner upload failed: %w", err)
	}
	defer file.Close()

	origName, _, err := filemgr.SaveImageWithThumb(file, header, filemgr.EntityUser, filemgr.PicBanner, 300, "")
	if err != nil {
		return nil, fmt.Errorf("save image with thumb failed: %w", err)
	}

	update["banner"] = origName
	mq.Notify("banner-uploaded", models.Index{})

	return update, nil
}

func updateAvatars(_ http.ResponseWriter, r *http.Request, claims *middleware.Claims) (bson.M, error) {
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

	update["avatar"] = origName
	update["profile_thumb"] = thumbName

	mq.Notify("avatar-uploaded", models.Index{})

	return update, nil
}
