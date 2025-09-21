package analytics

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// Analytics represents generic analytics data for any entity
type Analytics struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Type         string         `json:"type"` // "event", "place", "product", etc.
	Metrics      map[string]int `json:"metrics"`
	Trend        []int          `json:"trend"`        // last 7 days
	TopLocations []string       `json:"topLocations"` // meaningful for places
}

// --- Handlers for specific entity types ---

func getEventAnalytics(entityID string) Analytics {
	return Analytics{
		ID:   entityID,
		Name: "Sample Event " + entityID,
		Type: "event",
		Metrics: map[string]int{
			"total":    120,
			"views":    300,
			"attended": 80,
			"rsvps":    60,
		},
		Trend: []int{12, 18, 22, 20, 25, 30, 28},
	}
}

func getPlaceAnalytics(entityID string) Analytics {
	return Analytics{
		ID:   entityID,
		Name: "Sample Place " + entityID,
		Type: "place",
		Metrics: map[string]int{
			"total":     100,
			"views":     250,
			"favorites": 40,
			"attended":  75,
		},
		Trend:        []int{10, 15, 20, 18, 25, 30, 28},
		TopLocations: []string{"Downtown", "Uptown", "Central Park"},
	}
}

func getProductAnalytics(entityID string) Analytics {
	return Analytics{
		ID:   entityID,
		Name: "Sample Product " + entityID,
		Type: "product",
		Metrics: map[string]int{
			"totalSold":    500,
			"views":        1200,
			"favorites":    300,
			"reviews":      45,
			"availableQty": 120,
		},
		Trend: []int{50, 60, 70, 65, 80, 90, 85},
	}
}

// --- Delegator handler ---

func GetEntityAnalytics(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	entityType := ps.ByName("entityType")
	entityID := ps.ByName("entityId")

	var analytics Analytics

	switch entityType {
	case "events":
		analytics = getEventAnalytics(entityID)
	case "places":
		analytics = getPlaceAnalytics(entityID)
	case "product":
		analytics = getProductAnalytics(entityID)
	default:
		http.Error(w, "Invalid entity type", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(analytics)
}
