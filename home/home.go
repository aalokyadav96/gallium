package home

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
)

var routeHandlers = map[string]func() (interface{}, error){
	"news":    wrap(getNews),
	"trends":  wrap(getTrends),
	"events":  wrap(getEvents),
	"places":  wrap(getPlaces),
	"posts":   wrap(getCommunityPosts),
	"media":   wrap(getMedia),
	"notices": wrap(getNotices),
}

func wrap[T any](fn func() (T, error)) func() (any, error) {
	return func() (any, error) {
		return fn()
	}
}

func GetHomeContent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	apiRoute := strings.ToLower(ps.ByName("apiRoute"))

	handler, ok := routeHandlers[apiRoute]
	if !ok {
		http.Error(w, `{"error":"Invalid API route"}`, http.StatusNotFound)
		return
	}

	data, err := handler()
	if err != nil {
		log.Printf("Error fetching %s: %v", apiRoute, err)
		http.Error(w, `{"error":"Internal Server Error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60")
	json.NewEncoder(w).Encode(data)
}

func getNews() ([]map[string]string, error) {
	return []map[string]string{
		{"title": "New Event System Launch", "link": "/news/launch"},
		{"title": "Platform v2.0 Announced", "link": "/news/v2"},
	}, nil
}

func getTrends() ([]string, error) {
	return []string{"#Nightlife", "#Foodie", "#LiveMusic", "#Workspaces"}, nil
}

func getEvents() ([]map[string]string, error) {
	return []map[string]string{
		{"title": "Tech Conference 2025", "link": "/events/techconf"},
		{"title": "DJ Night at Club XO", "link": "/events/djnight"},
	}, nil
}

func getPlaces() ([]map[string]string, error) {
	return []map[string]string{
		{"name": "Cafe Mocha", "link": "/places/cafe-mocha"},
		{"name": "Studio 88", "link": "/places/studio-88"},
	}, nil
}

func getCommunityPosts() ([]map[string]string, error) {
	return []map[string]string{
		{"title": "Had an amazing time at the festival!", "link": "/posts/festival123"},
		{"title": "Anyone been to Rooftop Blues?", "link": "/posts/rooftop"},
	}, nil
}

func getMedia() ([]map[string]string, error) {
	return []map[string]string{
		{"url": "/media/img1.jpg", "alt": "Concert Crowd"},
		{"url": "/media/img2.jpg", "alt": "Venue Interior"},
	}, nil
}

func getNotices() ([]map[string]string, error) {
	return []map[string]string{
		{"text": "Maintenance on 12th June from 2 AM to 5 AM"},
		{"text": "New features rolling out next week"},
	}, nil
}
