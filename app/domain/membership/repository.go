package domain_membership

import "duluin_invoice/app/model"

// IMembershipRepository is the persistence port for user_account_sso. Every
// method is a bounded number of queries — no method fans out per row.
type IMembershipRepository interface {
	// FindByUserAndCompany returns the (soft-delete-aware) membership for the
	// pair, or (nil, nil) when there is none.
	FindByUserAndCompany(userID, companyID string) (*model.UserAccountSSO, error)

	// ListActiveByUser returns the user's activated, non-banned memberships with
	// the Company preloaded (1 IN-join, no N+1).
	ListActiveByUser(userID string) ([]model.UserAccountSSO, error)

	// DefaultCompanyID returns the company id of the user's oldest active
	// membership, or "" when they have none.
	DefaultCompanyID(userID string) (string, error)

	// ListByCompany is the paginated team list (1 COUNT + 1 SELECT).
	ListByCompany(f MemberFilter) ([]model.UserAccountSSO, int64, error)

	FindByIDInCompany(companyID, id string) (*model.UserAccountSSO, error)
	FindActiveMemberByEmail(companyID, email string) (*model.UserAccountSSO, error)
	FindPendingInviteByEmail(companyID, email string) (*model.UserAccountSSO, error)
	FindInviteByToken(token string) (*model.UserAccountSSO, error)

	// Upsert inserts or updates the membership keyed by (user_id, company_id)
	// when user_id is set, else by (company_id, lower(email)). It also
	// soft-deletes stale sibling rows for the same key (acc-master dedupe).
	Upsert(m *model.UserAccountSSO) error

	// Save persists an already-loaded row (full update).
	Save(m *model.UserAccountSSO) error

	SoftDeleteByID(id string) error

	CountActiveByRole(companyID, roleID string) (int64, error)
	// CountConsumedInviteSlots counts active + pending memberships that are not
	// the owner role — i.e. how many of the free invite quota are used.
	CountConsumedInviteSlots(companyID, ownerRoleID string) (int64, error)
}
