package controller

import (
	"github.com/gofiber/fiber/v2"

	membership "duluin_invoice/app/domain/membership"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

// MyCompaniesLister is the membership-service slice MeController needs.
type MyCompaniesLister interface {
	ListMyCompanies(a membership.Actor) ([]membership.CompanyMembership, error)
}

type MeController struct {
	companies MyCompaniesLister
}

func NewMeController(companies MyCompaniesLister) *MeController {
	return &MeController{companies: companies}
}

// Get returns the authenticated identity plus the user's companies and the
// active company's onboarding state — the frontend uses onboarding_status to
// route to /onboarding vs /dashboard, and companies[] to build the switcher.
func (ctrl *MeController) Get(c *fiber.Ctx) error {
	isActivated, _ := c.Locals("isActivated").(bool)

	actor := membership.Actor{
		UserID:          middlewares.GetUserID(c),
		Email:           middlewares.GetEmail(c),
		Name:            middlewares.GetName(c),
		ActiveCompanyID: middlewares.GetCompanyID(c),
		Token:           c.Get("Authorization"),
	}

	companies, err := ctrl.companies.ListMyCompanies(actor)
	if err != nil {
		companies = []membership.CompanyMembership{}
	}

	var activeCompany any
	onboardingStatus := "not_started"
	activeID := middlewares.GetCompanyID(c)
	if company := middlewares.GetCompany(c); company != nil {
		onboardingStatus = string(company.OnboardingStatus)
	}
	for i := range companies {
		if companies[i].Company != nil && companies[i].Company.ID == activeID {
			activeCompany = companies[i]
			if companies[i].Company.OnboardingStatus != "" {
				onboardingStatus = string(companies[i].Company.OnboardingStatus)
			}
			break
		}
	}

	return utils.Ok(c, fiber.Map{
		"user_id":           middlewares.GetUserID(c),
		"company_id":        activeID,
		"active_company":    activeCompany,
		"companies":         companies,
		"onboarding_status": onboardingStatus,
		"name":              middlewares.GetName(c),
		"email":             middlewares.GetEmail(c),
		"roles":             middlewares.GetRoles(c),
		"permissions":       middlewares.GetPermissions(c),
		"is_activated":      isActivated,
		"account_type":      c.Locals("accountType"),
	}, "OK")
}
