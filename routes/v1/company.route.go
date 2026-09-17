package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// CompanyRoutes — company list, active-company detail, and the shareable-ID
// directory lookup (network invoicing). Membership-exempt so a user with zero
// companies can still see the (empty) list. A new company is created via
// POST /onboarding, not here.
func CompanyRoutes(router fiber.Router, ctrl *controller.CompanyController) {
	router.Get("/companies", ctrl.List)
	router.Get("/companies/me", ctrl.Me)
	router.Get("/companies/lookup/:code", ctrl.Lookup)
}

// CompanySettingsRoutes — company profile + logo edit. Owner-only (invoice-settings).
func CompanySettingsRoutes(router fiber.Router, ctrl *controller.CompanyController) {
	router.Put("/companies/me", middlewares.RequirePermission("invoice-settings"), ctrl.UpdateMe)
}
