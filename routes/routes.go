package routes

import (
	"naevis/activity"
	"naevis/admin"
	"naevis/ads"
	"naevis/artists"
	"naevis/auth"
	"naevis/baito"
	"naevis/beats"
	"naevis/booking"
	"naevis/cart"
	"naevis/cartoons"
	"naevis/chats"
	"naevis/comments"
	"naevis/discord"
	"naevis/events"
	"naevis/farms"
	"naevis/feed"
	"naevis/home"
	"naevis/itinerary"
	"naevis/maps"
	"naevis/media"
	"naevis/menu"
	"naevis/merch"
	"naevis/middleware"
	"naevis/newchat"
	"naevis/places"
	"naevis/posts"
	"naevis/profile"
	"naevis/qna"
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
	_ "net/http/pprof"

	"github.com/julienschmidt/httprouter"
)

func AddStaticRoutes(router *httprouter.Router) {
	router.ServeFiles("/static/postpic/*filepath", http.Dir("static/postpic"))
	router.ServeFiles("/static/merchpic/*filepath", http.Dir("static/merchpic"))
	router.ServeFiles("/static/menupic/*filepath", http.Dir("static/menupic"))
	router.ServeFiles("/static/uploads/*filepath", http.Dir("static/uploads"))
	router.ServeFiles("/static/placepic/*filepath", http.Dir("static/placepic"))
	router.ServeFiles("/static/businesspic/*filepath", http.Dir("static/eventpic"))
	router.ServeFiles("/static/userpic/*filepath", http.Dir("static/userpic"))
	router.ServeFiles("/static/eventpic/*filepath", http.Dir("static/eventpic"))
	router.ServeFiles("/static/artistpic/*filepath", http.Dir("static/artistpic"))
	router.ServeFiles("/static/cartoonpic/*filepath", http.Dir("static/cartoonpic"))
	router.ServeFiles("/static/chatpic/*filepath", http.Dir("static/chatpic"))
	router.ServeFiles("/static/newchatpic/*filepath", http.Dir("static/newchatpic"))
	router.ServeFiles("/static/threadpic/*filepath", http.Dir("static/threadpic"))
}

func AddActivityRoutes(router *httprouter.Router) {
	// router.POST("/api/v1/activity/log", ratelim.RateLimit(middleware.Authenticate(activity.LogActivities)))
	// router.GET("/api/v1/activity/get", middleware.Authenticate(activity.GetActivityFeed))

	router.POST("/api/v1/scitylana/event", activity.HandleAnalyticsEvent)

}

func AddAdminRoutes(router *httprouter.Router) {
	router.GET("/api/v1/admin/reports", middleware.Authenticate(admin.GetReports))
}

func AddBaitoRoutes(router *httprouter.Router) {
	router.POST("/api/v1/baitos/baito", ratelim.RateLimit(middleware.Authenticate(baito.CreateBaito)))
	router.PUT("/api/v1/baitos/baito/:id", ratelim.RateLimit(middleware.Authenticate(baito.UpdateBaito)))
	router.GET("/api/v1/baitos/latest", ratelim.RateLimit(middleware.Authenticate(baito.GetLatestBaitos)))
	router.GET("/api/v1/baitos/related", ratelim.RateLimit(middleware.Authenticate(baito.GetRelatedBaitos)))
	router.GET("/api/v1/baitos/baito/:id", ratelim.RateLimit(middleware.Authenticate(baito.GetBaitoByID)))
	// owner
	router.GET("/api/v1/baitos/mine", middleware.Authenticate(baito.GetMyBaitos))
	router.GET("/api/v1/baitos/baito/:id/applicants", middleware.Authenticate(baito.GetBaitoApplicants))
	// part timer
	router.POST("/api/v1/baitos/baito/:id/apply", middleware.Authenticate(baito.ApplyToBaito))
	router.GET("/api/v1/baitos/applications", middleware.Authenticate(baito.GetMyApplications))

	router.POST("/api/v1/baitos/profile", middleware.Authenticate(baito.CreateBaitoUserProfile))

	// workers
	router.GET("/api/v1/baitos/workers", ratelim.RateLimit(middleware.Authenticate(baito.GetWorkers)))
	router.GET("/api/v1/baitos/workers/skills", ratelim.RateLimit(middleware.Authenticate(baito.GetWorkerSkills)))
	router.GET("/api/v1/baitos/worker/:workerId", ratelim.RateLimit(middleware.Authenticate(baito.GetWorkerById)))
}

