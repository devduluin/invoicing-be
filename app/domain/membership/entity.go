// Package domain_membership holds the pure types + ports for company membership
// and per-company RBAC (acc-master pattern). No HTTP, no DB, no SSO SDK here.
package domain_membership

import "duluin_invoice/app/model"

// Actor is the authenticated caller of a membership action.
type Actor struct {
	UserID          string
	Email           string
	Name            string
	ActiveCompanyID string
	Token           string // raw Authorization header, forwarded to SSO for role reads
}

// AccessResolution is the outcome of resolving one (user, company) pair to its
// effective access. Mirrors acc-master-service AccessResolution.
type AccessResolution struct {
	HasAccess       bool     `json:"has_access"`
	IsActivated     bool     `json:"is_activated"`
	IsBanned        bool     `json:"is_banned"`
	BannedReason    string   `json:"banned_reason,omitempty"`
	RoleID          string   `json:"role_id,omitempty"`
	RoleName        string   `json:"role_name,omitempty"`
	Permissions     []string `json:"permissions,omitempty"`
	Roles           []string `json:"roles,omitempty"`
	UsedSSOFallback bool     `json:"used_sso_fallback,omitempty"`

	// ResolutionStatus is "" (unknown), "resolved" or "unresolved". An
	// unresolved access means the caller could not prove the user's rights and
	// the middleware must fail closed (503), never fall through as "no access".
	ResolutionStatus string `json:"resolution_status,omitempty"`
	ResolutionReason string `json:"resolution_reason,omitempty"`
}

const (
	resolutionResolved   = "resolved"
	resolutionUnresolved = "unresolved"
)

func (a *AccessResolution) MarkResolved() {
	a.ResolutionStatus = resolutionResolved
	a.ResolutionReason = ""
}

func (a *AccessResolution) MarkUnresolved(reason string) {
	a.ResolutionStatus = resolutionUnresolved
	a.ResolutionReason = reason
}

func (a *AccessResolution) IsUnresolved() bool { return a.ResolutionStatus == resolutionUnresolved }

// InviteInput is one Step-4 / team-page invitation. Role is an SSO role id
// (global or company-scoped custom), never a fixed enum.
type InviteInput struct {
	Email  string
	Name   string
	RoleID string
}

// CompanyMembership is one row of "which companies can this user act in", used
// by GET /me and the company switcher.
type CompanyMembership struct {
	Company     *model.Company `json:"company"`
	RoleID      string         `json:"role_id,omitempty"`
	RoleName    string         `json:"role,omitempty"`
	IsActivated bool           `json:"is_activated"`
	IsOwner     bool           `json:"is_owner"`
}

// MemberView is one row of the team page.
type MemberView struct {
	ID          string  `json:"id"`
	UserID      string  `json:"user_id,omitempty"`
	Email       string  `json:"email"`
	Name        string  `json:"name,omitempty"`
	RoleID      string  `json:"role_id,omitempty"`
	RoleName    string  `json:"role,omitempty"`
	IsActivated bool    `json:"is_activated"`
	IsBanned    bool    `json:"is_banned"`
	Pending     bool    `json:"pending"`
	InvitedAt   *string `json:"invited_at,omitempty"`
	Status      string  `json:"status,omitempty"`
	Phone       string  `json:"phone,omitempty"`
	CreatedAt   string  `json:"created_at,omitempty"`
	// InviteURL is the link SSO put in the invitation email. Only filled outside production.
	InviteURL string `json:"invite_url,omitempty"`
	// Companies the caller can see this person in (each with its own role).
	Companies []MemberCompany `json:"companies,omitempty"`
}

// MemberFilter drives the paginated team list.
type MemberFilter struct {
	CompanyID string
	Search    string
	Page      int
	PageSize  int
	Sort      string
	Order     string
	Fields    []string
}

// MemberListColumns — fields the team MasterTable may show / sort by.
var MemberListColumns = []string{
	"name", "email", "phone", "role", "status", "companies", "created_at",
}

// Member statuses shown in User Management.
const (
	StatusActive   = "active"
	StatusPending  = "pending"
	StatusInactive = "inactive"
)

// MemberCompany is one company a person has (or may be given) access to.
type MemberCompany struct {
	MemberID    string `json:"member_id,omitempty"`
	CompanyID   string `json:"company_id"`
	CompanyName string `json:"company_name"`
	CompanyCode string `json:"company_code,omitempty"`
	RoleID      string `json:"role_id,omitempty"`
	RoleName    string `json:"role,omitempty"`
	Status      string `json:"status"`
}

// MemberDetail is the full picture of one person across the companies the
// caller may see.
type MemberDetail struct {
	MemberView
	AcceptedAt *string         `json:"accepted_at,omitempty"`
	Companies  []MemberCompany `json:"companies"`
}

// CompanyAssignment is one (company, role) grant.
type CompanyAssignment struct {
	CompanyID string `json:"company_id"`
	RoleID    string `json:"role_id"`
}

// InviteMultiInput invites one person to several companies at once.
type InviteMultiInput struct {
	Email     string
	Name      string
	Phone     string
	Active    bool
	SendEmail bool
	Grants    []CompanyAssignment
}

// ValidateResult tells the invite form what is already known about an email.
type ValidateResult struct {
	Status           string   `json:"status"` // new_user | existing_user | already_member
	ExistsInSSO      bool     `json:"exists_in_sso"`
	HasPendingInvite bool     `json:"has_pending_invite"`
	CanInvite        bool     `json:"can_invite"`
	UserName         string   `json:"user_name,omitempty"`
	UserPhone        string   `json:"user_phone,omitempty"`
	MemberCompanyIDs []string `json:"member_company_ids"`
}

// ManageableCompany is a company the caller may grant access to.
type ManageableCompany struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Code string `json:"code,omitempty"`
}
