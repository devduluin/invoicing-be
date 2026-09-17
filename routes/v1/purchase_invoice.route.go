package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// PurchaseInvoiceRoutes — vendor billing documents ("Purchase Invoice").
// Gated by invoice-bill-* — the SSO seeder's standard AP term for this
// document ("Bill"); the UI label stays "Purchase Invoice" (see the
// Purchase Order/Invoice/Receipt build plan for the naming rationale).
func PurchaseInvoiceRoutes(router fiber.Router, ctrl *controller.PurchaseInvoiceController) {
	g := router.Group("/purchase-invoices")
	g.Get("/", middlewares.RequirePermission("invoice-bill-list"), ctrl.List)
	g.Get("/next-number", middlewares.RequirePermission("invoice-bill-create"), ctrl.PreviewNumber)
	g.Get("/:id", middlewares.RequirePermission("invoice-bill-list"), ctrl.Get)
	g.Post("/", middlewares.RequirePermission("invoice-bill-create"), ctrl.Create)
	g.Put("/:id", middlewares.RequirePermission("invoice-bill-update"), ctrl.Update)
	g.Delete("/:id", middlewares.RequirePermission("invoice-bill-delete"), ctrl.Delete)
	g.Post("/:id/confirm", middlewares.RequirePermission("invoice-bill-update"), ctrl.Confirm)
	g.Post("/:id/cancel", middlewares.RequirePermission("invoice-bill-update"), ctrl.Cancel)
	g.Post("/:id/draft", middlewares.RequirePermission("invoice-bill-update"), ctrl.BackToDraft)
}