func AddBeatRoutes(router *httprouter.Router) {
	router.POST("/api/v1/likes/:entitytype/:entityid", ratelim.RateLimit(middleware.Authenticate(beats.ToggleLike)))
	router.GET("/api/v1/likes/:entitytype/:entityid", ratelim.RateLimit(middleware.Authenticate(beats.GetLikeCount)))
}

func AddRecipeRoutes(router *httprouter.Router) {
	router.GET("/api/v1/recipes/tags", ratelim.RateLimit(recipes.GetRecipeTags))
	router.GET("/api/v1/recipes", middleware.OptionalAuth(recipes.GetRecipes))
	router.GET("/api/v1/recipes/recipe/:id", middleware.OptionalAuth(recipes.GetRecipe))
	router.POST("/api/v1/recipes", middleware.Authenticate(recipes.CreateRecipe))
	router.PUT("/api/v1/recipes/recipe/:id", middleware.Authenticate(recipes.UpdateRecipe))
	router.DELETE("/api/v1/recipes/recipe/:id", middleware.Authenticate(recipes.DeleteRecipe))
}

func AddDiscordRoutes(router *httprouter.Router) {
	router.GET("/api/v1/merechats/all", middleware.Authenticate(discord.GetUserChats))
	router.POST("/api/v1/merechats/start", middleware.Authenticate(discord.StartNewChat))
	router.GET("/api/v1/merechats/chat/:chatId", middleware.Authenticate(discord.GetChatByID))
	router.GET("/api/v1/merechats/chat/:chatId/messages", middleware.Authenticate(discord.GetChatMessages))
	router.POST("/api/v1/merechats/chat/:chatId/message", middleware.Authenticate(discord.SendMessageREST))
	router.PATCH("/api/v1/meremessages/:messageId", middleware.Authenticate(discord.EditMessage))
	router.DELETE("/api/v1/meremessages/:messageId", middleware.Authenticate(discord.DeleteMessage))
	// router.GET("/ws/merechat", middleware.Authenticate(func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// 	discord.HandleWebSocket(w, r, httprouter.Params{{Key: "userId", Value: utils.GetUserIDFromRequest(r)}})
	// }))
	router.GET("/ws/merechat", middleware.Authenticate(func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		discord.HandleWebSocket(w, r, httprouter.Params{}) // or just nil
	}))
	router.POST("/api/v1/merechats/chat/:chatId/upload", middleware.Authenticate(discord.UploadAttachment))
	router.GET("/api/v1/merechats/chat/:chatId/search", middleware.Authenticate(discord.SearchMessages))
	router.GET("/api/v1/meremessages/unread-count", middleware.Authenticate(discord.GetUnreadCount))
	router.POST("/api/v1/meremessages/:messageId/read", middleware.Authenticate(discord.MarkAsRead))

}

func AddNewChatRoutes(router *httprouter.Router, hub *newchat.Hub) {
	router.GET("/api/v1/newchats/all", middleware.Authenticate(chats.GetUserChats))
	// router.POST("/api/v1/newchats/init", middleware.Authenticate(newchat.InitNewChat))
	router.GET("/ws/newchat/:room", newchat.WebSocketHandler(hub))
	router.POST("/newchat/upload", middleware.Authenticate(newchat.UploadHandler(hub)))
	router.POST("/newchat/edit", newchat.EditMessageHandler(hub))
	router.POST("/newchat/delete", newchat.DeleteMessageHandler(hub))

}

func AddChatRoutes(router *httprouter.Router) {
	router.GET("/api/v1/chats/all", middleware.Authenticate(chats.GetUserChats))
	router.POST("/api/v1/chats/init", middleware.Authenticate(chats.InitChat))
	router.GET("/api/v1/chat/:chatid", middleware.Authenticate(chats.GetChat))
	router.POST("/api/v1/chat/:chatid/message", middleware.Authenticate(chats.CreateMessage))
	router.PUT("/api/v1/chat/:chatid/message/:msgid", middleware.Authenticate(chats.UpdateMessage))
	// router.GET("/api/v1/chat/:chatid", middleware.Authenticate(chats.GetMessage))
	router.DELETE("/api/v1/chat/:chatid/message/:msgid", middleware.Authenticate(chats.DeleteMessage))
	router.GET("/ws/chat", chats.ChatWebSocket)
	router.GET("/api/v1/chat/:chatid/search", middleware.Authenticate(chats.SearchChat))
}

func AddHomeRoutes(router *httprouter.Router) {
	router.GET("/api/v1/home/:apiRoute", middleware.OptionalAuth(home.GetHomeContent))
}

