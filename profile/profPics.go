package profile

import (
	"naevis/middleware"
	"naevis/models"
	"naevis/mq"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// Update profile picture
func EditProfilePic(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Validate JWT token
	claims, err := middleware.ValidateJWT(r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse the multipart form
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// Update profile picture
	pictureUpdates, err := updateProfilePictures(w, r, claims)
	if err != nil {
		http.Error(w, "Failed to update profile picture", http.StatusInternalServerError)
		return
	}

	// Save updated profile picture to the database
	if err := ApplyProfileUpdates(claims.UserID, pictureUpdates); err != nil {
		http.Error(w, "Failed to update profile picture", http.StatusInternalServerError)
		return
	}

	m := models.Index{}
	mq.Notify("profilepic-updated", m)

	InvalidateCachedProfile(claims.Username)

	// Respond with the updated profile
	if err := RespondWithUserProfile(w, claims.UserID); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Update banner picture
func EditProfileBanner(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Validate JWT token
	claims, err := middleware.ValidateJWT(r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse the multipart form
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// Update banner picture
	bannerUpdates, err := uploadBannerHandler(w, r, claims)
	if err != nil {
		http.Error(w, "Failed to update banner picture", http.StatusInternalServerError)
		return
	}

	// Save updated banner picture to the database
	if err := ApplyProfileUpdates(claims.UserID, bannerUpdates); err != nil {
		http.Error(w, "Failed to update banner picture", http.StatusInternalServerError)
		return
	}

	m := models.Index{}
	mq.Notify("bannerpic-updated", m)

	InvalidateCachedProfile(claims.Username)

	// Respond with the updated profile
	if err := RespondWithUserProfile(w, claims.UserID); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
