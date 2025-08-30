package routes

import (
	places "naevis/places/tabs"
	"naevis/ratelim"

	"github.com/julienschmidt/httprouter"
)

// 🍽️ Restaurant / Café → Menu
func DisplayPlaceMenu(router *httprouter.Router) {
	router.GET("/api/v1/place/:placeid/menu", places.GetMenuTab)
	router.POST("/api/v1/place/:placeid/menu", places.PostMenuTab)
	router.PUT("/api/v1/place/:placeid/menu/:itemId", places.PutMenuTab)
	router.DELETE("/api/v1/place/:placeid/menu/:itemId", places.DeleteMenuTab)
	router.POST("/api/v1/place/:placeid/menu/:itemId/order", places.PostMenuOrder)
}

// 🏨 Hotel → Rooms
func DisplayPlaceRooms(router *httprouter.Router) {
	router.GET("/api/v1/place/:placeid/rooms", places.GetRooms)
	router.GET("/api/v1/place/:placeid/rooms/:roomId", places.GetRoom)
	router.POST("/api/v1/place/:placeid/rooms", places.PostRoom)
	router.PUT("/api/v1/place/:placeid/rooms/:roomId", places.PutRoom)
	router.DELETE("/api/v1/place/:placeid/rooms/:roomId", places.DeleteRoom)
}

// 🌳 Park → Facilities
func DisplayPlaceFacilities(router *httprouter.Router) {
	router.GET("/api/v1/place/:placeid/facilities", places.GetFacilities)
	router.POST("/api/v1/place/:placeid/facilities", places.PostFacility)
	router.PUT("/api/v1/place/:placeid/facilities/:facilityId", places.PutFacility)
	router.GET("/api/v1/place/:placeid/facilities/:facilityId", places.GetFacility)
	router.DELETE("/api/v1/place/:placeid/facilities/:facilityId", places.DeleteFacility)
}

// 🏢 Business → Services
func DisplayPlaceServices(router *httprouter.Router) {
	router.GET("/api/v1/place/:placeid/services", places.GetServices)
	router.POST("/api/v1/place/:placeid/services", places.PostService)
	router.PUT("/api/v1/place/:placeid/services/:serviceId", places.PutService)
	router.GET("/api/v1/place/:placeid/services/:serviceId", places.GetService)
	router.DELETE("/api/v1/place/:placeid/services/:serviceId", places.DeleteService)
}

// 🛍️ Shop → Products
func DisplayPlaceProducts(router *httprouter.Router) {
	router.GET("/api/v1/place/:placeid/products", places.GetProducts)
	router.POST("/api/v1/place/:placeid/products", places.PostProduct)
	router.PUT("/api/v1/place/:placeid/products/:productId", places.PutProduct)
	router.GET("/api/v1/place/:placeid/products/:productId", places.GetProduct)
	router.DELETE("/api/v1/place/:placeid/products/:productId", places.DeleteProduct)
	router.POST("/api/v1/place/:placeid/products/:productId/buy", places.PostProductPurchase)
}

// 🖼️ Museum → Exhibits
func DisplayPlaceExhibits(router *httprouter.Router) {
	router.GET("/api/v1/place/:placeid/exhibits", places.GetExhibits)
	router.POST("/api/v1/place/:placeid/exhibits", places.PostExhibit)
	router.PUT("/api/v1/place/:placeid/exhibits/:exhibitId", places.PutExhibit)
	router.GET("/api/v1/place/:placeid/exhibits/:exhibitId", places.GetExhibit)
	router.DELETE("/api/v1/place/:placeid/exhibits/:exhibitId", places.DeleteExhibit)
}

// 🏋️ Gym → Membership
func DisplayPlaceMembership(router *httprouter.Router) {
	router.GET("/api/v1/place/:placeid/membership", places.GetMemberships)
	router.POST("/api/v1/place/:placeid/membership", places.PostMembership)
	router.PUT("/api/v1/place/:placeid/membership/:membershipId", places.PutMembership)
	router.GET("/api/v1/place/:placeid/membership/:membershipId", places.GetMembership)
	router.DELETE("/api/v1/place/:placeid/membership/:membershipId", places.DeleteMembership)
	router.POST("/api/v1/place/:placeid/membership/:membershipId/join", places.PostJoinMembership)
}

// 🎭 Theater → Shows
func DisplayPlaceShows(router *httprouter.Router) {
	router.GET("/api/v1/place/:placeid/shows", places.GetShows)
	router.POST("/api/v1/place/:placeid/shows", places.PostShow)
	router.PUT("/api/v1/place/:placeid/shows/:showId", places.PutShow)
	router.GET("/api/v1/place/:placeid/shows/:showId", places.GetShow)
	router.DELETE("/api/v1/place/:placeid/shows/:showId", places.DeleteShow)
	router.POST("/api/v1/place/:placeid/shows/:showId/book", places.PostBookShow)
}

// 🏟️ Arena → Events
func DisplayPlaceEvents(router *httprouter.Router) {
	router.GET("/api/v1/place/:placeid/events", places.GetEvents)
	router.POST("/api/v1/place/:placeid/events", places.PostEvent)
	router.PUT("/api/v1/place/:placeid/events/:eventId", places.PutEvent)
	router.GET("/api/v1/place/:placeid/events/:eventId", places.GetEvent)
	router.DELETE("/api/v1/place/:placeid/events/:eventId", places.DeleteEvent)
	router.POST("/api/v1/place/:placeid/events/:eventId/view", places.PostViewEventDetails)
}

// 💈 Saloon → Slots (if applicable)
func DisplaySaloonSlots(router *httprouter.Router) {
	router.GET("/api/v1/place/:placeid/saloon/slots", places.GetSaloonSlots)
	router.POST("/api/v1/place/:placeid/saloon/slots", places.PostSaloonSlot)
	router.PUT("/api/v1/place/:placeid/saloon/slots/:slotId", places.PutSaloonSlot)
	router.DELETE("/api/v1/place/:placeid/saloon/slots/:slotId", places.DeleteSaloonSlot)
	router.POST("/api/v1/place/:placeid/saloon/slots/:slotId/book", places.BookSaloonSlot)
}

// ❓ Fallback → Generic Place Info
func DisplayPlaceDetailsFallback(router *httprouter.Router) {
	router.GET("/api/v1/place/:placeid/details", places.GetDetailsFallback)
}

func AddPlaceTabRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	DisplayPlaceMenu(router)
	DisplayPlaceRooms(router)
	DisplayPlaceFacilities(router)
	DisplayPlaceServices(router)
	DisplayPlaceProducts(router)
	DisplayPlaceExhibits(router)
	DisplayPlaceMembership(router)
	DisplayPlaceShows(router)
	DisplayPlaceEvents(router)
	DisplaySaloonSlots(router)
	DisplayPlaceDetailsFallback(router)
}
