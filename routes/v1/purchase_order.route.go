package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// PurchaseOrderRoutes — pre-bill purchase documents ("Purchase Order").
func PurchaseOrderRoutes(router fiber.Router, ctrl *controller.PurchaseOrderController) {
	g := router.Group("/purchase-orders")
	g.Get("/", middlewares.RequirePermission("invoice-purchase-order-list"), ctrl.List)
	g.Get("/next-number", middlewares.RequirePermission("invoice-purchase-order-create"), ctrl.PreviewNumber)
	g.Get("/:id", middlewares.RequirePermission("invoice-purchase-order-list"), ctrl.Get)
	g.Post("/", middlewares.RequirePermission("invoice-purchase-order-create"), ctrl.Create)
	g.Put("/:id", middlewares.RequirePermission("invoice-purchase-order-update"), ctrl.Update)
	g.Delete("/:id", middlewares.RequirePermission("invoice-purchase-order-delete"), ctrl.Delete)
	// Confirm/cancel/draft are lifecycle updates, not a new permission tier —
	// gated the same as Update, matching Sales Order's convention.
	g.Put("/:id/template", middlewares.RequirePermission("invoice-purchase-order-update"), ctrl.SetTemplate)
	g.Post("/:id/confirm", middlewares.RequirePermission("invoice-purchase-order-update"), ctrl.Confirm)
	g.Post("/:id/cancel", middlewares.RequirePermission("invoice-purchase-order-update"), ctrl.Cancel)
	g.Post("/:id/draft", middlewares.RequirePermission("invoice-purchase-order-update"), ctrl.BackToDraft)
}
