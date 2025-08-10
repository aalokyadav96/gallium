// package models

// import (
// 	"time"
// )

// type User struct {
// 	UserID         string            `json:"user_id" bson:"user_id"`
// 	Username       string            `json:"username" bson:"username"`
// 	Email          string            `json:"email" bson:"email"`
// 	PasswordHash   string            `json:"password_hash" bson:"password_hash"`
// 	Role           []string          `json:"role,omitempty" bson:"role,omitempty"`
// 	Name           string            `json:"name,omitempty" bson:"name,omitempty"`
// 	Bio            string            `json:"bio,omitempty" bson:"bio,omitempty"`
// 	ProfilePicture string            `json:"profile_picture,omitempty" bson:"profile_picture,omitempty"`
// 	BannerPicture  string            `json:"banner_picture,omitempty" bson:"banner_picture,omitempty"`
// 	PhoneNumber    string            `json:"phone_number,omitempty" bson:"phone_number,omitempty"`
// 	Address        string            `json:"address,omitempty" bson:"address,omitempty"`
// 	SocialLinks    map[string]string `json:"social_links,omitempty" bson:"social_links,omitempty"`
// 	IsVerified     bool              `json:"is_verified,omitempty" bson:"is_verified,omitempty"`
// 	EmailVerified  bool              `json:"email_verified,omitempty" bson:"email_verified,omitempty"`
// 	FollowersCount int               `json:"followers_count,omitempty" bson:"followers_count,omitempty"`
// 	FollowingCount int               `json:"following_count,omitempty" bson:"following_count,omitempty"`
// 	ProfileViews   int               `json:"profile_views,omitempty" bson:"profile_views,omitempty"`
// 	Online         bool              `json:"online,omitempty" bson:"online,omitempty"`
// 	LastLogin      time.Time         `json:"last_login,omitempty" bson:"last_login,omitempty"`
// 	CreatedAt      time.Time         `json:"created_at,omitempty" bson:"created_at,omitempty"`
// 	UpdatedAt      time.Time         `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
// }

// type UserProfileResponse struct {
// 	UserID         string            `json:"user_id" bson:"user_id"`
// 	Username       string            `json:"username" bson:"username"`
// 	Name           string            `json:"name,omitempty" bson:"name,omitempty"`
// 	Email          string            `json:"email,omitempty" bson:"email,omitempty"`
// 	Bio            string            `json:"bio,omitempty" bson:"bio,omitempty"`
// 	PhoneNumber    string            `json:"phone_number,omitempty" bson:"phone_number,omitempty"`
// 	ProfilePicture string            `json:"profile_picture,omitempty" bson:"profile_picture,omitempty"`
// 	BannerPicture  string            `json:"banner_picture,omitempty" bson:"banner_picture,omitempty"`
// 	IsFollowing    bool              `json:"is_following,omitempty" bson:"is_following,omitempty"`
// 	FollowersCount int               `json:"followers_count,omitempty" bson:"followers_count,omitempty"`
// 	FollowingCount int               `json:"following_count,omitempty" bson:"following_count,omitempty"`
// 	SocialLinks    map[string]string `json:"social_links,omitempty" bson:"social_links,omitempty"`
// 	Online         bool              `json:"online,omitempty" bson:"online,omitempty"`
// 	LastLogin      time.Time         `json:"last_login,omitempty" bson:"last_login,omitempty"`
// }

// type UserFollow struct {
// 	UserID    string   `json:"user_id" bson:"user_id"`
// 	Follows   []string `json:"follows,omitempty" bson:"follows,omitempty"`
// 	Followers []string `json:"followers,omitempty" bson:"followers,omitempty"`
// }

// type Response struct {
// 	Message string      `json:"message,omitempty"`
// 	Data    interface{} `json:"data,omitempty"`
// 	Error   string      `json:"error,omitempty"`
// }

// type Setting struct {
// 	Type        string      `json:"type" bson:"type"`
// 	Value       interface{} `json:"value" bson:"value"`
// 	Description string      `json:"description,omitempty" bson:"description,omitempty"`
// }

// type FeedPost struct {
// 	PostID      string    `json:"post_id,omitempty" bson:"post_id,omitempty"`
// 	UserID      string    `json:"user_id" bson:"user_id"`
// 	Username    string    `json:"username" bson:"username"`
// 	Text        string    `json:"text,omitempty" bson:"text,omitempty"`
// 	Type        string    `json:"type" bson:"type"`
// 	Media       []string  `json:"media,omitempty" bson:"media,omitempty"`
// 	Content     string    `json:"content,omitempty" bson:"content,omitempty"`
// 	MediaURL    []string  `json:"media_url,omitempty" bson:"media_url,omitempty"`
// 	Resolutions []int     `json:"resolutions,omitempty" bson:"resolutions,omitempty"`
// 	Likes       int       `json:"likes,omitempty" bson:"likes,omitempty"`
// 	Likers      []string  `json:"likers,omitempty" bson:"likers,omitempty"`
// 	Timestamp   time.Time `json:"timestamp" bson:"timestamp"`
// 	CreatedAt   time.Time `json:"created_at" bson:"created_at"`
// }

// type Activity struct {
// 	ID           string    `json:"id,omitempty" bson:"_id,omitempty"`
// 	UserID       string    `json:"user_id" bson:"user_id"`
// 	PlaceID      string    `json:"place_id,omitempty" bson:"place_id,omitempty"`
// 	Action       string    `json:"action,omitempty" bson:"action,omitempty"`
// 	PerformedBy  string    `json:"performed_by,omitempty" bson:"performed_by,omitempty"`
// 	Details      string    `json:"details,omitempty" bson:"details,omitempty"`
// 	IPAddress    string    `json:"ip_address,omitempty" bson:"ip_address,omitempty"`
// 	DeviceInfo   string    `json:"device_info,omitempty" bson:"device_info,omitempty"`
// 	ActivityType string    `json:"activity_type,omitempty" bson:"activity_type,omitempty"`
// 	EntityID     string    `json:"entity_id,omitempty" bson:"entity_id,omitempty"`
// 	EntityType   string    `json:"entity_type,omitempty" bson:"entity_type,omitempty"`
// 	Timestamp    time.Time `json:"timestamp,omitempty" bson:"timestamp,omitempty"`
// }