func AddReportRoutes(router *httprouter.Router) {
	router.POST("/api/v1/report", ratelim.RateLimit(middleware.Authenticate(reports.ReportContent)))
	router.GET("/api/v1/reports", ratelim.RateLimit(middleware.Authenticate(reports.GetReports)))
	router.PUT("/api/v1/report/:id", ratelim.RateLimit(middleware.Authenticate(reports.UpdateReport)))
}

func AddCommentsRoutes(router *httprouter.Router) {
	router.POST("/api/v1/comments/:entitytype/:entityid", comments.CreateComment)
	router.GET("/api/v1/comments/:entitytype/:entityid", comments.GetComments)
	router.GET("/api/v1/comments/:entitytype", comments.GetComment)
	router.PUT("/api/v1/comments/:entitytype/:entityid/:commentid", comments.UpdateComment)
	router.DELETE("/api/v1/comments/:entitytype/:entityid/:commentid", comments.DeleteComment)
}

func AddAuthRoutes(router *httprouter.Router) {
	router.POST("/api/v1/auth/register", ratelim.RateLimit(auth.Register))
	router.POST("/api/v1/auth/login", ratelim.RateLimit(auth.Login))
	router.POST("/api/v1/auth/logout", middleware.Authenticate(auth.LogoutUser))
	router.POST("/api/v1/auth/token/refresh", ratelim.RateLimit(middleware.Authenticate(auth.RefreshToken)))

	router.POST("/api/v1/auth/verify-otp", ratelim.RateLimit(auth.VerifyOTPHandler))
	router.POST("/api/v1/auth/request-otp", ratelim.RateLimit(auth.VerifyOTPHandler))
}

func AddBookingRoutes(router *httprouter.Router) {
	router.POST("/api/v1/slots", ratelim.RateLimit(booking.AddSlot))
	router.DELETE("/api/v1/slots/:date/:time", ratelim.RateLimit(booking.DeleteSlot))
	router.GET("/api/v1/slots/:date", middleware.Authenticate(booking.GetSlotsByDate))
	router.GET("/api/v1/bookings/:date", ratelim.RateLimit(middleware.Authenticate(booking.GetBookingsByDate)))
	router.POST("/api/v1/bookings", ratelim.RateLimit(middleware.Authenticate(booking.CreateBooking)))
}

func AddEventsRoutes(router *httprouter.Router) {
	router.GET("/api/v1/events/events", ratelim.RateLimit(events.GetEvents))
	router.GET("/api/v1/events/events/count", ratelim.RateLimit(events.GetEventsCount))
	router.POST("/api/v1/events/event", middleware.Authenticate(events.CreateEvent))
	router.GET("/api/v1/events/event/:eventid", events.GetEvent)
	router.PUT("/api/v1/events/event/:eventid", middleware.Authenticate(events.EditEvent))
	router.DELETE("/api/v1/events/event/:eventid", middleware.Authenticate(events.DeleteEvent))
	router.POST("/api/v1/events/event/:eventid/faqs", events.AddFAQs)
}

func AddCartRoutes(router *httprouter.Router) {
	// Cart operations
	router.POST("/api/v1/cart", middleware.Authenticate(cart.AddToCart))
	router.GET("/api/v1/cart", middleware.Authenticate(cart.GetCart))
	router.POST("/api/v1/cart/update", middleware.Authenticate(cart.UpdateCart))
	router.POST("/api/v1/cart/checkout", middleware.Authenticate(cart.InitiateCheckout))

	// Checkout session creation
	router.POST("/api/v1/checkout/session", middleware.Authenticate(cart.CreateCheckoutSession))

	// Order placement
	router.POST("/api/v1/order", middleware.Authenticate(cart.PlaceOrder))
}

// // RegisterFarmRoutes wires up endpoints to the given router
// func RegisterFarmRoutes(router *httprouter.Router) {
// 	router.POST("/api/v1/farms", middleware.Authenticate(farms.CreateFarm))
// 	router.GET("/api/v1/farms", farms.GetPaginatedFarms)
// 	router.GET("/api/v1/farms/:id", middleware.Authenticate(farms.GetFarm))
// 	router.PUT("/api/v1/farms/:id", middleware.Authenticate(farms.EditFarm))
// 	router.DELETE("/api/v1/farms/:id", middleware.Authenticate(farms.DeleteFarm))
// 	// Crop routes
// 	router.POST("/api/v1/farms/:id/crops", middleware.Authenticate(farms.AddCrop))
// 	router.PUT("/api/v1/farms/:id/crops/:cropid", middleware.Authenticate(farms.EditCrop))
// 	router.DELETE("/api/v1/farms/:id/crops/:cropid", middleware.Authenticate(farms.DeleteCrop))
// 	router.PUT("/api/v1/farms/:id/crops/:cropid/buy", middleware.Authenticate(farms.BuyCrop))

