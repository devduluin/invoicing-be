package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// SalesOrderRoutes — pre-invoice sales documents ("Order Penjualan").
func SalesOrderRoutes(router fiber.Router, ctrl *controller.SalesOrderController) {
	g := router.Group("/sales-orders")
	g.Get("/", middlewares.RequirePermission("invoice-sales-order-list"), ctrl.List)
	g.Get("/next-number", middlewares.RequirePermission("invoice-sales-order-create"), ctrl.PreviewNumber)
	g.Get("/:id", middlewares.RequirePermission("invoice-sales-order-list"), ctrl.Get)
	g.Post("/", middlewares.RequirePermission("invoice-sales-order-create"), ctrl.Create)
	g.Put("/:id", middlewares.RequirePermission("invoice-sales-order-update"), ctrl.Update)
	g.Delete("/:id", middlewares.RequirePermission("invoice-sales-order-delete"), ctrl.Delete)
	// Confirm/cancel/draft are lifecycle updates, not a new permission tier —
	// gated the same as Update, matching Journal Entry's post/draft actions.
	g.Put("/:id/template", middlewares.RequirePermission("invoice-sales-order-update"), ctrl.SetTemplate)
	g.Post("/:id/confirm", middlewares.RequirePermission("invoice-sales-order-update"), ctrl.Confirm)
	g.Post("/:id/cancel", middlewares.RequirePermission("invoice-sales-order-update"), ctrl.Cancel)
	g.Post("/:id/draft", middlewares.RequirePermission("invoice-sales-order-update"), ctrl.BackToDraft)
}