// type Merch struct {
// 	MerchID     string    `json:"merch_id" bson:"merch_id"`
// 	Name        string    `json:"name" bson:"name"`
// 	Slug        string    `json:"slug,omitempty" bson:"slug,omitempty"`
// 	SKU         string    `json:"sku,omitempty" bson:"sku,omitempty"`
// 	Category    string    `json:"category,omitempty" bson:"category,omitempty"`
// 	Price       float64   `json:"price" bson:"price"`
// 	Discount    float64   `json:"discount,omitempty" bson:"discount,omitempty"`
// 	Stock       int       `json:"stock" bson:"stock"`
// 	StockStatus string    `json:"stock_status,omitempty" bson:"stock_status,omitempty"`
// 	MerchPhoto  string    `json:"merch_photo,omitempty" bson:"merch_photo,omitempty"`
// 	Gallery     []string  `json:"gallery,omitempty" bson:"gallery,omitempty"`
// 	EntityID    string    `json:"entity_id" bson:"entity_id"`
// 	EntityType  string    `json:"entity_type" bson:"entity_type"`
// 	Description string    `json:"description,omitempty" bson:"description,omitempty"`
// 	ShortDesc   string    `json:"short_desc,omitempty" bson:"short_desc,omitempty"`
// 	Rating      float64   `json:"rating,omitempty" bson:"rating,omitempty"`
// 	ReviewCount int       `json:"review_count,omitempty" bson:"review_count,omitempty"`
// 	Weight      float64   `json:"weight,omitempty" bson:"weight,omitempty"`
// 	Dimensions  string    `json:"dimensions,omitempty" bson:"dimensions,omitempty"`
// 	Tags        []string  `json:"tags,omitempty" bson:"tags,omitempty"`
// 	UserID      string    `json:"user_id,omitempty" bson:"user_id,omitempty"`
// 	CreatedAt   time.Time `json:"created_at" bson:"created_at"`
// 	UpdatedAt   time.Time `json:"updated_at" bson:"updated_at"`
// }
// type Menu struct {
// 	MenuID      string    `json:"menu_id" bson:"menu_id"`
// 	PlaceID     string    `json:"place_id" bson:"place_id"`
// 	Name        string    `json:"name" bson:"name"`
// 	Price       float64   `json:"price" bson:"price"`
// 	Stock       int       `json:"stock" bson:"stock"`
// 	MenuPhoto   string    `json:"menu_photo,omitempty" bson:"menu_photo,omitempty"`
// 	Description string    `json:"description,omitempty" bson:"description,omitempty"`
// 	UserID      string    `json:"user_id,omitempty" bson:"user_id,omitempty"`
// 	CreatedAt   time.Time `json:"created_at" bson:"created_at"`
// 	UpdatedAt   time.Time `json:"updated_at" bson:"updated_at"`
// }

// type Ticket struct {
// 	TicketID    string    `json:"ticket_id" bson:"ticket_id"`
// 	EventID     string    `json:"event_id" bson:"event_id"`
// 	Name        string    `json:"name" bson:"name"`
// 	Price       float64   `json:"price" bson:"price"`
// 	Currency    string    `json:"currency" bson:"currency"`
// 	Color       string    `json:"color,omitempty" bson:"color,omitempty"`
// 	Quantity    int       `json:"quantity" bson:"quantity"`
// 	EntityID    string    `json:"entity_id" bson:"entity_id"`
// 	EntityType  string    `json:"entity_type" bson:"entity_type"`
// 	Available   int       `json:"available,omitempty" bson:"available,omitempty"`
// 	Total       int       `json:"total,omitempty" bson:"total,omitempty"`
// 	Description string    `json:"description,omitempty" bson:"description,omitempty"`
// 	Sold        int       `json:"sold,omitempty" bson:"sold,omitempty"`
// 	SeatStart   int       `json:"seat_start,omitempty" bson:"seat_start,omitempty"`
// 	SeatEnd     int       `json:"seat_end,omitempty" bson:"seat_end,omitempty"`
// 	Seats       []string  `json:"seats,omitempty" bson:"seats,omitempty"`
// 	CreatedAt   time.Time `json:"created_at" bson:"created_at"`
// 	UpdatedAt   time.Time `json:"updated_at" bson:"updated_at"`
// }

// type Seat struct {
// 	SeatID     string `json:"seat_id,omitempty" bson:"seat_id,omitempty"`
// 	EntityID   string `json:"entity_id" bson:"entity_id"`
// 	EntityType string `json:"entity_type" bson:"entity_type"`
// 	SeatNumber string `json:"seat_number" bson:"seat_number"`
// 	UserID     string `json:"user_id,omitempty" bson:"user_id,omitempty"`
// 	Status     string `json:"status" bson:"status"`
// }

// type UserSuggest struct {
// 	Username    string `json:"username" bson:"username"`
// 	UserID      string `json:"user_id" bson:"user_id"`
// 	IsFollowing bool   `json:"is_following,omitempty" bson:"is_following,omitempty"`
// 	Bio         string `json:"bio,omitempty" bson:"bio,omitempty"`
// }

// type Suggestion struct {
// 	ID          string `json:"id,omitempty" bson:"_id,omitempty"`
// 	Type        string `json:"type" bson:"type"`
// 	Title       string `json:"title" bson:"title"`
// 	Description string `json:"description,omitempty" bson:"description,omitempty"`
// 	Name        string `json:"name,omitempty" bson:"name,omitempty"`
// }

// type Review struct {
// 	ReviewID    string    `json:"review_id" bson:"review_id"`
// 	EntityID    string    `json:"entity_id" bson:"entity_id"`
// 	EntityType  string    `json:"entity_type" bson:"entity_type"`
// 	UserID      string    `json:"user_id" bson:"user_id"`
// 	Comment     string    `json:"comment,omitempty" bson:"comment,omitempty"`
// 	Content     string    `json:"content,omitempty" bson:"content,omitempty"`
// 	Rating      int       `json:"rating" bson:"rating"`
// 	Likes       int       `json:"likes,omitempty" bson:"likes,omitempty"`
// 	Dislikes    int       `json:"dislikes,omitempty" bson:"dislikes,omitempty"`
// 	Attachments []string  `json:"attachments,omitempty" bson:"attachments,omitempty"`
// 	Date        time.Time `json:"date" bson:"date"`
// 	UpdatedAt   time.Time `json:"updated_at" bson:"updated_at"`
// 	CreatedAt   time.Time `json:"created_at" bson:"created_at"`
// }

