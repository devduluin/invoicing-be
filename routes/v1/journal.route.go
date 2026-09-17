package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// JournalRoutes — manual journal entries ("Ayat Jurnal").
func JournalRoutes(router fiber.Router, ctrl *controller.JournalController) {
	g := router.Group("/journal-entries")
	g.Get("/", middlewares.RequirePermission("invoice-journal-list"), ctrl.List)
	g.Get("/:id", middlewares.RequirePermission("invoice-journal-list"), ctrl.Get)
	g.Post("/", middlewares.RequirePermission("invoice-journal-create"), ctrl.Create)
	g.Put("/:id", middlewares.RequirePermission("invoice-journal-update"), ctrl.Update)
	g.Delete("/:id", middlewares.RequirePermission("invoice-journal-delete"), ctrl.Delete)
	// Posting/drafting is an update to the entry's lifecycle, not a new
	// permission tier — gated the same as Update.
	g.Post("/:id/post", middlewares.RequirePermission("invoice-journal-update"), ctrl.Post)
	g.Post("/:id/draft", middlewares.RequirePermission("invoice-journal-update"), ctrl.BackToDraft)
}
