package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// GoodsReceiptRoutes — "Penerimaan Barang". No PUT/DELETE: the seeded
// permissions are invoice-goods-receipt-{list,create} only.
func GoodsReceiptRoutes(router fiber.Router, ctrl *controller.GoodsReceiptController) {
	g := router.Group("/goods-receipts")
	g.Get("/", middlewares.RequirePermission("invoice-goods-receipt-list"), ctrl.List)
	g.Get("/:id", middlewares.RequirePermission("invoice-goods-receipt-list"), ctrl.Get)
	g.Post("/", middlewares.RequirePermission("invoice-goods-receipt-create"), ctrl.Create)
}
