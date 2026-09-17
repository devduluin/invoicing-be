package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/middlewares"
)

// baseRouter — SSO token validation + identity injection + active-company
// context. Onboarding, /me and /companies hang off this directly (no membership
// gate); rbacRouter stacks the RBAC middleware on top.
func baseRouter(
	router fiber.Router,
	loadCompany middlewares.CompanyLoaderFunc,
	defaultCompany middlewares.DefaultCompanyFunc,
) fiber.Router {
	return router.Group("",
		middlewares.ValidateToken(),
		middlewares.Authenticate,
		middlewares.LoadCompanyContext(loadCompany, defaultCompany),
	)
}

// rbacRouter — requires an activated, non-banned membership in the active
// company and loads its company-scoped permissions.
func rbacRouter(base fiber.Router, resolver middlewares.AccessResolver) fiber.Router {
	return base.Group("",
		middlewares.LoadLocalPermissions(resolver),
		middlewares.RequireCompanyMembership(),
	)
}
