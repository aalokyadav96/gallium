package routes

import (
	"naevis/middleware"
	"naevis/pay"
	"naevis/ratelim"

	"github.com/julienschmidt/httprouter"
)

// AddPayRoutes wires PaymentService handlers to the router
func AddPayRoutes(router *httprouter.Router, rateLimiter *ratelim.RateLimiter) {
	// Create a single instance of PaymentService
	payService := pay.NewPaymentService()

	// Register resolvers (if using DI for repositories/services)
	payService.RegisterDefaultResolvers()

	// Wallet routes
	router.GET("/api/p1/wallet/balance",
		middleware.Chain(
			rateLimiter.Limit,
			middleware.Authenticate,
			middleware.RequireRoles("user"),
		)(payService.GetBalance),
	)

	router.POST("/api/p1/wallet/topup",
		middleware.Chain(
			rateLimiter.Limit,
			middleware.Authenticate,
			middleware.RequireRoles("user"),
			middleware.WithTxn, // ensures transaction is started
		)(payService.TopUp),
	)

	router.POST("/api/p1/wallet/pay",
		middleware.Chain(
			rateLimiter.Limit,
			middleware.Authenticate,
			middleware.RequireRoles("user"),
			middleware.WithTxn,
		)(payService.Pay),
	)

	// Transfer & Refund
	router.POST("/api/p1/wallet/transfer",
		middleware.Chain(
			rateLimiter.Limit,
			middleware.Authenticate,
			middleware.RequireRoles("user"),
			middleware.WithTxn,
		)(payService.Transfer),
	)

	router.POST("/api/p1/wallet/refund",
		middleware.Chain(
			rateLimiter.Limit,
			middleware.Authenticate,
			middleware.RequireRoles("user"),
			middleware.WithTxn,
		)(payService.Refund),
	)

	// List transactions (no txn wrapper needed, only reads)
	router.GET("/api/p1/wallet/transactions",
		middleware.Chain(
			rateLimiter.Limit,
			middleware.Authenticate,
			middleware.RequireRoles("user"),
		)(payService.ListTransactions),
	)
}