// type Media struct {
// 	MediaID       string    `json:"media_id" bson:"media_id"`
// 	Type          string    `json:"type" bson:"type"`
// 	URL           string    `json:"url" bson:"url"`
// 	ThumbnailURL  string    `json:"thumbnail_url,omitempty" bson:"thumbnail_url,omitempty"`
// 	Caption       string    `json:"caption,omitempty" bson:"caption,omitempty"`
// 	Description   string    `json:"description,omitempty" bson:"description,omitempty"`
// 	CreatorID     string    `json:"creator_id" bson:"creator_id"`
// 	LikesCount    int       `json:"likes_count,omitempty" bson:"likes_count,omitempty"`
// 	CommentsCount int       `json:"comments_count,omitempty" bson:"comments_count,omitempty"`
// 	Visibility    string    `json:"visibility" bson:"visibility"`
// 	Tags          []string  `json:"tags,omitempty" bson:"tags,omitempty"`
// 	Duration      float64   `json:"duration,omitempty" bson:"duration,omitempty"`
// 	FileSize      int64     `json:"file_size,omitempty" bson:"file_size,omitempty"`
// 	MimeType      string    `json:"mime_type,omitempty" bson:"mime_type,omitempty"`
// 	IsFeatured    bool      `json:"is_featured,omitempty" bson:"is_featured,omitempty"`
// 	EntityID      string    `json:"entity_id,omitempty" bson:"entity_id,omitempty"`
// 	EntityType    string    `json:"entity_type,omitempty" bson:"entity_type,omitempty"`
// 	UserID        string    `json:"user_id,omitempty" bson:"user_id,omitempty"`
// 	CreatedAt     time.Time `json:"created_at" bson:"created_at"`
// 	UpdatedAt     time.Time `json:"updated_at" bson:"updated_at"`
// }

// type Coordinates struct {
// 	Latitude  float64 `json:"latitude,omitempty" bson:"latitude,omitempty"`
// 	Longitude float64 `json:"longitude,omitempty" bson:"longitude,omitempty"`
// }

// type Place struct {
// 	PlaceID           string            `json:"place_id" bson:"place_id"`
// 	Name              string            `json:"name" bson:"name"`
// 	Description       string            `json:"description,omitempty" bson:"description,omitempty"`
// 	Place             string            `json:"place,omitempty" bson:"place,omitempty"`
// 	Capacity          int               `json:"capacity,omitempty" bson:"capacity,omitempty"`
// 	Date              time.Time         `json:"date,omitempty" bson:"date,omitempty"`
// 	Address           string            `json:"address,omitempty" bson:"address,omitempty"`
// 	CreatedBy         string            `json:"created_by,omitempty" bson:"created_by,omitempty"`
// 	OrganizerName     string            `json:"organizer_name,omitempty" bson:"organizer_name,omitempty"`
// 	OrganizerContact  string            `json:"organizer_contact,omitempty" bson:"organizer_contact,omitempty"`
// 	Tickets           []Ticket          `json:"tickets,omitempty" bson:"tickets,omitempty"`
// 	Merch             []Merch           `json:"merch,omitempty" bson:"merch,omitempty"`
// 	StartDateTime     time.Time         `json:"start_date_time,omitempty" bson:"start_date_time,omitempty"`
// 	EndDateTime       time.Time         `json:"end_date_time,omitempty" bson:"end_date_time,omitempty"`
// 	Category          string            `json:"category,omitempty" bson:"category,omitempty"`
// 	Banner            string            `json:"banner,omitempty" bson:"banner,omitempty"`
// 	WebsiteURL        string            `json:"website_url,omitempty" bson:"website_url,omitempty"`
// 	Status            string            `json:"status,omitempty" bson:"status,omitempty"`
// 	AccessibilityInfo string            `json:"accessibility_info,omitempty" bson:"accessibility_info,omitempty"`
// 	SocialLinks       map[string]string `json:"social_links,omitempty" bson:"social_links,omitempty"`
// 	Tags              []string          `json:"tags,omitempty" bson:"tags,omitempty"`
// 	CustomFields      map[string]any    `json:"custom_fields,omitempty" bson:"custom_fields,omitempty"`
// 	City              string            `json:"city,omitempty" bson:"city,omitempty"`
// 	Country           string            `json:"country,omitempty" bson:"country,omitempty"`
// 	ZipCode           string            `json:"zip_code,omitempty" bson:"zip_code,omitempty"`
// 	Location          Coordinates       `json:"location,omitempty" bson:"location,omitempty"`
// 	Phone             string            `json:"phone,omitempty" bson:"phone,omitempty"`
// 	Website           string            `json:"website,omitempty" bson:"website,omitempty"`
// 	IsOpen            bool              `json:"is_open,omitempty" bson:"is_open,omitempty"`
// 	Distance          float64           `json:"distance,omitempty" bson:"distance,omitempty"`
// 	Views             int               `json:"views,omitempty" bson:"views,omitempty"`
// 	ReviewCount       int               `json:"review_count,omitempty" bson:"review_count,omitempty"`
// 	UpdatedBy         string            `json:"updated_by,omitempty" bson:"updated_by,omitempty"`
// 	DeletedAt         *time.Time        `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
// 	Amenities         []string          `json:"amenities,omitempty" bson:"amenities,omitempty"`
// 	Events            []string          `json:"events,omitempty" bson:"events,omitempty"`
// 	OperatingHours    []string          `json:"operating_hours,omitempty" bson:"operating_hours,omitempty"`
// 	Keywords          []string          `json:"keywords,omitempty" bson:"keywords,omitempty"`
// 	CreatedAt         time.Time         `json:"created_at" bson:"created_at"`
// 	UpdatedAt         time.Time         `json:"updated_at" bson:"updated_at"`
// }
// type Event struct {
// 	EventID       string      `json:"event_id" bson:"event_id"`
// 	Title         string      `json:"title" bson:"title"`
// 	Description   string      `json:"description,omitempty" bson:"description,omitempty"`
// 	Location      Coordinates `json:"location,omitempty" bson:"location,omitempty"`
// 	Address       string      `json:"address,omitempty" bson:"address,omitempty"`
// 	City          string      `json:"city,omitempty" bson:"city,omitempty"`
// 	Country       string      `json:"country,omitempty" bson:"country,omitempty"`
// 	ZipCode       string      `json:"zip_code,omitempty" bson:"zip_code,omitempty"`
// 	StartDateTime time.Time   `json:"start_date_time" bson:"start_date_time"`
// 	EndDateTime   time.Time   `json:"end_date_time" bson:"end_date_time"`
// 	Category      string      `json:"category,omitempty" bson:"category,omitempty"`
// 	Tags          []string    `json:"tags,omitempty" bson:"tags,omitempty"`
// 	CreatorID     string      `json:"creator_id,omitempty" bson:"creator_id,omitempty"`
// 	Banner        string      `json:"banner,omitempty" bson:"banner,omitempty"`
// 	Status        string      `json:"status,omitempty" bson:"status,omitempty"`
// 	Tickets       []Ticket    `json:"tickets,omitempty" bson:"tickets,omitempty"`
// 	CreatedAt     time.Time   `json:"created_at" bson:"created_at"`
// 	UpdatedAt     time.Time   `json:"updated_at" bson:"updated_at"`
// }

