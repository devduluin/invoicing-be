package middlewares

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	membership "duluin_invoice/app/domain/membership"
	"duluin_invoice/app/model"
	"duluin_invoice/config"
	"duluin_invoice/utils"
)

// CompanyLoaderFunc resolves a local company by id. Injected so this package
// never touches the DB. Returns (nil, nil) when the company does not exist.
type CompanyLoaderFunc func(id string) (*model.Company, error)

// DefaultCompanyFunc returns the user's default company id (oldest active
// membership) — used only when the request carries no explicit company header.
type DefaultCompanyFunc func(userID string) (string, error)

// AccessResolver is the membership-service slice the RBAC middleware needs.
type AccessResolver interface {
	ResolveAccess(a membership.Actor, companyID string, ssoRoles, ssoPerms []string) (*membership.AccessResolution, error)
}

// LoadCompanyContext resolves the active company id from the request and, when
// present, loads the company row into locals. Company context comes from the
// X-Company-ID / x-callback-token header (SSO secondary_id is the user id, not a
// company); when absent it falls back to the user's default company. Never fails
// the request — onboarding runs before a company exists.
func LoadCompanyContext(load CompanyLoaderFunc, defaultCompany DefaultCompanyFunc) fiber.Handler {
	return func(c *fiber.Ctx) error {
		companyID := ResolveActiveCompanyID(c)
		if companyID == "" && defaultCompany != nil {
			if uid := strings.TrimSpace(GetUserID(c)); uid != "" {
				if def, err := defaultCompany(uid); err == nil {
					companyID = def
				}
			}
		}
		if companyID == "" {
			return c.Next()
		}
		c.Locals("companyID", companyID)

		company, err := load(companyID)
		if err != nil {
			return utils.InternalError(c, err)
		}
		if company != nil {
			c.Locals("company", company)
			c.Locals("onboardingStatus", string(company.OnboardingStatus))
		}
		return c.Next()
	}
}

// LoadLocalPermissions resolves per-company access (role_id → permissions via
// SSO + Redis) and overwrites the coarse signin-cookies roles/permissions with
// the company-scoped set. Fails closed (503) when access cannot be resolved.
func LoadLocalPermissions(resolver AccessResolver) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !config.AppConfig.UseLocalRBAC {
			return c.Next()
		}
		userID := strings.TrimSpace(GetUserID(c))
		companyID := strings.TrimSpace(GetCompanyID(c))
		if userID == "" || companyID == "" {
			return c.Next()
		}

		actor := membership.Actor{
			UserID:          userID,
			Email:           GetEmail(c),
			Name:            GetName(c),
			ActiveCompanyID: companyID,
			Token:           strings.Clone(c.Get("Authorization")),
		}
		access, err := resolver.ResolveAccess(actor, companyID, GetRoles(c), GetPermissions(c))
		if err != nil || access == nil {
			return serviceUnavailable(c, "akses tidak dapat diverifikasi")
		}
		if access.IsUnresolved() {
			return serviceUnavailable(c, "akses tidak dapat diverifikasi")
		}

		c.Locals("localAccess", access)
		if len(access.Permissions) > 0 {
			c.Locals("permissions", access.Permissions)
		}
		if len(access.Roles) > 0 {
			c.Locals("roles", access.Roles)
		}
		return c.Next()
	}
}

// RequireCompanyMembership blocks the request unless the user has an activated,
// non-banned membership in the active company.
func RequireCompanyMembership() fiber.Handler {
	return func(c *fiber.Ctx) error {
		access := GetLocalAccess(c)
		if access == nil {
			// RBAC disabled or no company context — nothing to gate here.
			return c.Next()
		}
		switch {
		case !access.HasAccess:
			return forbiddenCode(c, "Tidak punya akses ke perusahaan ini", "no_company_access")
		case access.IsBanned:
			return forbiddenCode(c, "Keanggotaan diblokir", "membership_banned")
		case !access.IsActivated:
			return forbiddenCode(c, "Keanggotaan belum diaktifkan", "membership_not_activated")
		}
		return c.Next()
	}
}

// RequireCompletedOnboarding blocks transactional features until the PRD §4
// wizard is finished (companies.onboarding_status == "active").
func RequireCompletedOnboarding() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if status, _ := c.Locals("onboardingStatus").(string); status == string(model.OnboardingActive) {
			return c.Next()
		}
		return forbiddenCode(c, "Selesaikan onboarding terlebih dahulu", "onboarding_incomplete")
	}
}

// ── locals accessors ───────────────────────────────────────────────────────

func GetCompany(c *fiber.Ctx) *model.Company {
	company, _ := c.Locals("company").(*model.Company)
	return company
}

func GetLocalAccess(c *fiber.Ctx) *membership.AccessResolution {
	access, _ := c.Locals("localAccess").(*membership.AccessResolution)
	return access
}

func forbiddenCode(c *fiber.Ctx, message, code string) error {
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		"success": false, "message": message, "error_code": code,
	})
}

func serviceUnavailable(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
		"success": false, "message": message, "error_code": "rbac_unresolved",
	})
}
