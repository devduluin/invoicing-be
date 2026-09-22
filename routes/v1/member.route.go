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
	inv := []string{"invoice-user-invite", "invoice-user-create"}
	g.Post("/validate", middlewares.RequirePermission(inv...), ctrl.Validate)
	g.Get("/companies", middlewares.RequirePermission("invoice-user-list"), ctrl.Companies)
	g.Post("/invite-multi", middlewares.RequirePermission(inv...), ctrl.InviteMulti)
	g.Get("/:id", middlewares.RequirePermission("invoice-user-list"), ctrl.Get)
	g.Patch("/:id/status", middlewares.RequirePermission("invoice-user-update"), ctrl.SetStatus)
	g.Post("/:id/resend", middlewares.RequirePermission(inv...), ctrl.Resend)
	g.Put("/:id/assignments", middlewares.RequirePermission("invoice-user-update"), ctrl.SyncAssignments)
	g.Patch("/:id", middlewares.RequirePermission("invoice-user-update"), ctrl.UpdateProfile)
	g.Patch("/:id/role", middlewares.RequirePermission("invoice-user-update"), ctrl.UpdateRole)
	g.Delete("/:id", middlewares.RequirePermission("invoice-user-delete"), ctrl.Remove)
}