// type Gig struct {
// 	GigID       string    `json:"gig_id" bson:"gig_id"`
// 	Title       string    `json:"title" bson:"title"`
// 	Description string    `json:"description,omitempty" bson:"description,omitempty"`
// 	Price       float64   `json:"price" bson:"price"`
// 	Category    string    `json:"category,omitempty" bson:"category,omitempty"`
// 	Tags        []string  `json:"tags,omitempty" bson:"tags,omitempty"`
// 	UserID      string    `json:"user_id" bson:"user_id"`
// 	Attachments []string  `json:"attachments,omitempty" bson:"attachments,omitempty"`
// 	Rating      float64   `json:"rating,omitempty" bson:"rating,omitempty"`
// 	ReviewCount int       `json:"review_count,omitempty" bson:"review_count,omitempty"`
// 	CreatedAt   time.Time `json:"created_at" bson:"created_at"`
// 	UpdatedAt   time.Time `json:"updated_at" bson:"updated_at"`
// }

// type Booking struct {
// 	BookingID  string    `json:"booking_id" bson:"booking_id"`
// 	UserID     string    `json:"user_id" bson:"user_id"`
// 	EntityID   string    `json:"entity_id" bson:"entity_id"`
// 	EntityType string    `json:"entity_type" bson:"entity_type"`
// 	Quantity   int       `json:"quantity" bson:"quantity"`
// 	TotalPrice float64   `json:"total_price" bson:"total_price"`
// 	Status     string    `json:"status" bson:"status"`
// 	BookedAt   time.Time `json:"booked_at" bson:"booked_at"`
// 	UpdatedAt  time.Time `json:"updated_at" bson:"updated_at"`
// }

// type Promotion struct {
// 	PromotionID string    `json:"promotion_id" bson:"promotion_id"`
// 	Title       string    `json:"title" bson:"title"`
// 	Description string    `json:"description,omitempty" bson:"description,omitempty"`
// 	Discount    float64   `json:"discount" bson:"discount"`
// 	StartDate   time.Time `json:"start_date" bson:"start_date"`
// 	EndDate     time.Time `json:"end_date" bson:"end_date"`
// 	EntityID    string    `json:"entity_id" bson:"entity_id"`
// 	EntityType  string    `json:"entity_type" bson:"entity_type"`
// 	CreatedAt   time.Time `json:"created_at" bson:"created_at"`
// 	UpdatedAt   time.Time `json:"updated_at" bson:"updated_at"`
// }

// type Notification struct {
// 	NotificationID string    `json:"notification_id" bson:"notification_id"`
// 	UserID         string    `json:"user_id" bson:"user_id"`
// 	Title          string    `json:"title" bson:"title"`
// 	Message        string    `json:"message" bson:"message"`
// 	Type           string    `json:"type,omitempty" bson:"type,omitempty"`
// 	IsRead         bool      `json:"is_read" bson:"is_read"`
// 	CreatedAt      time.Time `json:"created_at" bson:"created_at"`
// }

// type ChatMessage struct {
// 	MessageID string    `json:"message_id" bson:"message_id"`
// 	ChatID    string    `json:"chat_id" bson:"chat_id"`
// 	SenderID  string    `json:"sender_id" bson:"sender_id"`
// 	Content   string    `json:"content" bson:"content"`
// 	Type      string    `json:"type,omitempty" bson:"type,omitempty"`
// 	CreatedAt time.Time `json:"created_at" bson:"created_at"`
// }

// type Chatx struct {
// 	ChatID       string        `json:"chat_id" bson:"chat_id"`
// 	Participants []string      `json:"participants" bson:"participants"`
// 	Messages     []ChatMessage `json:"messages,omitempty" bson:"messages,omitempty"`
// 	CreatedAt    time.Time     `json:"created_at" bson:"created_at"`
// 	UpdatedAt    time.Time     `json:"updated_at" bson:"updated_at"`
// }

