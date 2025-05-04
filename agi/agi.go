package agi

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
)

var sampleData = map[string][]map[string]string{
	"recommended_events": {
		{"title": "🎤 Jazz Night", "location": "Moonlight Bar"},
		{"title": "⚽ Football Match", "location": "National Stadium"},
		{"title": "🎨 Art Exhibition", "location": "City Gallery"},
		{"title": "🎭 Broadway Show", "location": "Grand Theatre"},
		{"title": "🏁 Formula 1 Race", "location": "Speedway"},
		{"title": "🎬 Film Festival", "location": "Downtown Cinema"},
		{"title": "💼 Networking Mixer", "location": "Business Lounge"},
		{"title": "🎮 Esports Tournament", "location": "Gaming Arena"},
		{"title": "🏋️ Fitness Bootcamp", "location": "Central Park"},
		{"title": "🚀 Space Science Expo", "location": "Tech Convention Center"},
	},
	"recommended_places": {
		{"name": "🍣 Sushi Express", "location": "City Center"},
		{"name": "🏞️ Mountain View Retreat", "location": "Countryside"},
		{"name": "🎳 Bowling Alley", "location": "Mall Area"},
		{"name": "📚 Cozy Bookstore", "location": "Old Town"},
		{"name": "🏖️ Beachside Lounge", "location": "Seafront"},
		{"name": "🎶 Live Jazz Cafe", "location": "Downtown"},
		{"name": "🛍️ Premium Shopping Mall", "location": "Commercial District"},
		{"name": "🚴 Cycling Trail", "location": "Nature Park"},
		{"name": "⛩️ Zen Garden", "location": "Temple Grounds"},
		{"name": "🌮 Street Food Market", "location": "City Center"},
	},
	"followed_posts": {
		{"user": "Daniel", "content": "Loved the energy at the football match!"},
		{"user": "Sophia", "content": "That jazz night was unforgettable."},
		{"user": "Michael", "content": "Esports finals were insane!"},
		{"user": "Emma", "content": "Just discovered this amazing sushi place!"},
		{"user": "Alex", "content": "The art exhibition was stunning."},
		{"user": "Olivia", "content": "Bowling night with friends was a blast!"},
		{"user": "James", "content": "Great networking at the tech mixer."},
		{"user": "Charlotte", "content": "The Formula 1 race was thrilling!"},
		{"user": "Henry", "content": "Spent the weekend at Mountain View Retreat, so peaceful."},
		{"user": "Isabella", "content": "Bookstore vibes are unmatched."},
	},
	"ads": {
		{"title": "🚗 Car Rental Discounts!", "description": "Exclusive rates for festival attendees."},
		{"title": "🏨 Hotel Flash Sale!", "description": "Book now & save up to 50%."},
		{"title": "📢 Special Tech Offer!", "description": "Early access to the latest gadgets."},
		{"title": "🍽️ Free Dessert!", "description": "Get a free dessert with any meal at Sushi Express."},
		{"title": "🏋️ Gym Membership Deal!", "description": "Sign up this month & get a free trainer session."},
		{"title": "🎧 Music Streaming Discount!", "description": "Enjoy premium membership at half price."},
		{"title": "📸 Photography Workshop!", "description": "Learn expert techniques at a special price."},
		{"title": "🖼️ Art Supplies Sale!", "description": "Get 30% off top-tier art tools."},
		{"title": "🎟️ Exclusive Concert Access!", "description": "Buy 1 ticket, get 1 free!"},
	},
}

// // Sample data for personalization
// var sampleData = map[string][]map[string]string{
// 	"recommended_events": {
// 		{"title": "🎉 Music Festival", "location": "City Park"},
// 		{"title": "📅 Tech Conference", "location": "Convention Center"},
// 		{"title": "🎭 Theatre Night", "location": "Grand Theatre"},
// 		{"title": "🎸 Rock Concert", "location": "Downtown Arena"},
// 		{"title": "💻 Hackathon", "location": "Tech Hub"},
// 	},
// 	"recommended_places": {
// 		{"name": "🌅 Sunset Cafe", "location": "Downtown"},
// 		{"name": "🏞️ River Walk", "location": "City Outskirts"},
// 		{"name": "☕ Coffee Corner", "location": "City Center"},
// 		{"name": "🍴 Diner Delight", "location": "Uptown"},
// 	},
// 	"followed_posts": {
// 		{"user": "Alice", "content": "Had an amazing time at the festival!"},
// 		{"user": "Bob", "content": "Tech Conference was insightful!"},
// 		{"user": "Charlie", "content": "The theatre show was mesmerizing."},
// 		{"user": "Alice", "content": "Rock Concert rocked my world!"},
// 	},
// 	"ads": {
// 		{"title": "🔥 Special Discount at Sunset Cafe!", "description": "Get 20% off your next visit."},
// 		{"title": "🎟️ Early Bird Offer!", "description": "Book your tickets now for the upcoming festival."},
// 	},
// }

// Shuffle function
func shuffleList(data []map[string]string) {
	rand.Shuffle(len(data), func(i, j int) { data[i], data[j] = data[j], data[i] })
}

// Paginate and randomize data
func paginateList(data []map[string]string, page, itemsPerPage int) []map[string]string {
	shuffleList(data) // Shuffle data each time

	start := (page - 1) * itemsPerPage
	end := start + itemsPerPage

	if start >= len(data) {
		return []map[string]string{} // Return empty if out of range
	}

	if end > len(data) {
		end = len(data)
	}
	return data[start:end]
}

// GetHomeFeed handles the home feed API
func GetHomeFeed(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")

	// Parse request body
	var requestData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, `{"error": "Invalid JSON"}`, http.StatusBadRequest)
		return
	}

	// Extract user_id, section, and page
	userID, ok := requestData["user_id"].(string)
	if !ok || strings.TrimSpace(userID) == "" {
		http.Error(w, `{"error": "User ID is required"}`, http.StatusBadRequest)
		return
	}

	section, ok := requestData["section"].(string)
	if !ok || strings.TrimSpace(section) == "" {
		http.Error(w, `{"error": "Invalid section"}`, http.StatusBadRequest)
		return
	}

	page := 1 // Default page = 1
	if p, ok := requestData["page"].(float64); ok {
		page = int(p)
	}

	// Check if section exists
	data, exists := sampleData[section]
	if !exists {
		http.Error(w, `{"error": "Invalid section"}`, http.StatusBadRequest)
		return
	}

	// Paginate and return randomized data
	itemsPerPage := 6
	response := paginateList(data, page, itemsPerPage)

	json.NewEncoder(w).Encode(response)
}
