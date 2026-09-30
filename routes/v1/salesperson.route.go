package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// SalespersonRoutes — sales person master data. Reading is also open to whoever creates or edits a
// sales order / invoice: they pick a salesperson there without needing the master's own permission.
func SalespersonRoutes(router fiber.Router, ctrl *controller.SalespersonController) {
	read := middlewares.RequirePermission(
		"invoice-salesperson-list",
		"invoice-sales-order-create", "invoice-sales-order-update",
		"invoice-sales-invoice-create", "invoice-sales-invoice-update",
	)
	g := router.Group("/salespersons")
	g.Get("/", read, ctrl.List)
	g.Get("/mine", read, ctrl.Mine)
	g.Get("/next-code", middlewares.RequirePermission("invoice-salesperson-create"), ctrl.NextCode)
	g.Get("/team-members", middlewares.RequirePermission("invoice-salesperson-create", "invoice-salesperson-update"), ctrl.TeamMembers)
	g.Post("/from-members", middlewares.RequirePermission("invoice-salesperson-create"), ctrl.FromMembers)
	g.Post("/bulk-activate", middlewares.RequirePermission("invoice-salesperson-update"), ctrl.BulkActivate)
	g.Post("/bulk-deactivate", middlewares.RequirePermission("invoice-salesperson-update"), ctrl.BulkDeactivate)
	g.Post("/bulk-delete", middlewares.RequirePermission("invoice-salesperson-delete"), ctrl.BulkDelete)
	g.Get("/:id", read, ctrl.Get)
	g.Post("/", middlewares.RequirePermission("invoice-salesperson-create"), ctrl.Create)
	g.Put("/:id", middlewares.RequirePermission("invoice-salesperson-update"), ctrl.Update)
	g.Delete("/:id", middlewares.RequirePermission("invoice-salesperson-delete"), ctrl.Delete)
}