package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	// ID          string    `json:"-" bson:"_id,omitempty"`
	UserID         string    `json:"userid" bson:"userid"`
	Username       string    `json:"username" bson:"username"`
	Email          string    `json:"email" bson:"email"`
	Password       string    `json:"-" bson:"password"`
	PasswordHash   string    `json:"password_hash" bson:"password_hash"`
	Role           []string  `json:"role" bson:"role"`
	Name           string    `json:"name,omitempty" bson:"name,omitempty"`
	CreatedAt      time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" bson:"updated_at"`
	Bio            string    `json:"bio,omitempty" bson:"bio,omitempty"`
	Online         bool      `json:"online"`
	LastLogin      time.Time `json:"last_login" bson:"last_login"`
	ProfilePicture string    `json:"profile_picture" bson:"profile_picture"`
	BannerPicture  string    `json:"banner_picture" bson:"banner_picture"`
	ProfileViews   int       `json:"profile_views,omitempty" bson:"profile_views,omitempty"`
	PhoneNumber    string    `json:"phone_number,omitempty" bson:"phone_number,omitempty"`
	Address        string    `json:"address,omitempty" bson:"address,omitempty"`
	// DateOfBirth    time.Time         `json:"dob" bson:"dob"`
	SocialLinks    map[string]string `json:"social_links,omitempty" bson:"social_links,omitempty"`
	IsVerified     bool              `json:"is_verified" bson:"is_verified"`
	EmailVerified  bool              `json:"email_verified" bson:"email_verified"`
	FollowersCount int               `json:"followerscount" bson:"followerscount"`
	FollowingCount int               `json:"followscount" bson:"followscount"`
}

// UserProfileResponse defines the structure for the user profile response
type UserProfileResponse struct {
	UserID         string            `json:"userid" bson:"userid"`
	Username       string            `json:"username" bson:"username"`
	Name           string            `json:"name" bson:"name"`
	Email          string            `json:"email" bson:"email"`
	Bio            string            `json:"bio,omitempty" bson:"bio,omitempty"`
	PhoneNumber    string            `json:"phone_number,omitempty" bson:"phone_number,omitempty"`
	ProfilePicture string            `json:"profile_picture" bson:"profile_picture"`
	BannerPicture  string            `json:"banner_picture" bson:"banner_picture"`
	IsFollowing    bool              `json:"is_following" bson:"is_following"` // Added here
	FollowersCount int               `json:"followerscount" bson:"followerscount"`
	FollowingCount int               `json:"followscount" bson:"followscount"`
	SocialLinks    map[string]string `json:"social_links,omitempty" bson:"social_links,omitempty"`
	Online         bool              `json:"online,omitempty"`
	LastLogin      time.Time         `json:"last_login" bson:"last_login"`
}

type UserFollow struct {
	UserID    string   `json:"userid" bson:"userid"`
	Follows   []string `json:"follows,omitempty" bson:"follows,omitempty"`
	Followers []string `json:"followers,omitempty" bson:"followers,omitempty"`
}

type UserData struct {
	UserID     string `json:"userid" bson:"userid"`
	EntityID   string `json:"entity_id" bson:"entity_id"`
	EntityType string `json:"entity_type" bson:"entity_type"`
	ItemID     string `json:"item_id" bson:"item_id"`
	ItemType   string `json:"item_type" bson:"item_type"`
	CreatedAt  string `json:"created_at" bson:"created_at"`
}

