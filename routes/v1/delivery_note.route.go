package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// DeliveryNoteRoutes — "Surat Jalan". Update replaces the record; Delete is a soft delete.
func DeliveryNoteRoutes(router fiber.Router, ctrl *controller.DeliveryNoteController) {
	g := router.Group("/delivery-notes")
	g.Get("/", middlewares.RequirePermission("invoice-delivery-note-list"), ctrl.List)
	g.Get("/:id", middlewares.RequirePermission("invoice-delivery-note-list"), ctrl.Get)
	g.Post("/", middlewares.RequirePermission("invoice-delivery-note-create"), ctrl.Create)
	g.Put("/:id", middlewares.RequirePermission("invoice-delivery-note-update"), ctrl.Update)
	g.Delete("/:id", middlewares.RequirePermission("invoice-delivery-note-delete"), ctrl.Delete)
}
