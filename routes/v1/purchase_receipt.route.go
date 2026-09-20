package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// PurchaseReceiptRoutes — "Purchase Receipt", the AP mirror of Sales Receipt.
// Update replaces the receipt; Delete is a soft delete.
func PurchaseReceiptRoutes(router fiber.Router, ctrl *controller.PurchaseReceiptController) {
	g := router.Group("/purchase-receipts")
	g.Get("/", middlewares.RequirePermission("invoice-purchase-receipt-list"), ctrl.List)
	g.Get("/next-number", middlewares.RequirePermission("invoice-purchase-receipt-create"), ctrl.PreviewNumber)
	g.Get("/:id", middlewares.RequirePermission("invoice-purchase-receipt-list"), ctrl.Get)
	g.Post("/", middlewares.RequirePermission("invoice-purchase-receipt-create"), ctrl.Create)
	g.Put("/:id", middlewares.RequirePermission("invoice-purchase-receipt-update"), ctrl.Update)
	g.Delete("/:id", middlewares.RequirePermission("invoice-purchase-receipt-delete"), ctrl.Delete)
}
