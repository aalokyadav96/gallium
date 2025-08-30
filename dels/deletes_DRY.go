package dels

import (
	"context"
	"fmt"
	"naevis/db"
	"naevis/globals"
	"naevis/middleware"
	"naevis/models"
	"naevis/mq"
	"naevis/rdx"
	"naevis/userdata"
	"naevis/utils"
	"net/http"
	"os"
	"slices"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type permissionFn func(ctx context.Context, r *http.Request, entityID string) error
type afterDeleteFn func(ctx context.Context, entityID, userID string)

// ---- Core Helper ----

func deleteByField(
	w http.ResponseWriter, r *http.Request, ps httprouter.Params,
	collection *mongo.Collection, paramKey, fieldKey, entityType, mqTopic string,
	perm permissionFn, after afterDeleteFn,
) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	entityID := ps.ByName(paramKey)
	if entityID == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	userID, _ := r.Context().Value(globals.UserIDKey).(string)

	if perm != nil {
		if err := perm(ctx, r, entityID); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
	}

	res, err := collection.DeleteOne(ctx, bson.M{fieldKey: entityID})
	if err != nil || res.DeletedCount == 0 {
		http.Error(w, "Delete failed", http.StatusInternalServerError)
		return
	}

	if after != nil {
		after(ctx, entityID, userID)
	}

	go mq.Emit(ctx, mqTopic, models.Index{EntityType: entityType, EntityId: entityID, Method: "DELETE"})

	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true})
}

// ---- Soft Delete Helper ----

func softDeleteByField(
	w http.ResponseWriter, r *http.Request, ps httprouter.Params,
	collection *mongo.Collection, paramKey, fieldKey, entityType, mqTopic string,
	update bson.M, perm permissionFn, after afterDeleteFn,
) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	entityID := ps.ByName(paramKey)
	if entityID == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	userID, _ := r.Context().Value(globals.UserIDKey).(string)

	if perm != nil {
		if err := perm(ctx, r, entityID); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
	}

	_, err := collection.UpdateOne(ctx, bson.M{fieldKey: entityID}, update)
	if err != nil {
		http.Error(w, "Delete failed", http.StatusInternalServerError)
		return
	}

	if after != nil {
		after(ctx, entityID, userID)
	}

	go mq.Emit(ctx, mqTopic, models.Index{EntityType: entityType, EntityId: entityID, Method: "DELETE"})

	utils.RespondWithJSON(w, http.StatusOK, utils.M{"success": true})
}

// ---- Handlers ----

func DeleteRecipe(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteByField(w, r, ps, db.RecipeCollection, "id", "recipeid", "recipe", "recipe-deleted", nil, nil)
}

func DeleteMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	softDeleteByField(w, r, ps, db.MessagesCollection, "messageId", "_id", "message", "message-deleted",
		bson.M{"$set": bson.M{"deleted": true}}, nil, nil)
}

func DeletesMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteByField(w, r, ps, db.MessagesCollection, "msgid", "_id", "message", "message-deleted",
		func(ctx context.Context, r *http.Request, entityID string) error {
			claims, err := middleware.ValidateJWT(r.Header.Get("Authorization"))
			if err != nil {
				return fmt.Errorf("unauthorized")
			}
			objID, _ := primitive.ObjectIDFromHex(entityID)
			var msg models.Message
			if err := db.MessagesCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&msg); err != nil {
				return fmt.Errorf("not found")
			}
			if msg.UserID != claims.UserID {
				return fmt.Errorf("forbidden")
			}
			chatObjID, _ := primitive.ObjectIDFromHex(msg.ChatID)
			_, _ = db.ChatsCollection.UpdateOne(ctx, bson.M{"_id": chatObjID}, bson.M{"$set": bson.M{"updatedAt": time.Now()}})
			return nil
		}, nil)
}

func DeleteComment(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteByField(w, r, ps, db.CommentsCollection, "commentid", "_id", "comment", "comment-deleted",
		func(ctx context.Context, r *http.Request, entityID string) error {
			objID, _ := primitive.ObjectIDFromHex(entityID)
			claims, err := middleware.ValidateJWT(r.Header.Get("Authorization"))
			if err != nil {
				return fmt.Errorf("unauthorized")
			}
			var c models.Comment
			if err := db.CommentsCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&c); err != nil {
				return fmt.Errorf("not found")
			}
			if c.CreatedBy != claims.UserID {
				return fmt.Errorf("forbidden")
			}
			return nil
		}, nil)
}

func DeleteEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteByField(w, r, ps, db.EventsCollection, "eventid", "eventid", "event", "event-deleted",
		func(ctx context.Context, r *http.Request, entityID string) error {
			userID, _ := r.Context().Value(globals.UserIDKey).(string)
			var ev models.Event
			if err := db.EventsCollection.FindOne(ctx, bson.M{"eventid": entityID}).Decode(&ev); err != nil {
				return fmt.Errorf("not found")
			}
			if ev.CreatorID != userID {
				return fmt.Errorf("forbidden")
			}
			return nil
		},
		func(ctx context.Context, entityID, userID string) {
			deleteRelatedData(entityID)
			userdata.DelUserData("event", entityID, userID)
		},
	)
}

func DeleteFarm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteByField(w, r, ps, db.FarmsCollection, "id", "farmid", "farm", "farm-deleted", nil,
		func(ctx context.Context, entityID, userID string) {
			var farm models.Farm
			if err := db.FarmsCollection.FindOne(ctx, bson.M{"farmid": entityID}).Decode(&farm); err == nil {
				if farm.Banner != "" {
					_ = os.Remove("." + farm.Banner)
				}
			}
		},
	)
}

func DeleteCrop(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteByField(w, r, ps, db.CropsCollection, "cropid", "cropid", "crop", "crop-deleted", nil, nil)
}

func DeleteProduct(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteByField(w, r, ps, db.ProductCollection, "id", "productid", "product", "farmitem-deleted", nil, nil)
}

func DeleteTool(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	DeleteProduct(w, r, ps) // alias
}

func DeleteMerch(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteByField(w, r, ps, db.MerchCollection, "merchid", "merchid", "merch", "merch-deleted", nil,
		func(ctx context.Context, entityID, userID string) {
			rdx.RdxDel("merch:" + entityID)
		},
	)
}

func DeleteTicket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteByField(w, r, ps, db.TicketsCollection, "ticketid", "ticketid", "ticket", "ticket-deleted", nil, nil)
}

func DeleteReview(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteByField(w, r, ps, db.ReviewsCollection, "reviewId", "reviewid", "review", "review-deleted",
		func(ctx context.Context, r *http.Request, entityID string) error {
			userID, _ := r.Context().Value(globals.UserIDKey).(string)
			var rev models.Review
			if err := db.ReviewsCollection.FindOne(ctx, bson.M{"reviewid": entityID}).Decode(&rev); err != nil {
				return fmt.Errorf("not found")
			}
			if rev.UserID != userID && !isAdmin(ctx) {
				return fmt.Errorf("forbidden")
			}
			return nil
		},
		func(ctx context.Context, entityID, userID string) {
			userdata.DelUserData("review", entityID, userID)
		},
	)
}

func DeleteMedia(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteByField(w, r, ps, db.MediaCollection, "id", "mediaid", "media", "media-deleted", nil,
		func(ctx context.Context, entityID, userID string) {
			userdata.DelUserData("media", entityID, userID)
		},
	)
}

func DeletePlace(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteByField(w, r, ps, db.PlacesCollection, "placeid", "placeid", "place", "place-deleted",
		func(ctx context.Context, r *http.Request, entityID string) error {
			userID, _ := r.Context().Value(globals.UserIDKey).(string)
			var place models.Place
			if err := db.PlacesCollection.FindOne(ctx, bson.M{"placeid": entityID}).Decode(&place); err != nil {
				return fmt.Errorf("not found")
			}
			if place.CreatedBy != userID {
				return fmt.Errorf("forbidden")
			}
			return nil
		},
		func(ctx context.Context, entityID, userID string) {
			rdx.RdxDel("place:" + entityID)
			userdata.DelUserData("place", entityID, userID)
		},
	)
}

func DeleteMenu(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteByField(w, r, ps, db.MenuCollection, "menuid", "menuid", "menu", "menu-deleted", nil,
		func(ctx context.Context, entityID, userID string) {
			rdx.RdxDel("menu:" + entityID)
		},
	)
}

