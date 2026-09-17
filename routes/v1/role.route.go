package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// RoleReadRoutes — custom-role catalog reads. Available during onboarding (the
// Step 4 invite dropdown needs it) so it hangs off the rbac router, not the
// completed-onboarding router.
func RoleReadRoutes(router fiber.Router, ctrl *controller.RoleController) {
	g := router.Group("/roles", middlewares.RequirePermission("invoice-role-list", "invoice-user-list"))
	g.Get("/", ctrl.List)
	g.Get("/:id", ctrl.Get)
}

// RoleWriteRoutes — custom-role CRUD (deferred editor UI; API is live).
func RoleWriteRoutes(router fiber.Router, ctrl *controller.RoleController) {
	g := router.Group("/roles")
	g.Post("/", middlewares.RequirePermission("invoice-create-role"), ctrl.Create)
	g.Put("/:id", middlewares.RequirePermission("invoice-update-role"), ctrl.Update)
	g.Delete("/:id", middlewares.RequirePermission("invoice-delete-role"), ctrl.Delete)
}
