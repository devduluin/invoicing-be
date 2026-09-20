package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// SalesReceiptRoutes — "Kuitansi Penjualan". Update replaces the receipt; Delete is a soft delete.
func SalesReceiptRoutes(router fiber.Router, ctrl *controller.SalesReceiptController) {
	g := router.Group("/sales-receipts")
	g.Get("/", middlewares.RequirePermission("invoice-receipt-list"), ctrl.List)
	g.Get("/next-number", middlewares.RequirePermission("invoice-receipt-create"), ctrl.PreviewNumber)
	g.Get("/:id", middlewares.RequirePermission("invoice-receipt-list"), ctrl.Get)
	g.Post("/", middlewares.RequirePermission("invoice-receipt-create"), ctrl.Create)
	g.Put("/:id", middlewares.RequirePermission("invoice-receipt-update"), ctrl.Update)
	g.Delete("/:id", middlewares.RequirePermission("invoice-receipt-delete"), ctrl.Delete)
}
