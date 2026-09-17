package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// MitraRoutes mounts Mitra CRUD (PRD §8). Permission slugs match the SSO
// DuluinInvoiceSeeder (invoice-mitra-*). Viewer can only list/read; Admin & Owner
// can write; delete stays Owner-only.
func MitraRoutes(router fiber.Router, ctrl *controller.MitraController) {
	router.Get("/", middlewares.RequirePermission("invoice-mitra-list"), ctrl.List)
	router.Get("/:id", middlewares.RequirePermission("invoice-mitra-list"), ctrl.Get)
	router.Post("/", middlewares.RequirePermission("invoice-mitra-create"), ctrl.Create)
	router.Put("/:id", middlewares.RequirePermission("invoice-mitra-update"), ctrl.Update)
	router.Delete("/:id", middlewares.RequirePermission("invoice-mitra-delete"), ctrl.Delete)
}
