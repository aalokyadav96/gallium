package profile

import (
	"context"
	"encoding/json"
	"net/http"

	"naevis/db"
	"naevis/middleware"
	"naevis/models"
	"naevis/mq"
	"naevis/rdx"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"golang.org/x/crypto/bcrypt"
)

// EditProfile allows a user to update their own profile fields.
func EditProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	// 1. Extract and validate the JWT from the Authorization header (strip "Bearer " if present).
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	tokenString := authHeader
	// if strings.HasPrefix(authHeader, "Bearer ") {
	// 	tokenString = strings.TrimPrefix(authHeader, "Bearer ")
	// }

	claims, err := middleware.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Parse multipart form data (e.g., for profile picture upload). Limit to ~10 MiB.
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// 3. Invalidate any cached profile JSON in Redis, so subsequent reads fetch fresh data.
	_ = InvalidateCachedProfile(claims.Username)
	_ = UpdateCachedUsername(claims.UserID)
	// 4. Build a bson.M of all fields user wants to update.
	updates, err := UpdateProfileFields(r, claims)
	if err != nil {
		http.Error(w, "Failed to update profile fields", http.StatusInternalServerError)
		return
	}

	// // 5. Persist those updates into MongoDB.
	// if err := UpdateUserByUsername(claims.Username, updates); err != nil {
	// 	http.Error(w, "Failed to update profile", http.StatusInternalServerError)
	// 	return
	// }

	// 5. Persist those updates into MongoDB.
	if err := ApplyProfileUpdates(claims.UserID, updates); err != nil {
		http.Error(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}

	// 6. Emit a “profile-edited” event asynchronously.
	m := models.Index{
		EntityType: "profile",
		EntityId:   claims.UserID,
		Method:     "PUT",
	}
	go mq.Emit(ctx, "profile-edited", m)

	// 7. Respond with the newly updated profile.
	if err := RespondWithUserProfile(w, claims.UserID); err != nil {
		// If RespondWithUserProfile fails, return a generic 500.
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// DeleteProfile deletes the authenticated user’s profile completely.
func DeleteProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := r.Context()
	// 1. Extract and validate the JWT from the Authorization header (strip "Bearer " if present).
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	tokenString := authHeader
	// if strings.HasPrefix(authHeader, "Bearer ") {
	// 	tokenString = strings.TrimPrefix(authHeader, "Bearer ")
	// }
	claims, err := middleware.ValidateJWT(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Invalidate the cached profile JSON in Redis.
	_ = InvalidateCachedProfile(claims.Username)

	// 3. Remove the user document from MongoDB by userID.
	if err := DeleteUserByID(claims.UserID); err != nil {
		http.Error(w, "Failed to delete profile", http.StatusInternalServerError)
		return
	}

	// 4. Emit a “profile-deleted” event asynchronously.
	m := models.Index{
		EntityType: "profile",
		EntityId:   claims.UserID,
		Method:     "DELETE",
	}
	go mq.Emit(ctx, "profile-deleted", m)

	// 5. Return a simple JSON success message.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Profile deleted successfully",
	})
}

// UpdateProfileFields inspects form values (and potentially uploaded files)
// to assemble a bson.M of fields that should be updated for this user.
func UpdateProfileFields(r *http.Request, claims *middleware.Claims) (bson.M, error) {
	update := bson.M{}

	// Username change
	if newUsername := r.FormValue("username"); newUsername != "" && newUsername != claims.Username {
		update["username"] = newUsername
		// Also update Redis hash that maps userID -> username, if you use that for lookups.
		if err := rdx.RdxHset("users", claims.UserID, newUsername); err != nil {
			return nil, err
		}
	}

	// Email change
	if newEmail := r.FormValue("email"); newEmail != "" {
		update["email"] = newEmail
	}

	// Bio change
	if newBio := r.FormValue("bio"); newBio != "" {
		update["bio"] = newBio
	}

	// Name change
	if newName := r.FormValue("name"); newName != "" {
		update["name"] = newName
	}

	// Phone number change
	if newPhone := r.FormValue("phone"); newPhone != "" {
		update["phone_number"] = newPhone
	}

	// Optional: password change
	if newPassword := r.FormValue("password"); newPassword != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		update["password"] = string(hashed)
	}

	// (If you handle file uploads for Avatar or bannerPicture,
	//  you would process r.MultipartForm here and store the new file, then
	//  include something like update["Avatar"] = "<new URL>".
	//  That is omitted unless you specifically need to handle files.)

	// Notify a generic “profile-updated” event (payload is empty index here; fill as needed).
	m := models.Index{}
	mq.Notify("profile-updated", m)

	return update, nil
}

// ApplyProfileUpdates merges multiple bson.M maps into a single update map.
// (Not currently used if you’re only calling UpdateUserByUsername directly.)
func ApplyProfileUpdates(userid string, updates ...bson.M) error {
	finalUpdate := bson.M{}
	for _, u := range updates {
		for k, v := range u {
			finalUpdate[k] = v
		}
	}

	_, err := db.UserCollection.UpdateOne(
		context.TODO(),
		bson.M{"userid": userid},
		bson.M{"$set": finalUpdate},
	)
	return err
}

// UpdateUserByUsername writes a partial update (bson.M) into the "users" collection.
func UpdateUserByUsername(username string, update bson.M) error {
	_, err := db.UserCollection.UpdateOne(
		context.TODO(),
		bson.M{"username": username},
		bson.M{"$set": update},
	)
	return err
}

// DeleteUserByID removes a user document by its userID field.
func DeleteUserByID(userID string) error {
	_, err := db.UserCollection.DeleteOne(
		context.TODO(),
		bson.M{"userid": userID},
	)
	return err
}
