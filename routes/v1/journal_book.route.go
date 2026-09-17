package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// JournalBookRoutes — "Buku Jurnal" master data (Odoo: account.journal).
func JournalBookRoutes(router fiber.Router, ctrl *controller.JournalBookController) {
	g := router.Group("/journal-books")
	g.Get("/", middlewares.RequirePermission("invoice-journalbook-list"), ctrl.List)
	g.Get("/:id", middlewares.RequirePermission("invoice-journalbook-list"), ctrl.Get)
	g.Post("/", middlewares.RequirePermission("invoice-journalbook-create"), ctrl.Create)
	g.Put("/:id", middlewares.RequirePermission("invoice-journalbook-update"), ctrl.Update)
	g.Delete("/:id", middlewares.RequirePermission("invoice-journalbook-delete"), ctrl.Delete)
}