func DeleteProfile(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteByField(w, r, ps, db.UserCollection, "id", "userid", "profile", "profile-deleted",
		func(ctx context.Context, r *http.Request, entityID string) error {
			claims, err := middleware.ValidateJWT(r.Header.Get("Authorization"))
			if err != nil {
				return fmt.Errorf("unauthorized")
			}
			if claims.UserID != entityID {
				return fmt.Errorf("forbidden")
			}
			return nil
		},
		func(ctx context.Context, entityID, userID string) {
			InvalidateCachedProfile(userID)
		},
	)
}

func DeleteArtistByID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	softDeleteByField(w, r, ps, db.ArtistsCollection, "id", "artistid", "artist", "artist-deleted",
		bson.M{"$set": bson.M{"deleted": true}}, nil, nil)
}

func DeleteSong(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteByField(w, r, ps, db.SongsCollection, "songId", "songid", "song", "song-deleted", nil, nil)
}

func DeleteArtistEvent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteByField(w, r, ps, db.ArtistEventsCollection, "id", "eventid", "artistevent", "artistevent-deleted", nil, nil)
}

func DeleteItinerary(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	softDeleteByField(w, r, ps, db.ItineraryCollection, "id", "itineraryid", "itinerary", "itinerary-deleted",
		bson.M{"$set": bson.M{"deleted": true}},
		func(ctx context.Context, r *http.Request, entityID string) error {
			userID := utils.GetUserIDFromRequest(r)
			var itin models.Itinerary
			if err := db.ItineraryCollection.FindOne(ctx, bson.M{"itineraryid": entityID}).Decode(&itin); err != nil {
				return fmt.Errorf("not found")
			}
			if itin.UserID != userID {
				return fmt.Errorf("forbidden")
			}
			return nil
		}, nil)
}

func DeletePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	deleteByField(w, r, ps, db.PostsCollection, "postid", "postid", "feedpost", "post-deleted", nil,
		func(ctx context.Context, postID, userID string) {
			var existingFile models.FileMetadata
			db.FilesCollection.FindOne(context.TODO(), bson.M{"postid": postID}).Decode(&existingFile)
			RemoveUserFile(userID, postID, existingFile.Hash)
			userdata.DelUserData("feedpost", postID, userID)
		},
	)
}

// ---- Helpers ----

func deleteRelatedData(eventID string) error {
	_, err := db.TicketsCollection.DeleteMany(context.TODO(), bson.M{"eventid": eventID})
	if err != nil {
		return err
	}
	_, err = db.MediaCollection.DeleteMany(context.TODO(), bson.M{"eventid": eventID})
	if err != nil {
		return err
	}
	_, err = db.MerchCollection.DeleteMany(context.TODO(), bson.M{"eventid": eventID})
	if err != nil {
		return err
	}
	_, err = db.ArtistEventsCollection.DeleteOne(context.TODO(), bson.M{"eventid": eventID})
	return err
}

func InvalidateCachedProfile(username string) error {
	_, err := rdx.RdxDel("profile:" + username)
	return err
}

func RemoveUserFile(userID, postID, hash string) {
	result, err := db.FilesCollection.UpdateOne(
		context.TODO(),
		bson.M{"hash": hash},
		bson.M{"$pull": bson.M{"userPosts." + userID: postID}},
	)
	if err != nil || result.MatchedCount == 0 {
		return
	}
	var file models.FileMetadata
	if err := db.FilesCollection.FindOne(context.TODO(), bson.M{"hash": hash}).Decode(&file); err == nil {
		isPostAssociated := false
		for _, posts := range file.UserPosts {
			if slices.Contains(posts, postID) {
				isPostAssociated = true
				break
			}
		}
		if !isPostAssociated {
			_, _ = db.FilesCollection.UpdateOne(
				context.TODO(),
				bson.M{"hash": hash},
				bson.M{"$unset": bson.M{"postUrls." + postID: ""}},
			)
		}
		if len(file.UserPosts) == 0 {
			os.Remove(file.PostURLs[postID])
			db.FilesCollection.DeleteOne(context.TODO(), bson.M{"hash": hash})
		}
	}
}

func isAdmin(ctx context.Context) bool {
	role, ok := ctx.Value(roleKey).(string)
	return ok && role == "admin"
}

const roleKey = "role"
