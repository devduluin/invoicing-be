package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// SalesInvoiceRoutes — covers both Invoice Penjualan and Invoice Uang Muka
// (the ?kind= query param on List distinguishes them; both share one
// permission set, invoice-sales-invoice-*, since there's no separate
// down-payment slug in the SSO seeder).
func SalesInvoiceRoutes(router fiber.Router, ctrl *controller.SalesInvoiceController) {
	g := router.Group("/sales-invoices")
	g.Get("/", middlewares.RequirePermission("invoice-sales-invoice-list"), ctrl.List)
	g.Get("/next-number", middlewares.RequirePermission("invoice-sales-invoice-create"), ctrl.PreviewNumber)
	g.Get("/:id", middlewares.RequirePermission("invoice-sales-invoice-list"), ctrl.Get)
	g.Post("/", middlewares.RequirePermission("invoice-sales-invoice-create"), ctrl.Create)
	g.Put("/:id", middlewares.RequirePermission("invoice-sales-invoice-update"), ctrl.Update)
	g.Delete("/:id", middlewares.RequirePermission("invoice-sales-invoice-delete"), ctrl.Delete)
	g.Post("/:id/confirm", middlewares.RequirePermission("invoice-sales-invoice-update"), ctrl.Confirm)
	g.Post("/:id/cancel", middlewares.RequirePermission("invoice-sales-invoice-update"), ctrl.Cancel)
	g.Post("/:id/draft", middlewares.RequirePermission("invoice-sales-invoice-update"), ctrl.BackToDraft)
}
