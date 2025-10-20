package routes

import (
	"naevis/ratelim"

	"github.com/julienschmidt/httprouter"
)

func RoutesWrapper(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	AddActivityRoutes(router, rateLimiter)
	AddAdminRoutes(router, rateLimiter)
	AddAdsRoutes(router, rateLimiter)
	AddAnalyticsRoutes(router, rateLimiter)
	AddArtistRoutes(router, rateLimiter)
	AddBaitoRoutes(router, rateLimiter)
	AddBannerRoutes(router, rateLimiter)
	AddBeatRoutes(router, rateLimiter)
	AddBookingRoutes(router, rateLimiter)
	AddAuthRoutes(router, rateLimiter)
	AddCartRoutes(router, rateLimiter)
	AddDiscordRoutes(router, rateLimiter)
	AddCommentsRoutes(router, rateLimiter)
	AddEventsRoutes(router, rateLimiter)
	AddFanmadeRoutes(router, rateLimiter)
	RegisterFarmRoutes(router, rateLimiter)
	AddFeedRoutes(router, rateLimiter)
	AddHomeRoutes(router, rateLimiter)
	AddHashtagRoutes(router, rateLimiter)
	AddItineraryRoutes(router, rateLimiter)
	AddJobRoutes(router, rateLimiter)
	AddMapRoutes(router, rateLimiter)
	AddMediaRoutes(router, rateLimiter)
	AddMerchRoutes(router, rateLimiter)
	AddPayRoutes(router, rateLimiter)
	AddPlaceRoutes(router, rateLimiter)
	AddPlaceTabRoutes(router, rateLimiter)
	AddPostRoutes(router, rateLimiter)
	AddProductRoutes(router, rateLimiter)
	AddProfileRoutes(router, rateLimiter)
	AddRecipeRoutes(router, rateLimiter)
	AddReportRoutes(router, rateLimiter)
	AddReviewsRoutes(router, rateLimiter)
	AddSearchRoutes(router, rateLimiter)
	AddSettingsRoutes(router, rateLimiter)
	AddSuggestionsRoutes(router, rateLimiter)
	AddTicketRoutes(router, rateLimiter)
	AddUtilityRoutes(router, rateLimiter)
	AddMiscRoutes(router, rateLimiter)
}
