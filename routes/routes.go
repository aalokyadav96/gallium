package routes

import (
	"naevis/activity"
	"naevis/ads"
	"naevis/analytics"
	"naevis/artists"
	"naevis/auth"
	"naevis/baito"
	"naevis/beats"
	"naevis/booking"
	"naevis/cart"
	"naevis/comments"
	"naevis/dels"
	"naevis/discord"
	"naevis/events"
	"naevis/farms"
	"naevis/feed"
	"naevis/filemgr"
	"naevis/hashtags"
	"naevis/home"
	"naevis/itinerary"
	"naevis/jobs"
	"naevis/maps"
	"naevis/media"
	"naevis/menu"
	"naevis/merch"
	"naevis/metadata"
	"naevis/middleware"
	"naevis/moderator"
	"naevis/newchat"
	"naevis/places"
	"naevis/posts"
	"naevis/products"
	"naevis/profile"
	"naevis/ratelim"
	"naevis/recipes"
	"naevis/reports"
	"naevis/reviews"
	"naevis/search"
	"naevis/settings"
	"naevis/suggestions"
	"naevis/tickets"
	"naevis/userdata"
	"naevis/utils"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func AddStaticRoutes(router *httprouter.Router) {
	router.ServeFiles("/static/uploads/*filepath", http.Dir("static/uploads"))
}

func AddActivityRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// If activity log/feed is user-specific, keep auth
	// router.POST("/api/v1/activity/log", rateLimiter.Limit(middleware.Authenticate(activity.LogActivities)))
	// router.GET("/api/v1/activity/get", middleware.Authenticate(activity.GetActivityFeed))

	// Public analytics/telemetry ingestion
	router.POST("/api/v1/scitylana/event", activity.HandleAnalyticsEvent)
	router.POST("/api/v1/telemetry/env", activity.HandleTelemetry)
	router.POST("/api/v1/telemetry/boot-error", activity.HandleTelemetry)
	router.POST("/api/v1/telemetry/sw-event", activity.HandleTelemetry)
}

// func AddAdminRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {

// 	// Moderator-only endpoints
// 	router.GET("/api/v1/moderator/reports",
// 		middleware.Authenticate(
// 			middleware.RequireRoles("moderator")(
// 				moderator.GetReports,
// 			),
// 		),
// 	)

// 	router.POST("/api/v1/moderator/apply",
// 		rateLimiter.Limit(
// 			middleware.Authenticate(moderator.ApplyModerator),
// 		),
// 	)
// }

func AddAdminRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {

	// Moderator-only endpoints
	router.GET("/api/v1/moderator/reports",
		middleware.Authenticate(
			middleware.RequireRoles("moderator")(
				moderator.GetReports,
			),
		),
	)

	router.POST("/api/v1/moderator/apply",
		rateLimiter.Limit(
			middleware.Authenticate(moderator.ApplyModerator),
		),
	)

	// Moderator-only: soft-delete entities
	router.PUT("/api/v1/moderator/delete/:type/:id",
		middleware.Authenticate(
			middleware.RequireRoles("moderator")(
				reports.SoftDeleteEntity,
			),
		),
	)

	// Moderator-only: appeals management (list + update)
	router.GET("/api/v1/moderator/appeals",
		middleware.Authenticate(
			middleware.RequireRoles("moderator")(
				reports.GetAppeals,
			),
		),
	)
	router.PUT("/api/v1/moderator/appeals/:id",
		middleware.Authenticate(
			middleware.RequireRoles("moderator")(
				reports.UpdateAppeal,
			),
		),
	)
}

func AddJobRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	router.GET("/api/v1/jobs/:entitytype/:entityid", rateLimiter.Limit(jobs.GetJobsRelatedTOEntity))
	router.POST("/api/v1/jobs/:entitytype/:entityid", rateLimiter.Limit(middleware.Authenticate(jobs.CreateBaitoForEntity)))
}

func AddBaitoRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// Create / update jobs → require auth
	router.POST("/api/v1/baitos/baito", rateLimiter.Limit(middleware.Authenticate(baito.CreateBaito)))
	router.PUT("/api/v1/baitos/baito/:baitoid", rateLimiter.Limit(middleware.Authenticate(baito.UpdateBaito)))
	router.DELETE("/api/v1/baitos/baito/:baitoid", rateLimiter.Limit(middleware.Authenticate(baito.DeleteBaito)))

	// Public job browsing
	router.GET("/api/v1/baitos/latest", rateLimiter.Limit(baito.GetLatestBaitos))
	router.GET("/api/v1/baitos/related", rateLimiter.Limit(baito.GetRelatedBaitos))

	router.GET("/api/v1/baitos/baito/:baitoid", rateLimiter.Limit(baito.GetBaitoByID))

	// Owner-specific views → require auth
	router.GET("/api/v1/baitos/mine", middleware.Authenticate(baito.GetMyBaitos))
	router.GET("/api/v1/baitos/baito/:baitoid/applicants", middleware.Authenticate(baito.GetBaitoApplicants))

	// Part-timer actions → require auth
	router.POST("/api/v1/baitos/baito/:baitoid/apply", middleware.Authenticate(baito.ApplyToBaito))
	router.GET("/api/v1/baitos/applications", middleware.Authenticate(baito.GetMyApplications))

	// Profile creation → require auth
	router.POST("/api/v1/baitos/profile", middleware.Authenticate(baito.CreateWorkerProfile))
	router.PATCH("/api/v1/baitos/profile/:workerId", middleware.Authenticate(baito.UpdateWorkerProfile))

	// Worker directory (probably private) → require auth
	router.GET("/api/v1/baitos/workers", rateLimiter.Limit(baito.GetWorkers))

	router.GET("/api/v1/baitos/workers/skills", rateLimiter.Limit(baito.GetWorkerSkills))
	router.GET("/api/v1/baitos/worker/:workerId", rateLimiter.Limit(baito.GetWorkerById))
}

func AddBeatRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// User must be logged in to like/unlike
	router.POST("/api/v1/likes/:entitytype/like/:entityid", rateLimiter.Limit(middleware.Authenticate(beats.ToggleLike)))

	// Get users who liked a post/beat
	router.GET("/api/v1/likes/:entitytype/users/:entityid", rateLimiter.Limit(middleware.Authenticate(beats.GetLikers)))

	// Batch check user likes
	router.POST("/api/v1/likes/:entitytype/batch/users", rateLimiter.Limit(middleware.Authenticate(beats.BatchUserLikes)))

	// Like count is public
	router.GET("/api/v1/likes/:entitytype/count/:entityid", rateLimiter.Limit(beats.GetLikeCount))

	// Follows
	router.PUT("/api/v1/follows/:id", rateLimiter.Limit(middleware.Authenticate(beats.ToggleFollow)))
	router.DELETE("/api/v1/follows/:id", rateLimiter.Limit(middleware.Authenticate(beats.ToggleUnFollow)))
	router.GET("/api/v1/follows/:id/status", rateLimiter.Limit(middleware.Authenticate(beats.DoesFollow)))
	router.GET("/api/v1/followers/:id", rateLimiter.Limit(beats.GetFollowers))
	router.GET("/api/v1/following/:id", rateLimiter.Limit(beats.GetFollowing))

	// Subscribes / Follows
	router.PUT("/api/v1/subscribes/:id", rateLimiter.Limit(middleware.Authenticate(beats.SubscribeEntity)))
	router.DELETE("/api/v1/subscribes/:id", rateLimiter.Limit(middleware.Authenticate(beats.UnsubscribeEntity)))
	router.GET("/api/v1/subscribes/:id", rateLimiter.Limit(middleware.Authenticate(beats.DoesSubscribeEntity)))

	// Get all subscribers of a user/artist
	router.GET("/api/v1/subscribers/:id", rateLimiter.Limit(beats.GetSubscribers))

}

func AddRecipeRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	router.GET("/api/v1/recipes/tags", rateLimiter.Limit(recipes.GetRecipeTags))         // Public
	router.GET("/api/v1/recipes", middleware.OptionalAuth(recipes.GetRecipes))           // Public/optional
	router.GET("/api/v1/recipes/recipe/:id", middleware.OptionalAuth(recipes.GetRecipe)) // Public/optional

	// Modifications require auth
	router.POST("/api/v1/recipes", middleware.Authenticate(recipes.CreateRecipe))
	router.PUT("/api/v1/recipes/recipe/:id", middleware.Authenticate(recipes.UpdateRecipe))
	router.DELETE("/api/v1/recipes/recipe/:id", middleware.Authenticate(dels.DeleteRecipe))
}

func AddDiscordRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	router.GET("/api/v1/merechats/all", middleware.Authenticate(discord.GetUserChats))
	router.POST("/api/v1/merechats/start", middleware.Authenticate(discord.StartNewChat))
	router.GET("/api/v1/merechats/chat/:chatId", middleware.Authenticate(discord.GetChatByID))
	router.GET("/api/v1/merechats/chat/:chatId/messages", middleware.Authenticate(discord.GetChatMessages))
	router.POST("/api/v1/merechats/chat/:chatId/message", middleware.Authenticate(discord.SendMessageREST))
	router.PATCH("/api/v1/meremessages/:messageId", middleware.Authenticate(discord.EditMessage))
	router.DELETE("/api/v1/meremessages/:messageId", middleware.Authenticate(dels.DeleteMessage))

	// WebSocket also needs auth to ensure only valid users connect
	router.GET("/ws/merechat", middleware.Authenticate(func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		discord.HandleWebSocket(w, r, httprouter.Params{})
	}))

	router.POST("/api/v1/merechats/chat/:chatId/upload", middleware.Authenticate(discord.UploadAttachment))
	router.GET("/api/v1/merechats/chat/:chatId/search", middleware.Authenticate(discord.SearchMessages))
	router.GET("/api/v1/meremessages/unread-count", middleware.Authenticate(discord.GetUnreadCount))
	router.POST("/api/v1/meremessages/:messageId/read", middleware.Authenticate(discord.MarkAsRead))
}

func AddHomeRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// router.GET("/api/v1/home/:apiRoute", middleware.OptionalAuth(home.GetHomeContent)) // Public/optional
	router.GET("/api/v1/homecards", middleware.OptionalAuth(home.HomeCardsHandler)) // Public/optional
}

func AddProductRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	router.GET("/api/v1/products/:entityType/:entityId", middleware.OptionalAuth(products.GetProductDetails))
}

func AddReportRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	router.PUT("/api/v1/report/:id",
		middleware.Authenticate(
			middleware.RequireRoles("moderator")(
				reports.UpdateReport,
			),
		),
	)

	router.POST("/api/v1/report",
		rateLimiter.Limit(
			middleware.Authenticate(reports.ReportContent),
		),
	)

	// Public (authenticated) endpoint to create appeals
	router.POST("/api/v1/appeals",
		rateLimiter.Limit(
			middleware.Authenticate(reports.CreateAppeal),
		),
	)
}

// func AddReportRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
// 	router.PUT("/api/v1/report/:id",
// 		middleware.Authenticate(
// 			middleware.RequireRoles("moderator")(
// 				reports.UpdateReport,
// 			),
// 		),
// 	)
// 	router.POST("/api/v1/report",
// 		rateLimiter.Limit(
// 			middleware.Authenticate(reports.ReportContent),
// 		),
// 	)

// }

func AddCommentsRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// Create comment
	router.POST("/api/v1/comments/:entitytype/:entityid", rateLimiter.Limit(middleware.Authenticate(comments.CreateComment)))

	// Get comments for an entity (supports pagination/sorting via query params)
	router.GET("/api/v1/comments/:entitytype/:entityid", comments.GetComments) // Public

	router.GET("/api/v1/comments/:entitytype/:entityid/:commentid", comments.GetComment)

	// Update & Delete
	router.PUT("/api/v1/comments/:entitytype/:entityid/:commentid", rateLimiter.Limit(middleware.Authenticate(comments.UpdateComment)))
	router.DELETE("/api/v1/comments/:entitytype/:entityid/:commentid", rateLimiter.Limit(middleware.Authenticate(dels.DeleteComment)))
}

func AddAuthRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	router.POST("/api/v1/auth/register", rateLimiter.Limit(auth.Register))
	router.POST("/api/v1/auth/login", rateLimiter.Limit(auth.Login))
	router.POST("/api/v1/auth/logout", middleware.Authenticate(auth.LogoutUser))

	router.POST("/api/v1/auth/verify-otp", rateLimiter.Limit(auth.VerifyOTPHandler))
	// router.POST("/api/v1/auth/request-otp", rateLimiter.Limit(auth.RequestOTPHandler)) // FIX: Should request OTP, not verify
}

func AddBookingRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// existing routes
	router.GET("/api/v1/bookings/slots", rateLimiter.Limit(middleware.Authenticate(booking.ListSlots)))
	router.POST("/api/v1/bookings/slots", rateLimiter.Limit(middleware.Authenticate(booking.CreateSlot)))
	router.DELETE("/api/v1/bookings/slots/:id", rateLimiter.Limit(middleware.Authenticate(booking.DeleteSlot)))

	router.GET("/api/v1/bookings/bookings", rateLimiter.Limit(middleware.Authenticate(booking.ListBookings)))
	router.POST("/api/v1/bookings/bookings", rateLimiter.Limit(middleware.Authenticate(booking.CreateBooking)))
	router.PUT("/api/v1/bookings/bookings/:id/status", rateLimiter.Limit(middleware.Authenticate(booking.UpdateBookingStatus)))
	router.DELETE("/api/v1/bookings/bookings/:id", rateLimiter.Limit(middleware.Authenticate(booking.CancelBooking)))

	router.GET("/api/v1/bookings/date-capacity", rateLimiter.Limit(middleware.Authenticate(booking.GetDateCapacity)))
	router.POST("/api/v1/bookings/date-capacity", rateLimiter.Limit(middleware.Authenticate(booking.SetDateCapacity)))

	// NEW: pricing tiers
	router.GET("/api/v1/bookings/tiers", rateLimiter.Limit(middleware.Authenticate(booking.ListTiers)))
	router.POST("/api/v1/bookings/tiers", rateLimiter.Limit(middleware.Authenticate(booking.CreateTier)))
	router.DELETE("/api/v1/bookings/tiers/:id", rateLimiter.Limit(middleware.Authenticate(booking.DeleteTier)))

	// NEW: auto slot generation from tier
	router.POST("/api/v1/bookings/tiers/:id/generate-slots", rateLimiter.Limit(middleware.Authenticate(booking.GenerateSlotsFromTier)))
}

func AddEventsRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	router.GET("/api/v1/events/events", rateLimiter.Limit(events.GetEvents))            // Public
	router.GET("/api/v1/events/events/count", rateLimiter.Limit(events.GetEventsCount)) // Public
	router.POST("/api/v1/events/event", middleware.Authenticate(events.CreateEvent))
	router.GET("/api/v1/events/event/:eventid", events.GetEvent) // Public
	router.PUT("/api/v1/events/event/:eventid", middleware.Authenticate(events.EditEvent))
	router.DELETE("/api/v1/events/event/:eventid", middleware.Authenticate(dels.DeleteEvent))

	// Should probably require auth if restricted
	router.POST("/api/v1/events/event/:eventid/faqs", middleware.Authenticate(events.AddFAQs))
}

func AddCartRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// Cart operations
	router.POST("/api/v1/cart", rateLimiter.Limit(middleware.Authenticate(cart.AddToCart)))
	router.GET("/api/v1/cart", middleware.Authenticate(cart.GetCart))
	router.POST("/api/v1/cart/update", rateLimiter.Limit(middleware.Authenticate(cart.UpdateCart)))
	router.POST("/api/v1/cart/checkout", rateLimiter.Limit(middleware.Authenticate(cart.InitiateCheckout)))

	// Checkout session creation
	router.POST("/api/v1/checkout/session", rateLimiter.Limit(middleware.Authenticate(cart.CreateCheckoutSession)))

	// Order placement
	router.POST("/api/v1/order", rateLimiter.Limit(middleware.Authenticate(cart.PlaceOrder)))

	router.POST("/api/v1/coupon/validate", rateLimiter.Limit(middleware.Authenticate(cart.ValidateCouponHandler)))

}

func RegisterFarmRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// 🌾 Farm CRUD
	router.POST("/api/v1/farms", rateLimiter.Limit(middleware.Authenticate(farms.CreateFarm)))
	router.GET("/api/v1/farms", farms.GetPaginatedFarms) // Public
	router.GET("/api/v1/farms/:id", middleware.OptionalAuth(farms.GetFarm))
	router.PUT("/api/v1/farms/:id", rateLimiter.Limit(middleware.Authenticate(farms.EditFarm)))
	router.DELETE("/api/v1/farms/:id", rateLimiter.Limit(middleware.Authenticate(dels.DeleteFarm)))

	// 🌱 Crops (within farm)
	router.POST("/api/v1/farms/:id/crops", rateLimiter.Limit(middleware.Authenticate(farms.AddCrop)))
	router.PUT("/api/v1/farms/:id/crops/:cropid", rateLimiter.Limit(middleware.Authenticate(farms.EditCrop)))
	router.DELETE("/api/v1/farms/:id/crops/:cropid", rateLimiter.Limit(middleware.Authenticate(dels.DeleteCrop)))
	router.PUT("/api/v1/farms/:id/crops/:cropid/buy", rateLimiter.Limit(middleware.Authenticate(farms.BuyCrop)))

	// 📊 Dashboard
	router.GET("/api/v1/dash/farms", middleware.Authenticate(farms.GetFarmDash))

	// 📦 Farm Orders
	router.GET("/api/v1/orders/mine", middleware.Authenticate(farms.GetMyFarmOrders))
	router.GET("/api/v1/orders/incoming", middleware.Authenticate(farms.GetIncomingFarmOrders))
	router.POST("/api/v1/farmorders/:id/accept", rateLimiter.Limit(middleware.Authenticate(farms.AcceptOrder)))
	router.POST("/api/v1/farmorders/:id/reject", rateLimiter.Limit(middleware.Authenticate(farms.RejectOrder)))
	router.POST("/api/v1/farmorders/:id/deliver", rateLimiter.Limit(middleware.Authenticate(farms.MarkOrderDelivered)))
	router.POST("/api/v1/farmorders/:id/markpaid", rateLimiter.Limit(middleware.Authenticate(farms.MarkOrderPaid)))
	router.GET("/api/v1/farmorders/:id/receipt", middleware.Authenticate(farms.DownloadReceipt))

	// 🌾 Crop catalogue & type browsing
	router.GET("/api/v1/crops", farms.GetFilteredCrops)                 // Public
	router.GET("/api/v1/crops/catalogue", farms.GetCropCatalogue)       // Public
	router.GET("/api/v1/crops/precatalogue", farms.GetPreCropCatalogue) // Public
	router.GET("/api/v1/crops/types", farms.GetCropTypes)               // Public
	router.GET("/api/v1/crops/crop/:cropname", middleware.OptionalAuth(farms.GetCropTypeFarms))

	// 🛒 Items, Products, Tools
	// -- GET
	router.GET("/api/v1/farm/items", farms.GetItems)                     // Public
	router.GET("/api/v1/farm/items/categories", farms.GetItemCategories) // Public

	// -- Products (CRUD)
	router.POST("/api/v1/farm/product", rateLimiter.Limit(middleware.Authenticate(farms.CreateProduct)))
	router.PUT("/api/v1/farm/product/:id", rateLimiter.Limit(middleware.Authenticate(farms.UpdateProduct)))
	router.DELETE("/api/v1/farm/product/:id", rateLimiter.Limit(middleware.Authenticate(dels.DeleteProduct)))

	// -- Tools (CRUD)
	router.POST("/api/v1/farm/tool", rateLimiter.Limit(middleware.Authenticate(farms.CreateTool)))
	router.PUT("/api/v1/farm/tool/:id", rateLimiter.Limit(middleware.Authenticate(farms.UpdateTool)))
	router.DELETE("/api/v1/farm/tool/:id", rateLimiter.Limit(middleware.Authenticate(dels.DeleteTool)))

	// 🖼 Upload
	router.POST("/api/v1/upload/images", rateLimiter.Limit(middleware.Authenticate(utils.UploadImages)))
}

func AddMerchRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// Create merch
	router.POST("/api/v1/merch/:entityType/:eventid", rateLimiter.Limit(middleware.Authenticate(merch.CreateMerch)))

	// Buy merch
	router.POST("/api/v1/merch/:entityType/:eventid/:merchid/buy", rateLimiter.Limit(middleware.Authenticate(merch.BuyMerch)))

	// Public view
	router.GET("/api/v1/merch/:entityType/:eventid", merch.GetMerchs)
	router.GET("/api/v1/merch/:entityType/:eventid/:merchid", merch.GetMerch)
	router.GET("/api/v1/merch/:entityType", merch.GetMerchPage)

	// Edit/Delete
	router.PUT("/api/v1/merch/:entityType/:eventid/:merchid", rateLimiter.Limit(middleware.Authenticate(merch.EditMerch)))
	router.DELETE("/api/v1/merch/:entityType/:eventid/:merchid", rateLimiter.Limit(middleware.Authenticate(dels.DeleteMerch)))

	// Payment flows
	router.POST("/api/v1/merch/:entityType/:eventid/:merchid/payment-session", rateLimiter.Limit(middleware.Authenticate(merch.CreateMerchPaymentSession)))
	router.POST("/api/v1/merch/:entityType/:eventid/:merchid/confirm-purchase", rateLimiter.Limit(middleware.Authenticate(merch.ConfirmMerchPurchase)))
}

func AddTicketRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// Ticket CRUD
	router.POST("/api/v1/ticket/event/:eventid", rateLimiter.Limit(middleware.Authenticate(tickets.CreateTicket)))
	router.GET("/api/v1/ticket/event/:eventid", rateLimiter.Limit(tickets.GetTickets))
	router.GET("/api/v1/ticket/event/:eventid/:ticketid", rateLimiter.Limit(tickets.GetTicket))
	router.PUT("/api/v1/ticket/event/:eventid/:ticketid", rateLimiter.Limit(middleware.Authenticate(tickets.EditTicket)))
	router.DELETE("/api/v1/ticket/event/:eventid/:ticketid", rateLimiter.Limit(middleware.Authenticate(dels.DeleteTicket)))

	// Buying
	router.POST("/api/v1/ticket/event/:eventid/:ticketid/buy", rateLimiter.Limit(middleware.Authenticate(tickets.BuyTicket)))
	router.POST("/api/v1/tickets/book", rateLimiter.Limit(middleware.Authenticate(tickets.BuysTicket)))

	// Payment flows
	router.POST("/api/v1/ticket/event/:eventid/:ticketid/payment-session", rateLimiter.Limit(middleware.Authenticate(tickets.CreateTicketPaymentSession)))
	router.POST("/api/v1/ticket/event/:eventid/:ticketid/confirm-purchase", rateLimiter.Limit(middleware.Authenticate(tickets.ConfirmTicketPurchase)))

	// Verification/printing
	router.GET("/api/v1/ticket/verify/:eventid", rateLimiter.Limit(tickets.VerifyTicket))
	router.GET("/api/v1/ticket/print/:eventid", rateLimiter.Limit(tickets.PrintTicket))

	// Event updates
	router.GET("/api/v1/events/event/:eventid/updates", rateLimiter.Limit(tickets.EventUpdates))

	// Seats
	router.GET("/api/v1/seats/:eventid/available-seats", rateLimiter.Limit(tickets.GetAvailableSeats))
	router.POST("/api/v1/seats/:eventid/lock-seats", rateLimiter.Limit(middleware.Authenticate(tickets.LockSeats)))
	router.POST("/api/v1/seats/:eventid/unlock-seats", rateLimiter.Limit(middleware.Authenticate(tickets.UnlockSeats)))
	router.POST("/api/v1/seats/:eventid/ticket/:ticketid/confirm-purchase", rateLimiter.Limit(middleware.Authenticate(tickets.ConfirmSeatPurchase)))
	router.GET("/api/v1/ticket/event/:eventid/:ticketid/seats", rateLimiter.Limit(tickets.GetTicketSeats))
}

func AddSuggestionsRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	router.GET("/api/v1/suggestions/places/nearby", rateLimiter.Limit(suggestions.GetNearbyPlaces))
	router.GET("/api/v1/suggestions/places", rateLimiter.Limit(suggestions.SuggestionsHandler))
	router.GET("/api/v1/suggestions/follow", rateLimiter.Limit(middleware.Authenticate(suggestions.SuggestFollowers)))
}

func AddReviewsRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// Public view, but rate-limited
	router.GET("/api/v1/reviews/:entityType/:entityId", rateLimiter.Limit(reviews.GetReviews))
	router.GET("/api/v1/reviews/:entityType/:entityId/:reviewId", rateLimiter.Limit(reviews.GetReview))

	// Authenticated actions
	router.POST("/api/v1/reviews/:entityType/:entityId", rateLimiter.Limit(middleware.Authenticate(reviews.AddReview)))
	router.PUT("/api/v1/reviews/:entityType/:entityId/:reviewId", rateLimiter.Limit(middleware.Authenticate(reviews.EditReview)))
	router.DELETE("/api/v1/reviews/:entityType/:entityId/:reviewId", rateLimiter.Limit(middleware.Authenticate(dels.DeleteReview)))
}

func AddMediaRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// Public view, but rate-limited
	router.GET("/api/v1/media/:entitytype/:entityid/:id", rateLimiter.Limit(media.GetMedia))
	router.GET("/api/v1/media/:entitytype/:entityid", rateLimiter.Limit(media.GetMedias))

	// Authenticated actions
	router.POST("/api/v1/media/:entitytype/:entityid", rateLimiter.Limit(middleware.Authenticate(media.AddMedia)))
	router.PUT("/api/v1/media/:entitytype/:entityid/:id", rateLimiter.Limit(middleware.Authenticate(media.EditMedia)))
	router.DELETE("/api/v1/media/:entitytype/:entityid/:id", rateLimiter.Limit(middleware.Authenticate(dels.DeleteMedia)))
}

func AddPostRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// Public read
	router.GET("/api/v1/posts/post/:id", rateLimiter.Limit(posts.GetPost))
	router.GET("/api/v1/posts", rateLimiter.Limit(posts.GetAllPosts))
	router.POST("/api/v1/posts/upload", rateLimiter.Limit(posts.UploadImage))

	// Authenticated write
	router.POST("/api/v1/posts/post", rateLimiter.Limit(middleware.Authenticate(posts.CreatePost)))
	router.PATCH("/api/v1/posts/post/:id", rateLimiter.Limit(middleware.Authenticate(posts.UpdatePost)))
	router.DELETE("/api/v1/posts/post/:id", rateLimiter.Limit(middleware.Authenticate(posts.DeletePost)))
}

func AddPlaceRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// Public
	router.GET("/api/v1/places/places", rateLimiter.Limit(places.GetPlaces))
	router.GET("/api/v1/places/place/:placeid", rateLimiter.Limit(places.GetPlace))
	router.GET("/api/v1/places/place-details", rateLimiter.Limit(places.GetPlaceQ))

	// Authenticated place management
	router.POST("/api/v1/places/place", rateLimiter.Limit(middleware.Authenticate(places.CreatePlace)))
	router.PUT("/api/v1/places/place/:placeid", rateLimiter.Limit(middleware.Authenticate(places.EditPlace)))
	router.DELETE("/api/v1/places/place/:placeid", rateLimiter.Limit(middleware.Authenticate(dels.DeletePlace)))

	// Menus (public view + auth for changes)
	router.GET("/api/v1/places/menu/:placeid", rateLimiter.Limit(menu.GetMenus))
	router.GET("/api/v1/places/menu/:placeid/:menuid/stock", rateLimiter.Limit(menu.GetStock))
	router.GET("/api/v1/places/menu/:placeid/:menuid", rateLimiter.Limit(menu.GetMenu))

	router.POST("/api/v1/places/menu/:placeid", rateLimiter.Limit(middleware.Authenticate(menu.CreateMenu)))
	router.PUT("/api/v1/places/menu/:placeid/:menuid", rateLimiter.Limit(middleware.Authenticate(menu.EditMenu)))
	router.DELETE("/api/v1/places/menu/:placeid/:menuid", rateLimiter.Limit(middleware.Authenticate(dels.DeleteMenu)))

	// Buying & payment flows
	router.POST("/api/v1/places/menu/:placeid/:menuid/buy", rateLimiter.Limit(middleware.Authenticate(menu.BuyMenu)))
	router.POST("/api/v1/places/menu/:placeid/:menuid/payment-session", rateLimiter.Limit(middleware.Authenticate(menu.CreateMenuPaymentSession)))
	router.POST("/api/v1/places/menu/:placeid/:menuid/confirm-purchase", rateLimiter.Limit(middleware.Authenticate(menu.ConfirmMenuPurchase)))
}

func AddProfileRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// Own profile
	router.GET("/api/v1/profile/profile", rateLimiter.Limit(middleware.Authenticate(profile.GetProfile)))
	router.PUT("/api/v1/profile/edit", rateLimiter.Limit(middleware.Authenticate(profile.EditProfile)))
	router.PUT("/api/v1/profile/avatar", rateLimiter.Limit(middleware.Authenticate(profile.EditProfilePic)))
	router.PUT("/api/v1/profile/banner", rateLimiter.Limit(middleware.Authenticate(profile.EditProfileBanner)))
	router.DELETE("/api/v1/profile/delete", rateLimiter.Limit(middleware.Authenticate(dels.DeleteProfile)))

	// Public profile viewing
	router.GET("/api/v1/user/:username", rateLimiter.Limit(profile.GetUserProfile))

	// Other user data (requires auth to see private info)
	router.GET("/api/v1/user/:username/data", rateLimiter.Limit(middleware.Authenticate(userdata.GetUserProfileData)))
	router.GET("/api/v1/user/:username/udata", rateLimiter.Limit(middleware.Authenticate(userdata.GetOtherUserProfileData)))

}

func AddArtistRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// Public read
	router.GET("/api/v1/artists", rateLimiter.Limit(artists.GetAllArtists))
	router.GET("/api/v1/artists/:id", rateLimiter.Limit(artists.GetArtistByID))
	router.GET("/api/v1/events/event/:eventid/artists", rateLimiter.Limit(artists.GetArtistsByEvent))
	router.GET("/api/v1/artists/:id/songs", rateLimiter.Limit(artists.GetArtistsSongs))
	router.GET("/api/v1/artists/:id/albums", rateLimiter.Limit(artists.GetArtistsAlbums))
	router.GET("/api/v1/artists/:id/posts", rateLimiter.Limit(artists.GetArtistsPosts))
	router.GET("/api/v1/artists/:id/merch", rateLimiter.Limit(artists.GetArtistsMerch))
	router.GET("/api/v1/artists/:id/behindthescenes", rateLimiter.Limit(artists.GetBTS))
	router.GET("/api/v1/artists/:id/events", rateLimiter.Limit(artists.GetArtistEvents))

	// Authenticated write
	router.POST("/api/v1/artists", rateLimiter.Limit(middleware.Authenticate(artists.CreateArtist)))
	router.PUT("/api/v1/artists/:id", rateLimiter.Limit(middleware.Authenticate(artists.UpdateArtist)))
	router.DELETE("/api/v1/artists/:id", rateLimiter.Limit(middleware.Authenticate(dels.DeleteArtistByID)))

	router.POST("/api/v1/artists/:id/songs", rateLimiter.Limit(middleware.Authenticate(artists.PostNewSong)))
	router.PUT("/api/v1/artists/:id/songs/:songId/edit", rateLimiter.Limit(middleware.Authenticate(artists.EditSong)))
	router.DELETE("/api/v1/artists/:id/songs/:songId", rateLimiter.Limit(middleware.Authenticate(dels.DeleteSong)))

	router.PUT("/api/v1/artists/:id/events/addtoevent", rateLimiter.Limit(middleware.Authenticate(artists.AddArtistToEvent)))
	router.POST("/api/v1/artists/:id/events", rateLimiter.Limit(middleware.Authenticate(artists.CreateArtistEvent)))
	router.PUT("/api/v1/artists/:id/events", rateLimiter.Limit(middleware.Authenticate(artists.UpdateArtistEvent)))
	router.DELETE("/api/v1/artists/:id/events", rateLimiter.Limit(middleware.Authenticate(dels.DeleteArtistEvent)))
}

// AddMapRoutes binds routes to the provided router and rate limiter
func AddMapRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// entity-specific endpoints
	router.GET("/api/v1/maps/config/:entity", rateLimiter.Limit(maps.GetMapConfig))
	router.GET("/api/v1/maps/markers/:entity", rateLimiter.Limit(maps.GetMapMarkers))

	// player progression endpoints
	router.POST("/api/v1/player/progress", rateLimiter.Limit(maps.UpdatePlayerProgress))
	router.GET("/api/v1/player/progress", rateLimiter.Limit(maps.GetPlayerProgress))
}

// func AddMapRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
// 	router.GET("/api/v1/maps/config/:entity", rateLimiter.Limit(maps.GetMapConfig))
// 	router.GET("/api/v1/maps/markers/:entity", rateLimiter.Limit(maps.GetMapMarkers))
// 	// player progression endpoint (POST)
// 	// query param: ?entity=ls (defaults to ls)
// 	router.POST("/api/v1/player/progress", rateLimiter.Limit(maps.UpdatePlayerProgress))

// }

func AddItineraryRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// Public
	router.GET("/api/v1/itineraries", rateLimiter.Limit(itinerary.GetItineraries))
	router.GET("/api/v1/itineraries/all/:id", rateLimiter.Limit(itinerary.GetItinerary))
	router.GET("/api/v1/itineraries/search", rateLimiter.Limit(itinerary.SearchItineraries))

	// Authenticated write
	router.POST("/api/v1/itineraries", rateLimiter.Limit(middleware.Authenticate(itinerary.CreateItinerary)))
	router.PUT("/api/v1/itineraries/:id", rateLimiter.Limit(middleware.Authenticate(itinerary.UpdateItinerary)))
	router.DELETE("/api/v1/itineraries/:id", rateLimiter.Limit(middleware.Authenticate(dels.DeleteItinerary)))
	router.POST("/api/v1/itineraries/:id/fork", rateLimiter.Limit(middleware.Authenticate(itinerary.ForkItinerary)))
	router.PUT("/api/v1/itineraries/:id/publish", rateLimiter.Limit(middleware.Authenticate(itinerary.PublishItinerary)))
}

func AddUtilityRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	router.GET("/api/v1/csrf", rateLimiter.Limit(middleware.Authenticate(utils.CSRF)))
}

func AddFeedRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// Public viewing
	router.GET("/api/v1/feed/post/:postid", rateLimiter.Limit(feed.GetPost))

	// Authenticated feed actions
	router.GET("/api/v1/feed/feed", rateLimiter.Limit(middleware.Authenticate(feed.GetPosts)))
	router.GET("/api/v1/feed/media/:entityType/:entityId", rateLimiter.Limit(middleware.Authenticate(feed.GetPosts)))

	router.POST("/api/v1/feed/post", rateLimiter.Limit(middleware.Authenticate(feed.CreateTweetPost)))
	router.DELETE("/api/v1/feed/post/:postid", rateLimiter.Limit(middleware.Authenticate(dels.DeletePost)))

	// NEW
	router.PATCH("/api/v1/feed/post/:postid", rateLimiter.Limit(middleware.Authenticate(feed.EditPost)))
	router.POST("/api/v1/feed/post/:postid/subtitles/:lang", rateLimiter.Limit(middleware.Authenticate(feed.UploadSubtitle)))
}

