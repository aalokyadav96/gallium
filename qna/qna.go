package qna

import (
	"encoding/json"
	"naevis/db"
	"naevis/models"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func ListQuestions(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	cur, _ := db.QuestionCollection.Find(r.Context(), bson.M{})
	var questions []models.Question
	cur.All(r.Context(), &questions)

	if len(questions) == 0 {
		questions = []models.Question{}
	}

	json.NewEncoder(w).Encode(questions)
}

func GetQuestionByID(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	objID, _ := primitive.ObjectIDFromHex(id)
	var q models.Question
	err := db.QuestionCollection.FindOne(r.Context(), bson.M{"_id": objID}).Decode(&q)
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	json.NewEncoder(w).Encode(q)
}

func CreateQuestion(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var q models.Question
	json.NewDecoder(r.Body).Decode(&q)
	q.Timestamp = time.Now()
	res, _ := db.QuestionCollection.InsertOne(r.Context(), q)
	q.ID = res.InsertedID.(primitive.ObjectID).Hex()
	json.NewEncoder(w).Encode(q)
}

func ListAnswersByPostID(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	postid := r.URL.Query().Get("postid")
	cur, _ := db.AnswerCollection.Find(r.Context(), bson.M{"postid": postid})
	var answers []models.Answer
	cur.All(r.Context(), &answers)
	json.NewEncoder(w).Encode(answers)
}

func CreateAnswer(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var a models.Answer
	json.NewDecoder(r.Body).Decode(&a)
	a.Timestamp = time.Now()
	a.Replies = []string{}
	res, _ := db.AnswerCollection.InsertOne(r.Context(), a)
	a.ID = res.InsertedID.(primitive.ObjectID).Hex()
	json.NewEncoder(w).Encode(a)
}

func VoteAnswer(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	var payload struct{ Type string }
	json.NewDecoder(r.Body).Decode(&payload)

	objID, _ := primitive.ObjectIDFromHex(id)
	field := "$inc"
	var update bson.M
	if payload.Type == "up" {
		update = bson.M{field: bson.M{"votes": 1}}
	} else {
		update = bson.M{field: bson.M{"downvotes": 1}}
	}
	db.AnswerCollection.UpdateByID(r.Context(), objID, update)
	w.WriteHeader(204)
}

func MarkBestAnswer(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	objID, _ := primitive.ObjectIDFromHex(id)

	// unset existing best for this question
	var a models.Answer
	db.AnswerCollection.FindOne(r.Context(), bson.M{"_id": objID}).Decode(&a)
	db.AnswerCollection.UpdateMany(r.Context(), bson.M{"postid": a.PostID}, bson.M{"$set": bson.M{"isBest": false}})

	db.AnswerCollection.UpdateByID(r.Context(), objID, bson.M{"$set": bson.M{"isBest": true}})
	w.WriteHeader(204)
}

func ReportAnswer(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Just acknowledge for now, no storage logic
	w.WriteHeader(204)
}

func CreateReply(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var rep struct {
		AnswerId string `json:"answerId"`
		Content  string `json:"content"`
	}
	json.NewDecoder(r.Body).Decode(&rep)

	objID, _ := primitive.ObjectIDFromHex(rep.AnswerId)
	db.AnswerCollection.UpdateByID(r.Context(), objID, bson.M{
		"$push": bson.M{"replies": rep.Content},
	})
	w.WriteHeader(204)
}
