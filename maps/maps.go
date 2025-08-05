package maps

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func GetMapConfig(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var config struct {
		MapImage        string            `json:"mapImage"`
		SpritePositions map[string]string `json:"spritePositions"`
		TypeLabels      map[string]string `json:"typeLabels"`
	}

	config.MapImage = "/map.jpg"

	// ✅ Initialize maps before assignment
	config.SpritePositions = map[string]string{
		"petrol":     "0px 0px",
		"shop":       "0px -32px",
		"hospital":   "0px -64px",
		"barber":     "0px -96px",
		"restaurant": "0px -128px",
	}

	config.TypeLabels = map[string]string{
		"petrol":     "⛽ Petrol Pump",
		"shop":       "🏪 Shop",
		"hospital":   "🏥 Hospital",
		"barber":     "✂️ Barber",
		"restaurant": "🍔 Restaurant",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

func GetMapMarkers(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	res := r.URL.Query()
	fmt.Println(res)
	type Marker struct {
		ID   string `json:"id" bson:"_id"`
		Name string `json:"name"`
		Type string `json:"type"`
		X    int    `json:"x"`
		Y    int    `json:"y"`
	}

	var markers []Marker = make([]Marker, 6)

	markers[0] = Marker{ID: "AI6tul8sOvWCaC", Name: "Pump 1", Type: "petrol", X: 100, Y: 100}
	markers[1] = Marker{ID: "AI6tul8sOvWCaC", Name: "Shop 1", Type: "shop", X: 400, Y: 550}
	markers[2] = Marker{ID: "AI6tul8sOvWCaC", Name: "Pump 2", Type: "petrol", X: 100, Y: 750}
	markers[3] = Marker{ID: "AI6tul8sOvWCaC", Name: "Hospital", Type: "hospital", X: 800, Y: 600}
	markers[4] = Marker{ID: "AI6tul8sOvWCaC", Name: "Barber", Type: "police", X: 600, Y: 600}
	markers[5] = Marker{ID: "AI6tul8sOvWCaC", Name: "Burger Place", Type: "restaurant", X: 700, Y: 450}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(markers)
}
