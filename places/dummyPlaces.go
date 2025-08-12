package places

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

type Place struct {
	Icon          string   `json:"icon"`
	Type          string   `json:"type"`
	Name          string   `json:"name"`
	Desc          string   `json:"desc"`
	Tags          []string `json:"tags"`
	Distance      string   `json:"distance"`
	IsAccent      bool     `json:"isAccent"`
	IsPlaceholder bool     `json:"isPlaceholder"`
}

func GetDummyPlaces(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	places := []Place{
		{Icon: "☕", Type: "Cafés", Name: "Ground & Grind", Desc: "Cozy specialty coffee, light pastries — 2 min walk", Tags: []string{"Coffee", "Wi-Fi", "Open"}, Distance: "0.2 km", IsAccent: true},
		{Icon: "🌳", Type: "Parks", Name: "Riverbend Park", Desc: "Riverside trails and weekend farmers' market", Tags: []string{"Park", "Market"}, Distance: "0.6 km"},
		{Icon: "🔧", Type: "Events", Name: "Little Foundry", Desc: "Community makerspace — life drawing every Thu", Tags: []string{"Makerspace", "Events"}, Distance: "0.9 km"},
		{Icon: "🏪", Type: "Shops", Name: "Local Shop", Desc: "Independent store", Tags: []string{"Shop"}, Distance: "1.3 km", IsPlaceholder: true},
	}

	filterType := r.URL.Query().Get("type")

	var filtered []Place
	if filterType != "" && filterType != "All" {
		for _, p := range places {
			if p.Type == filterType {
				filtered = append(filtered, p)
			}
		}
	} else {
		filtered = places
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filtered)
}

// func GetDummyPlaces(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
// 	places := []Place{
// 		{Icon: "☕", Type: "Café", Name: "Ground & Grind", Desc: "Cozy specialty coffee, light pastries — 2 min walk", Tags: []string{"Coffee", "Wi-Fi", "Open"}, Distance: "0.2 km", IsAccent: true},
// 		{Icon: "🌳", Type: "Park", Name: "Riverbend Park", Desc: "Riverside trails and weekend farmers' market", Tags: []string{"Park", "Market"}, Distance: "0.6 km"},
// 		{Icon: "🔧", Type: "Makerspace", Name: "Little Foundry", Desc: "Community makerspace — life drawing every Thu", Tags: []string{"Makerspace", "Events"}, Distance: "0.9 km"},
// 		{Icon: "🏪", Type: "Shop", Name: "Local Shop", Desc: "Independent store", Tags: []string{"Shop"}, Distance: "1.3 km", IsPlaceholder: true},
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(places)
// }
