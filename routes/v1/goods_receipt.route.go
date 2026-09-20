package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// GoodsReceiptRoutes — "Penerimaan Barang". Update replaces the record; Delete is a soft delete.
func GoodsReceiptRoutes(router fiber.Router, ctrl *controller.GoodsReceiptController) {
	g := router.Group("/goods-receipts")
	g.Get("/", middlewares.RequirePermission("invoice-goods-receipt-list"), ctrl.List)
	g.Get("/:id", middlewares.RequirePermission("invoice-goods-receipt-list"), ctrl.Get)
	g.Post("/", middlewares.RequirePermission("invoice-goods-receipt-create"), ctrl.Create)
	g.Put("/:id", middlewares.RequirePermission("invoice-goods-receipt-update"), ctrl.Update)
	g.Delete("/:id", middlewares.RequirePermission("invoice-goods-receipt-delete"), ctrl.Delete)
}