type Response struct {
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

type Setting struct {
	Type        string `json:"type"`
	Value       any    `json:"value"`
	Description string `json:"description"`
}

// type UserSettings struct {
// 	UserID   string    `bson:"userID" json:"userID"`
// 	Settings []Setting `bson:"settings" json:"settings"`
// }

type FeedPost struct {
	// ID        any `bson:"_id,omitempty" json:"id"`
	Username string   `bson:"username" json:"username"`
	PostID   string   `bson:"postid,omitempty" json:"postid"`
	UserID   string   `json:"userid" bson:"userid"`
	Text     string   `bson:"text" json:"text"`
	Type     string   `bson:"type" json:"type"`   // Post type (e.g., "text", "image", "video", "blog", etc.)
	Media    []string `bson:"media" json:"media"` // Media URLs (images, videos, etc.)
	// RepostOf    string   `bson:"repostof" json:"repostof"`
	// RePostCount string   `bson:"repostcount" json:"repostcount"`
	Timestamp string `bson:"timestamp" json:"timestamp"`
	Likes     int    `bson:"likes" json:"likes"`
	// ID         primitive.ObjectID   `json:"id" bson:"_id,omitempty"`
	Content     string               `json:"content" bson:"content"`
	MediaURL    []string             `json:"media_url,omitempty" bson:"media_url,omitempty"`
	Likers      []primitive.ObjectID `json:"likers" bson:"likers"`
	CreatedAt   time.Time            `json:"created_at" bson:"created_at"`
	Resolutions []int                `json:"resolution" bson:"resolution"`
}

// type BlogPost struct {
// 	// ID        any `bson:"_id,omitempty" json:"id"`
// 	// Username  string   `json:"username" bson:"username"`
// 	PostID    string   `bson:"postid,omitempty" json:"postid"`
// 	UserID    string   `json:"userid" bson:"userid"`
// 	Title     string   `bson:"title" json:"title"`
// 	Text      string   `bson:"text" json:"text"`
// 	Type      string   `bson:"type" json:"type"`   // Post type (e.g., "text", "image", "video", "blog", etc.)
// 	Media     []string `bson:"media" json:"media"` // Media URLs (images, videos, etc.)
// 	Timestamp string   `bson:"timestamp" json:"timestamp"`
// 	Likes     int      `bson:"likes" json:"likes"`
// 	// ID         primitive.ObjectID   `json:"id" bson:"_id,omitempty"`
// 	Content   string               `json:"content" bson:"content"`
// 	Category  string               `json:"category,omitempty" bson:"category,omitempty"`
// 	Likers    []primitive.ObjectID `json:"likers" bson:"likers"`
// 	CreatedAt time.Time            `json:"created_at" bson:"created_at"`
// }

type Activity struct {
	// Username     string              `json:"username,omitempty" bson:"username,omitempty"`
	PlaceID      string    `json:"placeId,omitempty" bson:"placeId,omitempty"`
	Action       string    `json:"action,omitempty" bson:"action,omitempty"`
	PerformedBy  string    `json:"performedBy,omitempty" bson:"performedBy,omitempty"`
	Timestamp    time.Time `json:"timestamp,omitempty" bson:"timestamp,omitempty"`
	Details      string    `json:"details,omitempty" bson:"details,omitempty"`
	IPAddress    string    `json:"ipAddress,omitempty" bson:"ipAddress,omitempty"`
	DeviceInfo   string    `json:"deviceInfo,omitempty" bson:"deviceInfo,omitempty"`
	ActivityID   string    `json:"activityid" bson:"activityid,omitempty"`
	UserID       string    `json:"user_id" bson:"user_id"`
	ActivityType string    `json:"activity_type" bson:"activity_type"` // e.g., "follow", "review", "buy"
	EntityID     string    `json:"entity_id,omitempty" bson:"entity_id,omitempty"`
	EntityType   *string   `json:"entity_type,omitempty" bson:"entity_type,omitempty"` // "event", "place", or null
}
type Merch struct {
	MerchID string `json:"merchid" bson:"merchid"`
	// EventID     string             `json:"eventid" bson:"eventid"` // Reference to Event ID
	Name        string             `json:"name" bson:"name"`
	Slug        string             `json:"slug,omitempty" bson:"slug,omitempty"`         // URL-friendly name (e.g. "concert-tshirt")
	SKU         string             `json:"sku,omitempty" bson:"sku,omitempty"`           // Stock Keeping Unit, unique per product
	Category    string             `json:"category,omitempty" bson:"category,omitempty"` // e.g. “T-Shirts”, “Accessories”
	Price       float64            `json:"price" bson:"price"`
	Discount    float64            `json:"discount,omitempty" bson:"discount,omitempty"`         // e.g. 0.10 for 10% off
	Stock       int                `json:"stock" bson:"stock"`                                   // Number of items available
	StockStatus string             `json:"stock_status,omitempty" bson:"stock_status,omitempty"` // e.g. “In Stock”, “Out of Stock”, “Preorder”
	MerchPhoto  string             `json:"merch_pic" bson:"merch_pic"`
	Gallery     []string           `json:"gallery,omitempty" bson:"gallery,omitempty"` // Additional image filenames
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	EntityID    string             `json:"entity_id" bson:"entity_id"`
	EntityType  string             `json:"entity_type" bson:"entity_type"` // “event” or “place”
	Description string             `json:"description,omitempty" bson:"description,omitempty"`
	ShortDesc   string             `json:"short_desc,omitempty" bson:"short_desc,omitempty"` // One-line summary
	Rating      float64            `json:"rating,omitempty" bson:"rating,omitempty"`         // Average rating (0.0–5.0)
	ReviewCount int                `json:"review_count,omitempty" bson:"review_count,omitempty"`
	Weight      float64            `json:"weight,omitempty" bson:"weight,omitempty"`         // In kilograms/pounds
	Dimensions  string             `json:"dimensions,omitempty" bson:"dimensions,omitempty"` // e.g. “30×20×2 cm”
	Tags        []string           `json:"tags,omitempty" bson:"tags,omitempty"`             // e.g. ["rock", "tshirt"]
	CreatedAt   time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at" bson:"updatedAt"`
	UserID      primitive.ObjectID `bson:"user_id" json:"userId"`
}

type Menu struct {
	MenuID      string             `json:"menuid" bson:"menuid"`
	PlaceID     string             `json:"placeid" bson:"placeid"` // Reference to Place ID
	Name        string             `json:"name" bson:"name"`
	Price       float64            `json:"price" bson:"price"`
	Stock       int                `json:"stock" bson:"stock"` // Number of items available
	MenuPhoto   string             `json:"menu_pic" bson:"menu_pic"`
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Description string             `json:"description,omitempty" bson:"description,omitempty"`
	CreatedAt   time.Time          `json:"created_at" bson:"created_at"`
	UserID      primitive.ObjectID `bson:"user_id" json:"userId"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updatedAt"`
}

type Ticket struct {
	TicketID    string             `json:"ticketid" bson:"ticketid"`
	EventID     string             `json:"eventid" bson:"eventid"`
	Name        string             `json:"name" bson:"name"`
	Price       float64            `json:"price" bson:"price"`
	Currency    string             `json:"currency" bson:"currency"`
	Color       string             `json:"color" bson:"color"`
	Quantity    int                `json:"quantity" bson:"quantity"`
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	EntityID    string             `json:"entity_id" bson:"entity_id"`
	EntityType  string             `json:"entity_type" bson:"entity_type"` // "event" or "place"
	Available   int                `json:"available" bson:"available"`
	Total       int                `json:"total" bson:"total"`
	CreatedAt   time.Time          `json:"created_at" bson:"created_at"`
	Description string             `bson:"description,omitempty" json:"description"`
	Sold        int                `bson:"sold" json:"sold"`
	SeatStart   int                `bson:"seatstart" json:"seatstart"`
	SeatEnd     int                `bson:"seatend" json:"seatend"`
	Seats       []string           `bson:"seats" json:"seats"` // 👈 new field
	UpdatedAt   time.Time          `bson:"updated_at" json:"updatedAt"`
}

type Seat struct {
	ID         primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	EntityID   primitive.ObjectID `json:"entity_id" bson:"entity_id"`
	EntityType string             `json:"entity_type" bson:"entity_type"` // e.g., "event" or "place"
	SeatNumber string             `json:"seat_number" bson:"seat_number"`
	UserID     primitive.ObjectID `json:"user_id" bson:"user_id,omitempty"`
	Status     string             `json:"status" bson:"status"` // e.g., "booked", "available"
}

// UserProfileResponse defines the structure for the user profile response
type UserSuggest struct {
	Username    string `json:"username" bson:"username"`
	UserID      string `json:"userid" bson:"userid"`
	IsFollowing bool
	Bio         string `json:"bio,omitempty" bson:"bio,omitempty"`
}

type Suggestion struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Type        string             `json:"type" bson:"type"` // e.g., "place" or "event"
	Title       string             `json:"title" bson:"title"`
	Description string             `json:"description,omitempty" bson:"description,omitempty"`
	Name        string             `json:"name"`
}