// 	router.GET("/api/v1/dash/farms", middleware.Authenticate(farms.GetFarmDash))
// 	router.GET("/api/v1/farmorders/mine", middleware.Authenticate(farms.GetMyFarmOrders))
// 	router.GET("/api/v1/farmorders/incoming", middleware.Authenticate(farms.GetIncomingFarmOrders))

// 	router.GET("/api/v1/crops", farms.GetFilteredCrops)
// 	router.GET("/api/v1/crops/catalogue", farms.GetCropCatalogue)
// 	router.GET("/api/v1/crops/precatalogue", farms.GetPreCropCatalogue)
// 	router.GET("/api/v1/crops/types", farms.GetCropTypes)
// 	// router.GET("/api/v1/crops/crop/:cropid", middleware.Authenticate(farms.GetCropFarms))
// 	router.GET("/api/v1/crops/crop/:cropname", middleware.Authenticate(farms.GetCropTypeFarms))

// 	router.GET("/api/v1/farm/items", farms.GetItems)
// 	router.GET("/api/v1/farm/products", farms.GetProducts)
// 	router.GET("/api/v1/farm/tools", farms.GetTools)
// 	router.GET("/api/v1/farm/items/categories", farms.GetItemCategories)

// 	router.POST("/api/v1/farm/product", farms.CreateProduct)
// 	router.PUT("/api/v1/farm/product/:id", farms.UpdateProduct)
// 	router.DELETE("/api/v1/farm/product/:id", farms.DeleteProduct)

// 	router.POST("/api/v1/farm/tool", farms.CreateTool)
// 	router.PUT("/api/v1/farm/tool/:id", farms.UpdateTool)
// 	router.DELETE("/api/v1/farm/tool/:id", farms.DeleteTool)

//		router.POST("/api/v1/upload/images", utils.UploadImages)
//	}
func RegisterFarmRoutes(router *httprouter.Router) {
	// 🌾 Farm CRUD
	router.POST("/api/v1/farms", middleware.Authenticate(farms.CreateFarm))
	router.GET("/api/v1/farms", farms.GetPaginatedFarms)
	router.GET("/api/v1/farms/:id", middleware.OptionalAuth(farms.GetFarm))
	router.PUT("/api/v1/farms/:id", middleware.Authenticate(farms.EditFarm))
	router.DELETE("/api/v1/farms/:id", middleware.Authenticate(farms.DeleteFarm))

	// 🌱 Crops (within farm)
	router.POST("/api/v1/farms/:id/crops", middleware.Authenticate(farms.AddCrop))
	router.PUT("/api/v1/farms/:id/crops/:cropid", middleware.Authenticate(farms.EditCrop))
	router.DELETE("/api/v1/farms/:id/crops/:cropid", middleware.Authenticate(farms.DeleteCrop))
	router.PUT("/api/v1/farms/:id/crops/:cropid/buy", middleware.Authenticate(farms.BuyCrop))

	// 📊 Dashboard
	router.GET("/api/v1/dash/farms", middleware.Authenticate(farms.GetFarmDash))

	// 📦 Farm Orders
	router.GET("/api/v1/orders/mine", middleware.Authenticate(farms.GetMyFarmOrders))           // my own farm orders
	router.GET("/api/v1/orders/incoming", middleware.Authenticate(farms.GetIncomingFarmOrders)) // orders from buyers to me
	router.POST("/api/v1/farmorders/:id/accept", middleware.Authenticate(farms.AcceptOrder))
	router.POST("/api/v1/farmorders/:id/reject", middleware.Authenticate(farms.RejectOrder))
	router.POST("/api/v1/farmorders/:id/deliver", middleware.Authenticate(farms.MarkOrderDelivered))
	router.POST("/api/v1/farmorders/:id/markpaid", middleware.Authenticate(farms.MarkOrderPaid))
	router.GET("/api/v1/farmorders/:id/receipt", middleware.Authenticate(farms.DownloadReceipt))

	// 🌾 Crop catalogue & type browsing
	router.GET("/api/v1/crops", farms.GetFilteredCrops)                                         // for search/filter
	router.GET("/api/v1/crops/catalogue", farms.GetCropCatalogue)                               // full list
	router.GET("/api/v1/crops/precatalogue", farms.GetPreCropCatalogue)                         // pre-published
	router.GET("/api/v1/crops/types", farms.GetCropTypes)                                       // types list
	router.GET("/api/v1/crops/crop/:cropname", middleware.OptionalAuth(farms.GetCropTypeFarms)) // farms by crop name

	// 🛒 Items, Products, Tools
	// -- GET
	// router.GET("/api/v1/farm/products", farms.GetProducts)
	// router.GET("/api/v1/farm/tools", farms.GetTools)

	router.GET("/api/v1/farm/items", farms.GetItems)
	router.GET("/api/v1/farm/items/categories", farms.GetItemCategories)

	// -- Products (CRUD)
	router.POST("/api/v1/farm/product", farms.CreateProduct)
	router.PUT("/api/v1/farm/product/:id", farms.UpdateProduct)
	router.DELETE("/api/v1/farm/product/:id", farms.DeleteProduct)

	// -- Tools (CRUD)
	router.POST("/api/v1/farm/tool", farms.CreateTool)
	router.PUT("/api/v1/farm/tool/:id", farms.UpdateTool)
	router.DELETE("/api/v1/farm/tool/:id", farms.DeleteTool)

	// 🖼 Upload
	router.POST("/api/v1/upload/images", utils.UploadImages)
}

