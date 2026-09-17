package middlewares

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// GetRequestID returns the request id from header or fiber locals.
func GetRequestID(c *fiber.Ctx) string {
	if c == nil {
		return ""
	}
	if id := strings.TrimSpace(c.Get("X-Request-ID")); id != "" {
		return id
	}
	if id, ok := c.Locals("requestid").(string); ok {
		return strings.TrimSpace(id)
	}
	return ""
}

// ResolveActiveCompanyID picks the tenant context from the gateway headers or the
// SSO cookies set by Launchpad (company_id / app_company_id).
func ResolveActiveCompanyID(c *fiber.Ctx) string {
	if id := strings.TrimSpace(c.Get("X-Company-ID")); id != "" {
		return id
	}
	if id := strings.TrimSpace(c.Get("x-callback-token")); id != "" {
		return id
	}
	if id := strings.TrimSpace(c.Cookies("app_company_id")); id != "" {
		return id
	}
	if id := strings.TrimSpace(c.Cookies("company_id")); id != "" {
		return id
	}
	return strings.TrimSpace(c.Cookies("app_root_company_id"))
}

// ResolveSSOUserID returns the SSO user id from the sso_user_id cookie.
func ResolveSSOUserID(c *fiber.Ctx) string {
	return strings.TrimSpace(c.Cookies("sso_user_id"))
}