type Review struct {
	EntityID    string    `json:"entity_id" bson:"entity_id"`
	EntityType  string    `json:"entity_type" bson:"entity_type"` // "event" or "place"
	Comment     string    `json:"comment,omitempty" bson:"comment,omitempty"`
	UpdatedAt   time.Time `json:"updated_at" bson:"updated_at"`
	Content     string    `bson:"content" json:"content"`
	ReviewID    string    `json:"reviewid" bson:"reviewid"`
	UserID      string    `json:"userid" bson:"userid"` // Reference to User ID
	Rating      int       `json:"rating" bson:"rating"` // Rating out of 5
	Date        time.Time `json:"date" bson:"date"`     // Date of the review
	Likes       int       `json:"likes,omitempty" bson:"likes,omitempty"`
	Dislikes    int       `json:"dislikes,omitempty" bson:"dislikes,omitempty"`
	Attachments []string  `json:"attachments,omitempty" bson:"attachments,omitempty"`
	CreatedAt   string    `json:"createdAt,omitempty" bson:"createdAt,omitempty"`
}

type Media struct {
	MediaID       string             `json:"mediaid" bson:"mediaid"`
	Type          string             `json:"type" bson:"type"`
	URL           string             `json:"url" bson:"url"`
	ThumbnailURL  string             `json:"thumbnailUrl,omitempty" bson:"thumbnailUrl,omitempty"`
	Caption       string             `json:"caption" bson:"caption"`
	Description   string             `json:"description,omitempty" bson:"description,omitempty"`
	CreatorID     string             `json:"creatorid" bson:"creatorid"`
	LikesCount    int                `json:"likesCount" bson:"likesCount"`
	CommentsCount int                `json:"commentsCount" bson:"commentsCount"`
	Visibility    string             `json:"visibility" bson:"visibility"`
	Tags          []string           `json:"tags,omitempty" bson:"tags,omitempty"`
	Duration      float64            `json:"duration,omitempty" bson:"duration,omitempty"`
	FileSize      int64              `json:"fileSize,omitempty" bson:"fileSize,omitempty"`
	MimeType      string             `json:"mimeType,omitempty" bson:"mimeType,omitempty"`
	IsFeatured    bool               `json:"isFeatured,omitempty" bson:"isFeatured,omitempty"`
	EntityID      string             `json:"entityid" bson:"entityid"`
	EntityType    string             `json:"entitytype" bson:"entitytype"` // "event" or "place""video"
	CreatedAt     time.Time          `json:"created_at" bson:"created_at"`
	UserID        primitive.ObjectID `bson:"user_id" json:"userId"`
	UpdatedAt     time.Time          `bson:"updated_at" json:"updatedAt"`
}

type Place struct {
	PlaceID           string            `json:"placeid" bson:"placeid"`
	Name              string            `json:"name" bson:"name"`
	Description       string            `json:"description" bson:"description"`
	Place             string            `json:"place" bson:"place"`
	Capacity          int               `json:"capacity" bson:"capacity"`
	Date              time.Time         `json:"date" bson:"date"`
	Address           string            `json:"address" bson:"address"`
	CreatedBy         string            `json:"createdBy,omitempty" bson:"createdBy,omitempty"`
	OrganizerName     string            `json:"organizer_name" bson:"organizer_name"`
	OrganizerContact  string            `json:"organizer_contact" bson:"organizer_contact"`
	Tickets           []Ticket          `json:"tickets" bson:"tickets"`
	Merch             []Merch           `json:"merch" bson:"merch"`
	StartDateTime     time.Time         `json:"start_date_time" bson:"start_date_time"`
	EndDateTime       time.Time         `json:"end_date_time" bson:"end_date_time"`
	Category          string            `json:"category" bson:"category"`
	Banner            string            `json:"banner" bson:"banner"`
	WebsiteURL        string            `json:"website_url" bson:"website_url"`
	Status            string            `json:"status" bson:"status"`
	AccessibilityInfo string            `json:"accessibility_info" bson:"accessibility_info"`
	SocialMediaLinks  []string          `json:"social_links" bson:"social_links"`
	Tags              []string          `json:"tags" bson:"tags"`
	CustomFields      map[string]any    `json:"custom_fields" bson:"custom_fields"`
	CreatedAt         time.Time         `json:"created_at" bson:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at" bson:"updated_at"`
	City              string            `json:"city,omitempty" bson:"city,omitempty"`
	Country           string            `json:"country,omitempty" bson:"country,omitempty"`
	ZipCode           string            `json:"zipCode,omitempty" bson:"zipCode,omitempty"`
	Location          Coordinates       `json:"location,omitempty" bson:"location,omitempty"`
	Phone             string            `json:"phone,omitempty" bson:"phone,omitempty"`
	Website           string            `json:"website,omitempty" bson:"website,omitempty"`
	IsOpen            bool              `json:"isopen,omitempty" bson:"isopen,omitempty"`
	Distance          float64           `json:"distance,omitempty" bson:"distance,omitempty"`
	Views             int               `json:"views,omitempty" bson:"views,omitempty"`
	ReviewCount       int               `json:"reviewcount,omitempty" bson:"reviewcount,omitempty"`
	SocialLinks       map[string]string `json:"socialLinks,omitempty" bson:"socialLinks,omitempty"`
	UpdatedBy         string            `json:"updatedBy,omitempty" bson:"updatedBy,omitempty"`
	DeletedAt         *time.Time        `json:"deletedAt,omitempty" bson:"deletedAt,omitempty"`
	Amenities         []string          `json:"amenities,omitempty" bson:"amenities,omitempty"`
	Events            []string          `json:"events,omitempty" bson:"events,omitempty"`
	OperatingHours    []string          `json:"operatinghours,omitempty" bson:"operatinghours,omitempty"`
	Keywords          []string          `json:"keywords,omitempty" bson:"keywords,omitempty"`
}

type PlaceStatus string

const (
	Active   PlaceStatus = "active"
	Inactive PlaceStatus = "inactive"
	Closed   PlaceStatus = "closed"
)

type Coordinates struct {
	Latitude  float64 `json:"latitude,omitempty" bson:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty" bson:"longitude,omitempty"`
}

