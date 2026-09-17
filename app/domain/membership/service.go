package domain_membership

import (
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// IMembershipService is the application port used by the HTTP layer and by the
// onboarding service.
type IMembershipService interface {
	// ResolveAccess resolves (user, company) → effective access, using Redis
	// cache and SSO role→permission lookup. ssoRoles/ssoPerms are the coarse
	// signin-cookies fallback (used only when the local role_id is unset or SSO
	// is temporarily unreachable and the migration fallback is on).
	ResolveAccess(a Actor, companyID string, ssoRoles, ssoPerms []string) (*AccessResolution, error)

	// LinkCompanyCreator makes the actor the owner of a freshly created company:
	// resolves the "Invoice Owner" role id, upserts an activated membership and
	// best-effort syncs SSO (secondary_id = user id; coarse owner role).
	LinkCompanyCreator(a Actor, companyID string) error

	Invite(a Actor, companyID string, in InviteInput) (*model.UserAccountSSO, error)
	AcceptInvite(userID, name, token string) (*model.UserAccountSSO, error)

	ListMembers(a Actor, f MemberFilter) (*utils.OffsetPaginationResult, error)
	UpdateMemberRole(a Actor, companyID, targetMemberID, roleID string) (*MemberView, error)
	RemoveMember(a Actor, companyID, targetMemberID string) error

	ListMyCompanies(a Actor) ([]CompanyMembership, error)
}

// IRoleService is the custom-role catalog/CRUD port (proxies to SSO).
type IRoleService interface {
	List(a Actor, companyID string) ([]RoleInfo, error)
	Get(a Actor, companyID, roleID string) (*RoleDetail, error)
	Create(a Actor, companyID, name string, permissions []string) (*RoleInfo, error)
	Update(a Actor, companyID, roleID, name string, permissions []string) (*RoleInfo, error)
	Delete(a Actor, companyID, roleID string) error
}

// RoleInfo / RolePerms / RoleDetail are the SSO-role projections the service
// layer works with — a copy of the SSO SDK shapes so this package stays free of
// the HTTP client.
type RoleInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	CompanyID   *string  `json:"company_id,omitempty"`
	IsCustom    bool     `json:"is_custom"`
	Permissions []string `json:"permissions,omitempty"`
}

type RolePerms struct {
	RoleID      string
	RoleName    string
	Permissions []string
}

type RoleDetail struct {
	Role           RoleInfo `json:"role"`
	AllPermissions []string `json:"all_permissions"`
}

// CompanyReader is the slice of company persistence the membership service needs
// (onboarding owns the writes).
type CompanyReader interface {
	FindByID(id string) (*model.Company, error)
}
