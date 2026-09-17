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
	"name", "email", "role", "is_activated", "is_banned", "pending", "invited_at",
}