func AddMerchRoutes(router *httprouter.Router) {
	router.POST("/api/v1/merch/:entityType/:eventid", middleware.Authenticate(merch.CreateMerch))
	router.POST("/api/v1/merch/:entityType/:eventid/:merchid/buy", ratelim.RateLimit(middleware.Authenticate(merch.BuyMerch)))
	router.GET("/api/v1/merch/:entityType/:eventid", merch.GetMerchs)
	router.GET("/api/v1/merch/:entityType/:eventid/:merchid", merch.GetMerch)
	router.GET("/api/v1/merch/:entityType", merch.GetMerchPage)
	router.PUT("/api/v1/merch/:entityType/:eventid/:merchid", middleware.Authenticate(merch.EditMerch))
	router.DELETE("/api/v1/merch/:entityType/:eventid/:merchid", middleware.Authenticate(merch.DeleteMerch))

	router.POST("/api/v1/merch/:entityType/:eventid/:merchid/payment-session", middleware.Authenticate(merch.CreateMerchPaymentSession))
	router.POST("/api/v1/merch/:entityType/:eventid/:merchid/confirm-purchase", middleware.Authenticate(merch.ConfirmMerchPurchase))

}

func AddTicketRoutes(router *httprouter.Router) {
	router.POST("/api/v1/ticket/event/:eventid", ratelim.RateLimit(middleware.Authenticate(tickets.CreateTicket)))
	router.GET("/api/v1/ticket/event/:eventid", ratelim.RateLimit(tickets.GetTickets))
	router.GET("/api/v1/ticket/event/:eventid/:ticketid", ratelim.RateLimit(tickets.GetTicket))
	router.PUT("/api/v1/ticket/event/:eventid/:ticketid", ratelim.RateLimit(middleware.Authenticate(tickets.EditTicket)))
	router.DELETE("/api/v1/ticket/event/:eventid/:ticketid", ratelim.RateLimit(middleware.Authenticate(tickets.DeleteTicket)))
	router.POST("/api/v1/ticket/event/:eventid/:ticketid/buy", ratelim.RateLimit(middleware.Authenticate(tickets.BuyTicket)))
	router.GET("/api/v1/ticket/verify/:eventid", ratelim.RateLimit(tickets.VerifyTicket))
	router.GET("/api/v1/ticket/print/:eventid", ratelim.RateLimit(tickets.PrintTicket))

	// router.POST("/api/v1/ticket/confirm-purchase", middleware.Authenticate(ConfirmTicketPurchase))
	router.POST("/api/v1/ticket/event/:eventid/:ticketid/payment-session", ratelim.RateLimit(middleware.Authenticate(tickets.CreateTicketPaymentSession)))
	router.GET("/api/v1/events/event/:eventid/updates", ratelim.RateLimit(tickets.EventUpdates))
	// router.POST("/api/v1/seats/event/:eventid/:ticketid", ratelim.RateLimit(middleware.Authenticate(bookSeats)))
	router.POST("/api/v1/ticket/event/:eventid/:ticketid/confirm-purchase", ratelim.RateLimit(middleware.Authenticate(tickets.ConfirmTicketPurchase)))

	router.GET("/api/v1/seats/:eventid/available-seats", ratelim.RateLimit(tickets.GetAvailableSeats))
	router.POST("/api/v1/seats/:eventid/lock-seats", ratelim.RateLimit(tickets.LockSeats))
	router.POST("/api/v1/seats/:eventid/unlock-seats", ratelim.RateLimit(tickets.UnlockSeats))
	router.POST("/api/v1/seats/:eventid/ticket/:ticketid/confirm-purchase", ratelim.RateLimit(tickets.ConfirmSeatPurchase))
	router.GET("/api/v1/ticket/event/:eventid/:ticketid/seats", ratelim.RateLimit(tickets.GetTicketSeats))
}

