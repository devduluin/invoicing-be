package middlewares

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"duluin_invoice/utils"
)

// Fixed roles for Duluin Invoice Phase 1 (PRD §13). These names match the SSO
// DuluinInvoiceSeeder role names.
const (
	RoleOwner  = "Invoice Owner"
	RoleAdmin  = "Invoice Admin"
	RoleViewer = "Invoice Viewer"
)

// Authenticate re-injects identity from gateway headers when present, so the
// service works both behind the API gateway and with a direct SSO-cookie call.
func Authenticate(c *fiber.Ctx) error {
	if id := strings.TrimSpace(c.Get("X-User-ID")); id != "" {
		c.Locals("userID", id)
	}
	if id := strings.TrimSpace(c.Get("X-Company-ID")); id != "" {
		c.Locals("companyID", id)
	}
	if roles := c.Get("X-Roles"); roles != "" {
		c.Locals("roles", utils.ParseCommaSeparated(roles))
	}
	if perms := c.Get("X-Permissions"); perms != "" {
		c.Locals("permissions", utils.ParseCommaSeparated(perms))
	}
	if email := strings.TrimSpace(c.Get("X-Email")); email != "" {
		c.Locals("email", email)
	}
	if name := strings.TrimSpace(c.Get("X-Name")); name != "" {
		c.Locals("name", name)
	}
	return c.Next()
}

// RequireRole allows the request only if the user holds one of the given fixed
// roles. Company-scoped SSO labels ("{companyId}-Invoice Admin") also match.
func RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRoles := GetRoles(c)
		if len(userRoles) == 0 {
			return utils.Forbidden(c, []string{"No role assigned to user"})
		}
		companyID := strings.TrimSpace(GetCompanyID(c))
		for _, required := range roles {
			for _, ur := range userRoles {
				if roleMatches(ur, required, companyID) {
					return c.Next()
				}
			}
		}
		return utils.Forbidden(c, []string{"You don't have permission to access this resource"})
	}
}

// RequirePermission allows the request only if the user holds one of the given
// permission slugs (as seeded in SSO, e.g. "invoice-mitra-create").
func RequirePermission(permissions ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if HasAnyPermission(c, permissions...) {
			return c.Next()
		}
		return utils.Forbidden(c, []string{"You don't have permission to access this resource"})
	}
}

func roleMatches(userRole, required, companyID string) bool {
	if userRole == required {
		return true
	}
	if required == "" {
		return false
	}
	if !strings.HasSuffix(userRole, "-"+required) {
		return false
	}
	if companyID == "" {
		return true
	}
	return strings.HasPrefix(userRole, companyID+"-")
}

// ── locals accessors ────────────────────────────────────────────────────────

func GetCompanyID(c *fiber.Ctx) string {
	id, _ := c.Locals("companyID").(string)
	if strings.TrimSpace(id) != "" {
		return id
	}
	return ResolveActiveCompanyID(c)
}

func GetUserID(c *fiber.Ctx) string {
	if id, ok := c.Locals("userID").(string); ok && strings.TrimSpace(id) != "" {
		return id
	}
	return ResolveSSOUserID(c)
}

func GetName(c *fiber.Ctx) string {
	name, _ := c.Locals("name").(string)
	return name
}

func GetEmail(c *fiber.Ctx) string {
	email, _ := c.Locals("email").(string)
	return email
}

func GetRoles(c *fiber.Ctx) []string {
	if access := GetLocalAccess(c); access != nil && len(access.Roles) > 0 {
		return access.Roles
	}
	roles, _ := c.Locals("roles").([]string)
	return roles
}

func GetPermissions(c *fiber.Ctx) []string {
	if access := GetLocalAccess(c); access != nil && len(access.Permissions) > 0 {
		return access.Permissions
	}
	permissions, _ := c.Locals("permissions").([]string)
	return permissions
}

func HasAnyPermission(c *fiber.Ctx, permissions ...string) bool {
	assigned := GetPermissions(c)
	for _, want := range permissions {
		if utils.Contains(assigned, want) {
			return true
		}
	}
	return false
}
