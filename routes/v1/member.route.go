package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// MemberRoutes — team management for the active company. Permission slugs match
// the SSO DuluinInvoiceSeeder (invoice-user-*).
func MemberRoutes(router fiber.Router, ctrl *controller.MemberController) {
	g := router.Group("/members")
	g.Get("/", middlewares.RequirePermission("invoice-user-list"), ctrl.List)
	g.Post("/invite", middlewares.RequirePermission("invoice-user-invite", "invoice-user-create"), ctrl.Invite)
	g.Patch("/:id/role", middlewares.RequirePermission("invoice-user-update"), ctrl.UpdateRole)
	g.Delete("/:id", middlewares.RequirePermission("invoice-user-delete"), ctrl.Remove)
}
