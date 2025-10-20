package models

import "time"

type Event struct {
	EventID          string      `json:"eventid" bson:"eventid"`
	Title            string      `json:"title" bson:"title"`
	Description      string      `json:"description" bson:"description"`
	Date             time.Time   `json:"date" bson:"date"`
	PlaceID          string      `json:"placeid" bson:"placeid"`
	PlaceName        string      `json:"placename" bson:"placename"`
	Location         string      `json:"location" bson:"location"`
	Coords           Coordinates `json:"coords" bson:"coords"`
	CreatorID        string      `json:"creatorid" bson:"creatorid"`
	Tickets          []Ticket    `json:"tickets" bson:"tickets"`
	Merch            []Merch     `json:"merch" bson:"merch"`
	StartDateTime    time.Time   `json:"start_date_time" bson:"start_date_time"`
	EndDateTime      time.Time   `json:"end_date_time" bson:"end_date_time"`
	Category         string      `json:"category" bson:"category"`
	Banner           string      `json:"banner" bson:"banner"`
	SeatingPlanImage string      `json:"seating" bson:"seating"`
	WebsiteURL       string      `json:"website_url" bson:"website_url"`
	Status           string      `json:"status" bson:"status"`
	Tags             []string    `json:"tags" bson:"tags"`
	CreatedAt        time.Time   `json:"created_at" bson:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at" bson:"updated_at"`
	FAQs             []FAQ       `json:"faqs" bson:"faqs"`
	OrganizerName    string      `json:"organizer_name" bson:"organizer_name"`
	OrganizerContact string      `json:"organizer_contact" bson:"organizer_contact"`
	Artists          []string    `json:"artists,omitempty" bson:"artists,omitempty"`
	Published        string      `json:"published,omitempty" bson:"published,omitempty"`

	// Computed fields for frontend filters
	Prices   []float64 `json:"prices,omitempty" bson:"-"`
	Currency string    `json:"currency,omitempty" bson:"-"`
}

// FAQ represents a single FAQ structure
type FAQ struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type SocialMediaLinks struct {
	Title string `json:"title"`
	Url   string `json:"Url"`
}

type PurchasedTicket struct {
	EventID      string
	TicketID     string
	UserID       string
	BuyerName    string
	UniqueCode   string
	PurchaseDate time.Time
}
