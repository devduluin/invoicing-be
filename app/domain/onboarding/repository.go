package domain_onboarding

import (
	membership "duluin_invoice/app/domain/membership"
	"duluin_invoice/app/model"
)

// IOnboardingRepository is the company persistence port for onboarding. Team
// membership is owned by the membership repository (MembershipPort).
type IOnboardingRepository interface {
	FindCompanyByID(id string) (*model.Company, error)
	CreateCompany(c *model.Company) error
	// HardDeleteCompany removes a company row for good — the compensating action
	// when owner-linking fails right after a create.
	HardDeleteCompany(id string) error
	CompanyCodeExists(code string) (bool, error)
}

// MembershipPort is the slice of the membership service onboarding needs: make
// the creator an owner, and send the Step-4 invitations.
type MembershipPort interface {
	LinkCompanyCreator(a membership.Actor, companyID string) error
	Invite(a membership.Actor, companyID string, in membership.InviteInput) (*model.UserAccountSSO, error)
}

// MasterDataSeeder seeds the template COA + taxes for a fresh company (PRD §7/§9).
// Best-effort — a failure never rolls the company back.
type MasterDataSeeder interface {
	SeedCompanyDefaults(companyID, actorID string) error
}

// IOnboardingService is the application port used by the HTTP layer.
type IOnboardingService interface {
	// Submit is the ONE commit point: it creates the company, makes the caller
	// its owner and sends the invitations, all from a single payload. Nothing is
	// persisted before this call, so an abandoned wizard leaves no trace.
	Submit(actor Actor, dto SubmitDTO) (*SubmitResult, error)
}
