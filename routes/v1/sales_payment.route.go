package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// SalesPaymentRoutes — "Pembayaran Masuk". No PUT/DELETE: the seeded
// permissions are invoice-sales-payment-{list,create,verify} only.
func SalesPaymentRoutes(router fiber.Router, ctrl *controller.SalesPaymentController) {
	g := router.Group("/sales-payments")
	g.Get("/", middlewares.RequirePermission("invoice-sales-payment-list"), ctrl.List)
	g.Get("/:id", middlewares.RequirePermission("invoice-sales-payment-list"), ctrl.Get)
	g.Post("/", middlewares.RequirePermission("invoice-sales-payment-create"), ctrl.Create)
	g.Post("/:id/verify", middlewares.RequirePermission("invoice-sales-payment-verify"), ctrl.Verify)
}
