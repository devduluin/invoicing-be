package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// AccountRoutes — Chart of Accounts master data (PRD §7).
func AccountRoutes(router fiber.Router, ctrl *controller.AccountController) {
	g := router.Group("/accounts")
	g.Get("/", middlewares.RequirePermission("invoice-coa-list"), ctrl.List)
	g.Get("/:id", middlewares.RequirePermission("invoice-coa-list"), ctrl.Get)
	g.Post("/", middlewares.RequirePermission("invoice-coa-create"), ctrl.Create)
	g.Put("/:id", middlewares.RequirePermission("invoice-coa-update"), ctrl.Update)
	g.Delete("/:id", middlewares.RequirePermission("invoice-coa-delete"), ctrl.Delete)
}
