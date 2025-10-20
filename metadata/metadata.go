package metadata

import (
	"context"
	"naevis/db"
	"naevis/utils"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetUsersMeta returns minimal metadata for a set of users
func GetUsersMeta(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	idsParam := r.URL.Query().Get("ids")
	if idsParam == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Missing ids param")
		return
	}

	ids := strings.Split(idsParam, ",")
	filter := bson.M{"userid": bson.M{"$in": ids}}

	projection := bson.M{
		"userid":   1,
		"username": 1,
		"name":     1,
		"avatar":   1,
	}

	findOptions := options.Find().SetProjection(projection)

	cursor, err := db.UserCollection.Find(ctx, filter, findOptions)
	if err != nil {
		utils.RespondWithError(w, http.StatusInternalServerError, "DB query failed")
		return
	}
	defer cursor.Close(ctx)

	result := make(map[string]map[string]string)
	for cursor.Next(ctx) {
		var user struct {
			UserID   string `bson:"userid" json:"userid"`
			Username string `bson:"username" json:"username"`
			Name     string `bson:"name,omitempty" json:"name,omitempty"`
			Avatar   string `bson:"avatar,omitempty" json:"avatar,omitempty"`
		}
		if err := cursor.Decode(&user); err != nil {
			continue
		}
		result[user.UserID] = map[string]string{
			"username": user.Username,
			"name":     user.Name,
			"avatar":   user.Avatar,
		}
	}

	utils.RespondWithJSON(w, http.StatusOK, result)
}