func AddSuggestionsRoutes(router *httprouter.Router) {
	router.GET("/api/v1/suggestions/places/nearby", ratelim.RateLimit(suggestions.GetNearbyPlaces))
	router.GET("/api/v1/suggestions/places", ratelim.RateLimit(suggestions.SuggestionsHandler))
	router.GET("/api/v1/suggestions/follow", ratelim.RateLimit(middleware.Authenticate(suggestions.SuggestFollowers)))
}

func AddQnARoutes(router *httprouter.Router) {
	// Questions
	router.GET("/api/v1/questions", qna.ListQuestions)       // list all questions
	router.GET("/api/v1/questions/:id", qna.GetQuestionByID) // get a single question
	router.POST("/api/v1/questions", qna.CreateQuestion)     // create a new question

	// Answers
	router.GET("/api/v1/answers", qna.ListAnswersByPostID)      // list answers for a question
	router.POST("/api/v1/answers", qna.CreateAnswer)            // submit a new answer
	router.POST("/api/v1/answers/:id/vote", qna.VoteAnswer)     // upvote or downvote an answer
	router.POST("/api/v1/answers/:id/best", qna.MarkBestAnswer) // mark an answer as best
	router.POST("/api/v1/answers/:id/report", qna.ReportAnswer) // report an answer

	// Replies
	router.POST("/api/v1/replies", qna.CreateReply) // reply to an answer
}

func AddReviewsRoutes(router *httprouter.Router) {
	router.GET("/api/v1/reviews/:entityType/:entityId", ratelim.RateLimit(middleware.Authenticate(reviews.GetReviews)))
	router.GET("/api/v1/reviews/:entityType/:entityId/:reviewId", ratelim.RateLimit(middleware.Authenticate(reviews.GetReview)))
	router.POST("/api/v1/reviews/:entityType/:entityId", ratelim.RateLimit(middleware.Authenticate(reviews.AddReview)))
	router.PUT("/api/v1/reviews/:entityType/:entityId/:reviewId", ratelim.RateLimit(middleware.Authenticate(reviews.EditReview)))
	router.DELETE("/api/v1/reviews/:entityType/:entityId/:reviewId", ratelim.RateLimit(middleware.Authenticate(reviews.DeleteReview)))
}

func AddMediaRoutes(router *httprouter.Router) {
	// Set up routes with middlewares
	router.POST("/api/v1/media/:entitytype/:entityid", ratelim.RateLimit(middleware.Authenticate(media.AddMedia)))
	router.GET("/api/v1/media/:entitytype/:entityid/:id", ratelim.RateLimit(media.GetMedia))
	router.PUT("/api/v1/media/:entitytype/:entityid/:id", ratelim.RateLimit(middleware.Authenticate(media.EditMedia)))
	router.GET("/api/v1/media/:entitytype/:entityid", ratelim.RateLimit(media.GetMedias))
	router.DELETE("/api/v1/media/:entitytype/:entityid/:id", ratelim.RateLimit(middleware.Authenticate(media.DeleteMedia)))
}

func AddPostRoutes(router *httprouter.Router) {
	router.POST("/api/v1/posts/post", middleware.Authenticate(posts.CreatePost))
	router.PUT("/api/v1/posts/post/:id", middleware.Authenticate(posts.UpdatePost))
	router.GET("/api/v1/posts/:id", middleware.Authenticate(posts.GetPost))
	router.GET("/api/v1/posts", middleware.Authenticate(posts.GetAllPosts))

}

