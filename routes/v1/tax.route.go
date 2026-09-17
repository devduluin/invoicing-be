package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// TaxRoutes — configurable taxes master data (PRD §9).
func TaxRoutes(router fiber.Router, ctrl *controller.TaxController) {
	g := router.Group("/taxes")
	g.Get("/", middlewares.RequirePermission("invoice-tax-list"), ctrl.List)
	g.Get("/:id", middlewares.RequirePermission("invoice-tax-list"), ctrl.Get)
	g.Post("/", middlewares.RequirePermission("invoice-tax-create"), ctrl.Create)
	g.Put("/:id", middlewares.RequirePermission("invoice-tax-update"), ctrl.Update)
	g.Delete("/:id", middlewares.RequirePermission("invoice-tax-delete"), ctrl.Delete)
}
