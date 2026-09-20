package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// UnitRoutes — configurable unit-of-measure master data.
func UnitRoutes(router fiber.Router, ctrl *controller.UnitController) {
	g := router.Group("/units")
	g.Get("/", middlewares.RequirePermission("invoice-unit-list"), ctrl.List)
	g.Get("/:id", middlewares.RequirePermission("invoice-unit-list"), ctrl.Get)
	g.Post("/", middlewares.RequirePermission("invoice-unit-create"), ctrl.Create)
	g.Put("/:id", middlewares.RequirePermission("invoice-unit-update"), ctrl.Update)
	g.Delete("/:id", middlewares.RequirePermission("invoice-unit-delete"), ctrl.Delete)
}
