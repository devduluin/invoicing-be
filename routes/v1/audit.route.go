package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// AuditRoutes — read-only audit trail (append-only; there is no write route here on purpose).
// invoice-list-audit-log is an existing SSO permission (DuluinInvoiceSeeder), already granted to
// Invoice Owner and Invoice Admin, not Viewer.
func AuditRoutes(router fiber.Router, ctrl *controller.AuditController) {
	g := router.Group("/audit-log")
	g.Get("/", middlewares.RequirePermission("invoice-list-audit-log"), ctrl.List)
	g.Get("/:id", middlewares.RequirePermission("invoice-list-audit-log"), ctrl.Get)
}

// AuditSessionRoute — POST /audit-log/session, recording the caller's own login/logout. Mounted
// separately on the base router (auth + company context only, no RBAC/permission or completed-
// onboarding requirement) since the frontend calls it from /auth/logout too, at a point where the
// stricter checks other routes need would only get in the way of logging the logout itself.
func AuditSessionRoute(router fiber.Router, ctrl *controller.AuditController) {
	router.Post("/audit-log/session", ctrl.LogSession)
}
