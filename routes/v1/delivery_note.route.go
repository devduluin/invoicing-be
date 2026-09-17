package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// DeliveryNoteRoutes — "Surat Jalan". No PUT/DELETE: the seeded permissions
// are invoice-delivery-note-{list,create} only.
func DeliveryNoteRoutes(router fiber.Router, ctrl *controller.DeliveryNoteController) {
	g := router.Group("/delivery-notes")
	g.Get("/", middlewares.RequirePermission("invoice-delivery-note-list"), ctrl.List)
	g.Get("/:id", middlewares.RequirePermission("invoice-delivery-note-list"), ctrl.Get)
	g.Post("/", middlewares.RequirePermission("invoice-delivery-note-create"), ctrl.Create)
}