type CheckIn struct {
	UserID    string    `json:"userId,omitempty" bson:"userId,omitempty"`
	PlaceID   string    `json:"placeId,omitempty" bson:"placeId,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty" bson:"timestamp,omitempty"`
	Comment   string    `json:"comment,omitempty" bson:"comment,omitempty"`
	Rating    float64   `json:"rating,omitempty" bson:"rating,omitempty"` // Optional
	Medias    []Media   `json:"images,omitempty" bson:"images,omitempty"` // Optional
}

type PlaceVersion struct {
	PlaceID   string            `json:"placeId,omitempty" bson:"placeId,omitempty"`
	Version   int               `json:"version,omitempty" bson:"version,omitempty"`
	Data      Place             `json:"data,omitempty" bson:"data,omitempty"`
	UpdatedAt time.Time         `json:"updatedAt,omitempty" bson:"updatedAt,omitempty"`
	UpdatedBy string            `json:"updatedBy,omitempty" bson:"updatedBy,omitempty"`
	Changes   map[string]string `json:"changes,omitempty" bson:"changes,omitempty"`
}

type OperatingHours struct {
	Day          []string `json:"day,omitempty" bson:"day,omitempty"`
	OpeningHours []string `json:"opening,omitempty" bson:"opening,omitempty"`
	ClosingHours []string `json:"closing,omitempty" bson:"closing,omitempty"`
	TimeZone     string   `json:"timeZone,omitempty" bson:"timeZone,omitempty"`
}

type Tag struct {
	ID     string   `json:"id,omitempty" bson:"_id,omitempty"`
	Name   string   `json:"name,omitempty" bson:"name,omitempty"`
	Places []string `json:"places,omitempty" bson:"places,omitempty"` // List of Place IDs tagged with this keyword
}

const (
	PlaceStatusActive     = "active"
	PlaceStatusClosed     = "closed"
	PlaceStatusRenovation = "under renovation"
)

const (
	MediaTypeImage    = "image"
	MediaTypeVideo    = "video"
	MediaTypePhoto360 = "photo360"
)

type Business struct {
	ID          primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name        string             `json:"name" bson:"name"`
	Type        string             `json:"type" bson:"type"`
	Location    string             `json:"location" bson:"location"`
	Description string             `json:"description" bson:"description"`
}

type Booking struct {
	ID         primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	BusinessID primitive.ObjectID `json:"business_id" bson:"business_id"`
	UserID     string             `json:"user_id" bson:"user_id"`
	TimeSlot   string             `json:"time_slot" bson:"time_slot"`
}

type MenuItem struct {
	ID          primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name        string             `json:"name" bson:"name"`
	Price       float64            `json:"price" bson:"price"`
	Description string             `json:"description" bson:"description"`
	CreatedAt   time.Time          `json:"createdAt"`
}

type Promotion struct {
	ID          primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Title       string             `json:"title" bson:"title"`
	Description string             `json:"description" bson:"description"`
	ExpiryDate  time.Time          `json:"expiry_date" bson:"expiry_date"`
}

// Owner Management Handlers
type Owner struct {
	ID       primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name     string             `json:"name" bson:"name"`
	Email    string             `json:"email" bson:"email"`
	Password string             `json:"password" bson:"password"`
}

type Event struct {
	EventID          string    `json:"eventid" bson:"eventid"`
	Title            string    `json:"title" bson:"title"`
	Description      string    `json:"description" bson:"description"`
	Date             time.Time `json:"date" bson:"date"`
	PlaceID          string    `json:"placeid" bson:"placeid"`
	PlaceName        string    `json:"placename" bson:"placename"`
	Location         string    `json:"location" bson:"location"`
	CreatorID        string    `json:"creatorid" bson:"creatorid"`
	Tickets          []Ticket  `json:"tickets" bson:"tickets"`
	Merch            []Merch   `json:"merch" bson:"merch"`
	StartDateTime    time.Time `json:"start_date_time" bson:"start_date_time"`
	EndDateTime      time.Time `json:"end_date_time" bson:"end_date_time"`
	Category         string    `json:"category" bson:"category"`
	BannerImage      string    `json:"banner_image" bson:"banner_image"`
	SeatingPlanImage string    `json:"seatingplan" bson:"seatingplan"`
	WebsiteURL       string    `json:"website_url" bson:"website_url"`
	Status           string    `json:"status" bson:"status"`
	Tags             []string  `json:"tags" bson:"tags"`
	CreatedAt        time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" bson:"updated_at"`
	FAQs             []FAQ     `json:"faqs" bson:"faqs"`
	OrganizerName    string    `json:"organizer_name" bson:"organizer_name"`
	OrganizerContact string    `json:"organizer_contact" bson:"organizer_contact"`
	Artists          []string  `json:"artists,omitempty" bson:"artists,omitempty"`
	Published        string    `json:"published,omitempty" bson:"published,omitempty"`

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

type Gig struct {
	CreatorID string    `json:"creator_id" bson:"creator_id"` // ID of the user who created the gig
	GigID     string    `json:"gigid" bson:"gigid"`           // Unique identifier for the gig
	Name      string    `json:"name" bson:"name"`             // Name of the gig
	About     string    `json:"about" bson:"about"`           // Description or details about the gig
	Place     string    `json:"place" bson:"place"`           // Venue or location of the gig
	Area      string    `json:"area" bson:"area"`             // Area or region where the gig is held
	Type      string    `json:"type" bson:"type"`             // Type of the gig (e.g., concert, workshop, etc.)
	Category  string    `json:"category" bson:"category"`     // Category of the gig (e.g., music, art, business)
	Tags      []string  `json:"tags" bson:"tags"`             // Category of the gig (e.g., music, art, business)
	Discount  string    `json:"discount" bson:"discount"`     // Contact information for the gig
	Contact   string    `json:"contact" bson:"contact"`       // Contact information for the gig
	CreatedAt time.Time `json:"created_at" bson:"created_at"` // Timestamp of when the gig was created
	//
	UpdatedAt   time.Time `json:"updated_at,omitempty" bson:"updated_at,omitempty"`     // Optional timestamp of last update
	WebsiteURL  string    `json:"website_url,omitempty" bson:"website_url,omitempty"`   // Optional website URL for the gig
	BannerImage string    `json:"banner_image,omitempty" bson:"banner_image,omitempty"` // Path to the uploaded banner image
}