func AddPlaceRoutes(router *httprouter.Router) {
	router.GET("/api/v1/places/places", ratelim.RateLimit(places.GetPlaces))
	router.POST("/api/v1/places/place", middleware.Authenticate(places.CreatePlace))
	router.GET("/api/v1/places/place/:placeid", places.GetPlace)
	router.GET("/api/v1/places/place-details", places.GetPlaceQ)
	router.PUT("/api/v1/places/place/:placeid", middleware.Authenticate(places.EditPlace))
	router.DELETE("/api/v1/places/place/:placeid", middleware.Authenticate(places.DeletePlace))

	router.POST("/api/v1/places/menu/:placeid", middleware.Authenticate(menu.CreateMenu))
	router.GET("/api/v1/places/menu/:placeid", menu.GetMenus)
	router.GET("/api/v1/places/menu/:placeid/:menuid/stock", menu.GetStock)
	router.POST("/api/v1/places/menu/:placeid/:menuid/buy", menu.BuyMenu)
	router.GET("/api/v1/places/menu/:placeid/:menuid", menu.GetMenu)
	router.PUT("/api/v1/places/menu/:placeid/:menuid", middleware.Authenticate(menu.EditMenu))
	router.DELETE("/api/v1/places/menu/:placeid/:menuid", middleware.Authenticate(menu.DeleteMenu))

	router.POST("/api/v1/places/menu/:placeid/:menuid/payment-session", middleware.Authenticate(menu.CreateMenuPaymentSession))
	router.POST("/api/v1/places/menu/:placeid/:menuid/confirm-purchase", middleware.Authenticate(menu.ConfirmMenuPurchase))

}

func AddProfileRoutes(router *httprouter.Router) {
	router.GET("/api/v1/profile/profile", middleware.Authenticate(profile.GetProfile))
	router.PUT("/api/v1/profile/edit", middleware.Authenticate(profile.EditProfile))
	router.PUT("/api/v1/profile/avatar", middleware.Authenticate(profile.EditProfilePic))
	router.PUT("/api/v1/profile/banner", middleware.Authenticate(profile.EditProfileBanner))
	router.DELETE("/api/v1/profile/delete", middleware.Authenticate(profile.DeleteProfile))

	router.GET("/api/v1/user/:username", ratelim.RateLimit(profile.GetUserProfile))
	router.GET("/api/v1/user/:username/data", ratelim.RateLimit(middleware.Authenticate(userdata.GetUserProfileData)))
	router.GET("/api/v1/user/:username/udata", ratelim.RateLimit(middleware.Authenticate(userdata.GetOtherUserProfileData)))

	router.PUT("/api/v1/follows/:id", ratelim.RateLimit(middleware.Authenticate(profile.ToggleFollow)))
	router.DELETE("/api/v1/follows/:id", ratelim.RateLimit(middleware.Authenticate(profile.ToggleUnFollow)))
	router.GET("/api/v1/follows/:id/status", ratelim.RateLimit(middleware.Authenticate(profile.DoesFollow)))
	router.GET("/api/v1/followers/:id", ratelim.RateLimit(middleware.Authenticate(profile.GetFollowers)))
	router.GET("/api/v1/following/:id", ratelim.RateLimit(middleware.Authenticate(profile.GetFollowing)))

}

func AddArtistRoutes(router *httprouter.Router) {
	router.GET("/api/v1/artists", artists.GetAllArtists)
	router.GET("/api/v1/artists/:id", artists.GetArtistByID)
	router.DELETE("/api/v1/artists/:id", artists.DeleteArtistByID)
	router.GET("/api/v1/events/event/:eventid/artists", artists.GetArtistsByEvent)
	router.POST("/api/v1/artists", artists.CreateArtist)
	router.PUT("/api/v1/artists/:id", artists.UpdateArtist)

	router.GET("/api/v1/artists/:id/songs", artists.GetArtistsSongs)
	router.POST("/api/v1/artists/:id/songs", artists.PostNewSong)
	router.DELETE("/api/v1/artists/:id/songs/:songId", artists.DeleteSong)
	router.PUT("/api/v1/artists/:id/songs/:songId/edit", artists.EditSong) // ← new route

	router.GET("/api/v1/artists/:id/albums", artists.GetArtistsAlbums)
	router.GET("/api/v1/artists/:id/posts", artists.GetArtistsPosts)
	router.GET("/api/v1/artists/:id/merch", artists.GetArtistsMerch)
	router.GET("/api/v1/artists/:id/behindthescenes", artists.GetBTS)

	router.PUT("/api/v1/artists/:id/events/addtoevent", artists.AddArtistToEvent)
	router.GET("/api/v1/artists/:id/events", artists.GetArtistEvents)
	router.POST("/api/v1/artists/:id/events", artists.CreateArtistEvent)
	router.PUT("/api/v1/artists/:id/events", artists.UpdateArtistEvent)
	router.DELETE("/api/v1/artists/:id/events", artists.DeleteArtistEvent)
}