// func AddFeedRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
// 	// Public viewing
// 	router.GET("/api/v1/feed/post/:postid", rateLimiter.Limit(feed.GetPost))

// 	// Authenticated feed actions
// 	router.GET("/api/v1/feed/feed", rateLimiter.Limit(middleware.Authenticate(feed.GetPosts)))

// 	router.POST("/api/v1/feed/post", rateLimiter.Limit(middleware.Authenticate(feed.CreateTweetPost)))
// 	router.DELETE("/api/v1/feed/post/:postid", rateLimiter.Limit(middleware.Authenticate(feed.DeletePost)))
// }

func AddSettingsRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	router.GET("/api/v1/settings/init/:userid", rateLimiter.Limit(middleware.Authenticate(settings.InitUserSettings)))
	router.GET("/api/v1/settings/all", rateLimiter.Limit(middleware.Authenticate(settings.GetUserSettings)))
	router.PUT("/api/v1/settings/setting/:type", rateLimiter.Limit(middleware.Authenticate(settings.UpdateUserSetting)))
}

func AddAdsRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	router.GET("/api/v1/sda/sda", rateLimiter.Limit(middleware.OptionalAuth(ads.GetAds)))
}

func AddSearchRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	router.GET("/api/v1/ac", rateLimiter.Limit(search.Autocompleter))
	router.GET("/api/v1/search/:entityType", rateLimiter.Limit(search.SearchHandler))
	router.POST("/api/v1/emitted", rateLimiter.Limit(search.EventHandler))
}

func AddBannerRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	router.PUT("/api/v1/picture/:entitytype/:entityid", rateLimiter.Limit(middleware.Authenticate(filemgr.EditBanner)))
}

func AddHashtagRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	router.GET("/api/v1/hashtags/hashtag/:tag", hashtags.GetHashtagPosts)
	router.GET("/api/v1/hashtags/hashtag/:tag/top", hashtags.GetTopHashtagPosts)
	router.GET("/api/v1/hashtags/hashtag/:tag/latest", hashtags.GetLatestHashtagPosts)
	router.GET("/api/v1/hashtags/hashtag/:tag/people", hashtags.GetHashtagPeople)
	router.GET("/api/v1/hashtags/hashtags/trending", hashtags.GetTrendingHashtags)

	// router.GET("/api/v1/hashtags/hashtag/:tag", hashtags.GetHashtagPosts)
	// router.GET("/api/v1/hashtags/hashtags/trending", hashtags.GetTrendingHashtags)
}

func AddAnalyticsRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// Example: /api/v1/antics/events/123 or /api/v1/analytics/places/456
	router.GET("/api/v1/antics/:entityType/:entityId", rateLimiter.Limit(analytics.GetEntityAnalytics))
}

func AddMiscRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	router.GET("/api/v1/users/meta", rateLimiter.Limit(metadata.GetUsersMeta))

	// router.POST("/api/v1/check-file", rateLimiter.Limit(filecheck.CheckFileExists))
	// router.POST("/api/v1/upload", rateLimiter.Limit(filecheck.UploadFile))
	// router.POST("/api/v1/feed/remhash", rateLimiter.Limit(filecheck.RemoveUserFile))
	// router.GET("/resize/:folder/*filename", cdn.ServeStatic)

}

func AddNewChatRoutes(router *httprouter.Router, hub *newchat.Hub, rateLimiter *ratelim.RateLimiter) {
	router.GET("/api/v1/newchats/all", middleware.Authenticate(newchat.GetUserChats))
	router.POST("/api/v1/newchats/init", middleware.Authenticate(newchat.InitChat))

	// This should likely be protected; token could be in query or header
	router.GET("/ws/newchat/:room", middleware.Authenticate(newchat.WebSocketHandler(hub)))

	router.POST("/newchat/upload", middleware.Authenticate(newchat.UploadHandler(hub)))
	router.POST("/newchat/edit", middleware.Authenticate(newchat.EditMessageHandler(hub)))
	router.POST("/newchat/delete", middleware.Authenticate(newchat.DeleteMessageHandler(hub)))

	// router.GET("/newchat/:room/poll", middleware.Authenticate(newchat.PollMessagesHandler))

	router.GET("/api/v1/newchat/:room", middleware.Authenticate(newchat.GetChat))
	router.POST("/api/v1/newchat/:room/message", middleware.Authenticate(newchat.CreateMessage))
	router.DELETE("/api/v1/newchat/:room/message/:msgid", middleware.Authenticate(dels.DeletesMessage))

	/**/

	router.PUT("/api/v1/newchat/:room/message/:msgid", middleware.Authenticate(newchat.UpdateMessage))

}
