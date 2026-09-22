package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// DocumentConfigurationRoutes — reading is open to anyone who can list at least one document (the
// PDF of a document must look the same for everyone who may see it); changing or resetting needs
// invoice-template-update.
func DocumentConfigurationRoutes(router fiber.Router, ctrl *controller.DocumentConfigurationController) {
	read := middlewares.RequirePermission(
		"invoice-template-list", "invoice-sales-order-list", "invoice-sales-invoice-list", "invoice-receipt-list",
		"invoice-delivery-note-list", "invoice-purchase-order-list", "invoice-bill-list",
		"invoice-purchase-receipt-list", "invoice-goods-receipt-list",
	)
	g := router.Group("/document-configurations")
	g.Get("/", read, ctrl.List)
	g.Get("/:doc_type", read, ctrl.Get)
	g.Put("/:doc_type", middlewares.RequirePermission("invoice-template-update"), ctrl.Save)
	g.Delete("/:doc_type", middlewares.RequirePermission("invoice-template-update"), ctrl.Reset)
}