func AddCartoonRoutes(router *httprouter.Router) {
	router.GET("/api/v1/cartoons", cartoons.GetAllCartoons)
	router.GET("/api/v1/cartoons/:id", cartoons.GetCartoonByID)
	router.GET("/api/v1/events/event/:eventid/cartoons", cartoons.GetCartoonsByEvent)
	router.POST("/api/v1/cartoons", cartoons.CreateCartoon)
	router.PUT("/api/v1/cartoons/:id", cartoons.UpdateCartoon)

}

func AddMapRoutes(router *httprouter.Router) {
	router.GET("/api/v1/maps/config/:entity", maps.GetMapConfig)
	router.GET("/api/v1/maps/markers/:entity", maps.GetMapMarkers)

}

func AddItineraryRoutes(router *httprouter.Router) {
	router.GET("/api/v1/itineraries", itinerary.GetItineraries)               //Fetch all itineraries
	router.POST("/api/v1/itineraries", itinerary.CreateItinerary)             //Create a new itinerary
	router.GET("/api/v1/itineraries/all/:id", itinerary.GetItinerary)         //Fetch a single itinerary
	router.PUT("/api/v1/itineraries/:id", itinerary.UpdateItinerary)          //Update an itinerary
	router.DELETE("/api/v1/itineraries/:id", itinerary.DeleteItinerary)       //Delete an itinerary
	router.GET("/api/v1/itineraries/search", itinerary.SearchItineraries)     //Search an itinerary
	router.POST("/api/v1/itineraries/:id/fork", itinerary.ForkItinerary)      //Fork a new itinerary
	router.PUT("/api/v1/itineraries/:id/publish", itinerary.PublishItinerary) //Publish an itinerary
}

func AddUtilityRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	router.GET("/api/v1/check-file/:hash", rateLimiter.Limit(middleware.Authenticate(feed.CheckUserInFile)))
	router.GET("/api/v1/csrf", rateLimiter.Limit(middleware.Authenticate(utils.CSRF)))
}

func AddFeedRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	router.GET("/api/v1/feed/feed", middleware.Authenticate(feed.GetPosts))
	router.GET("/api/v1/feed/post/:postid", rateLimiter.Limit(feed.GetPost))
	// router.POST("/api/v1/feed/repost/:postid", feed.Repost)
	// router.DELETE("/api/v1/feed/repost/:postid", feed.DeleteRepost)
	router.POST("/api/v1/feed/post", ratelim.RateLimit(middleware.Authenticate(feed.CreateTweetPost)))
	router.PUT("/api/v1/feed/post/:postid", middleware.Authenticate(feed.EditPost))
	router.DELETE("/api/v1/feed/post/:postid", middleware.Authenticate(feed.DeletePost))
}

func AddSettingsRoutes(router *httprouter.Router) {
	router.GET("/api/v1/settings/init/:userid", middleware.Authenticate(settings.InitUserSettings))
	// router.GET("/api/v1/settings/setting/:type", getUserSettings)
	router.GET("/api/v1/settings/all", ratelim.RateLimit(middleware.Authenticate(settings.GetUserSettings)))
	router.PUT("/api/v1/settings/setting/:type", ratelim.RateLimit(middleware.Authenticate(settings.UpdateUserSetting)))
}

func AddAdsRoutes(router *httprouter.Router) {
	router.GET("/api/v1/sda/sda", ratelim.RateLimit(middleware.Authenticate(ads.GetAds)))
}

func AddSearchRoutes(router *httprouter.Router) {
	router.GET("/api/v1/ac", search.Autocompleter)
	router.GET("/api/v1/search/:entityType", ratelim.RateLimit(search.SearchHandler))
	router.POST("/emitted", search.EventHandler)
}

func AddMiscRoutes(router *httprouter.Router) {
	// Example Routes
	// router.GET("/", ratelim.RateLimit(wrapHandler(proxyWithCircuitBreaker("frontend-service"))))

	// router.GET("/api/v1/search/:entityType", ratelim.RateLimit(searchEvents))

	// router.POST("/api/v1/check-file", rateLimiter.Limit(filecheck.CheckFileExists))
	// router.POST("/api/v1/upload", rateLimiter.Limit(filecheck.UploadFile))
	// router.POST("/api/v1/feed/remhash", rateLimiter.Limit(filecheck.RemoveUserFile))

	// router.POST("/agi/home_feed_section", ratelim.RateLimit(middleware.Authenticate(agi.GetHomeFeed)))
	// router.GET("/resize/:folder/*filename", cdn.ServeStatic)

}
