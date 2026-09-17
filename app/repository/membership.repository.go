package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/membership"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// membershipColumns — explicit projection (no SELECT *).
var membershipColumns = []string{
	"id", "user_id", "company_id", "secondary_id", "role_id",
	"is_activated", "is_banned", "banned_reason", "activated_at",
	"email", "name", "invite_token", "invited_by", "invited_at",
	"created_at", "updated_at",
}

type MembershipRepository struct {
	db *gorm.DB
}

func NewMembershipRepository(db *gorm.DB) domain.IMembershipRepository {
	return &MembershipRepository{db: db}
}

func (r *MembershipRepository) FindByUserAndCompany(userID, companyID string) (*model.UserAccountSSO, error) {
	userID = strings.TrimSpace(userID)
	companyID = strings.TrimSpace(companyID)
	if userID == "" || companyID == "" {
		return nil, nil
	}
	return r.first("user_id = ? AND company_id = ?", []any{userID, companyID})
}

func (r *MembershipRepository) FindByIDInCompany(companyID, id string) (*model.UserAccountSSO, error) {
	return r.first("id = ? AND company_id = ?", []any{strings.TrimSpace(id), strings.TrimSpace(companyID)})
}

func (r *MembershipRepository) FindActiveMemberByEmail(companyID, email string) (*model.UserAccountSSO, error) {
	return r.first(
		"company_id = ? AND lower(email) = lower(?) AND is_activated = true",
		[]any{strings.TrimSpace(companyID), strings.TrimSpace(email)},
	)
}

func (r *MembershipRepository) FindPendingInviteByEmail(companyID, email string) (*model.UserAccountSSO, error) {
	return r.first(
		"company_id = ? AND lower(email) = lower(?) AND is_activated = false AND invited_at IS NOT NULL",
		[]any{strings.TrimSpace(companyID), strings.TrimSpace(email)},
	)
}

func (r *MembershipRepository) FindInviteByToken(token string) (*model.UserAccountSSO, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, nil
	}
	return r.first("invite_token = ?", []any{token})
}

func (r *MembershipRepository) first(cond string, args []any) (*model.UserAccountSSO, error) {
	var m model.UserAccountSSO
	err := r.db.Select(membershipColumns).Where(cond, args...).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find membership: %w", err)
	}
	return &m, nil
}

// ListActiveByUser — the user's usable companies: an activated, non-banned
// membership in a company that has finished onboarding. A company stuck at
// onboarding_status='pending' is a half-born row (onboarding is single-commit)
// and must not surface in the switcher or drive routing.
func (r *MembershipRepository) ListActiveByUser(userID string) ([]model.UserAccountSSO, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, nil
	}
	onboarded := r.db.Model(&model.Company{}).
		Select("id").Where("onboarding_status = ?", model.OnboardingActive)

	var rows []model.UserAccountSSO
	err := r.db.
		Select(membershipColumns).
		Preload("Company").
		Where("user_id = ? AND is_activated = true AND is_banned = false", userID).
		Where("company_id IN (?)", onboarded).
		Order("created_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list memberships for user %s: %w", userID, err)
	}
	return rows, nil
}

// DefaultCompanyID — the company to use when the request carries no explicit
// company header: the user's oldest activated membership in an onboarded
// company. Falls back to the oldest membership of any kind so a user who only
// has a pending company still resolves somewhere (they'll be routed to finish).
func (r *MembershipRepository) DefaultCompanyID(userID string) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", nil
	}
	base := func() *gorm.DB {
		return r.db.Model(&model.UserAccountSSO{}).
			Where("user_id = ? AND is_activated = true AND is_banned = false", userID).
			Order("created_at ASC").Limit(1)
	}

	// Prefer the oldest membership in an onboarded company.
	onboarded := r.db.Model(&model.Company{}).
		Select("id").Where("onboarding_status = ?", model.OnboardingActive)
	var companyID string
	if err := base().Where("company_id IN (?)", onboarded).
		Pluck("company_id", &companyID).Error; err != nil {
		return "", fmt.Errorf("default company for user %s: %w", userID, err)
	}
	if companyID != "" {
		return companyID, nil
	}

	// None yet — fall back to the oldest membership of any kind so the request
	// still resolves (the user will be routed to finish onboarding).
	if err := base().Pluck("company_id", &companyID).Error; err != nil {
		return "", fmt.Errorf("default company (fallback) for user %s: %w", userID, err)
	}
	return companyID, nil
}

// ListByCompany — 1 COUNT + 1 paginated SELECT.
func (r *MembershipRepository) ListByCompany(f domain.MemberFilter) ([]model.UserAccountSSO, int64, error) {
	q := r.db.Model(&model.UserAccountSSO{}).Where("company_id = ?", strings.TrimSpace(f.CompanyID))
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where("lower(email) LIKE ? OR lower(name) LIKE ?", like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count members: %w", err)
	}

	page, size := normalisePage(f.Page, f.PageSize)
	sortCol := utils.NormalizeSort(f.Sort, memberSortColumns, "")
	order := strings.ToUpper(strings.TrimSpace(f.Order))
	if order != "ASC" && order != "DESC" {
		order = "ASC"
	}
	orderClause := "is_activated DESC, created_at ASC"
	if sortCol != "" {
		orderClause = sortCol + " " + order
	}

	var rows []model.UserAccountSSO
	err := q.Select(membershipColumns).
		Order(orderClause).
		Offset((page - 1) * size).
		Limit(size).
		Find(&rows).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list members: %w", err)
	}
	return rows, total, nil
}

// memberSortColumns — DB columns the team list may be sorted by (the "role"
// column shown in the UI is enriched from SSO, not sortable here).
var memberSortColumns = []string{"name", "email", "is_activated", "is_banned", "invited_at", "created_at", "updated_at"}

// Upsert inserts or updates keyed by (user_id, company_id) when user_id is set,
// else by (company_id, lower(email)). Stale sibling rows for the key are
// soft-deleted so a user never ends up with two live rows for one company.
func (r *MembershipRepository) Upsert(m *model.UserAccountSSO) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing model.UserAccountSSO
		var lookup *gorm.DB
		byUser := strings.TrimSpace(m.UserID) != ""
		if byUser {
			lookup = tx.Where("user_id = ? AND company_id = ?", m.UserID, m.CompanyID)
		} else {
			lookup = tx.Where("company_id = ? AND lower(email) = lower(?) AND user_id = ''", m.CompanyID, m.Email)
		}

		err := lookup.Order("created_at ASC").First(&existing).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := tx.Create(m).Error; err != nil {
				return fmt.Errorf("insert membership: %w", err)
			}
		case err != nil:
			return fmt.Errorf("upsert lookup: %w", err)
		default:
			m.ID = existing.ID
			m.CreatedAt = existing.CreatedAt
			if err := tx.Model(&model.UserAccountSSO{}).
				Where("id = ?", existing.ID).
				Updates(upsertPatch(m)).Error; err != nil {
				return fmt.Errorf("update membership %s: %w", existing.ID, err)
			}
			// Dedupe: soft-delete any other live rows for the same key.
			dedupe := tx.Model(&model.UserAccountSSO{}).Where("id <> ?", existing.ID)
			if byUser {
				dedupe = dedupe.Where("user_id = ? AND company_id = ?", m.UserID, m.CompanyID)
			} else {
				dedupe = dedupe.Where("company_id = ? AND lower(email) = lower(?) AND user_id = ''", m.CompanyID, m.Email)
			}
			if err := dedupe.Update("deleted_at", time.Now()).Error; err != nil {
				return fmt.Errorf("dedupe memberships: %w", err)
			}
		}
		return nil
	})
}

func upsertPatch(m *model.UserAccountSSO) map[string]any {
	patch := map[string]any{
		"updated_at":   time.Now(),
		"is_activated": m.IsActivated,
		"is_banned":    m.IsBanned,
	}
	if m.RoleID != nil {
		patch["role_id"] = *m.RoleID
	}
	if m.SecondaryID != nil {
		patch["secondary_id"] = *m.SecondaryID
	}
	if strings.TrimSpace(m.UserID) != "" {
		patch["user_id"] = m.UserID
	}
	if strings.TrimSpace(m.Email) != "" {
		patch["email"] = strings.ToLower(m.Email)
	}
	if strings.TrimSpace(m.Name) != "" {
		patch["name"] = m.Name
	}
	if m.BannedReason != "" {
		patch["banned_reason"] = m.BannedReason
	}
	if m.ActivatedAt != nil {
		patch["activated_at"] = m.ActivatedAt
	}
	if m.InviteToken != nil {
		patch["invite_token"] = *m.InviteToken
	} else if m.IsActivated {
		patch["invite_token"] = nil
	}
	if m.InvitedBy != nil {
		patch["invited_by"] = *m.InvitedBy
	}
	if m.InvitedAt != nil {
		patch["invited_at"] = m.InvitedAt
	}
	return patch
}

// Save persists an already-loaded row.
func (r *MembershipRepository) Save(m *model.UserAccountSSO) error {
	if err := r.db.Save(m).Error; err != nil {
		return fmt.Errorf("save membership %s: %w", m.ID, err)
	}
	return nil
}

func (r *MembershipRepository) SoftDeleteByID(id string) error {
	if err := r.db.Where("id = ?", strings.TrimSpace(id)).Delete(&model.UserAccountSSO{}).Error; err != nil {
		return fmt.Errorf("soft-delete membership %s: %w", id, err)
	}
	return nil
}

func (r *MembershipRepository) CountActiveByRole(companyID, roleID string) (int64, error) {
	companyID = strings.TrimSpace(companyID)
	roleID = strings.TrimSpace(roleID)
	if companyID == "" || roleID == "" {
		return 0, nil
	}
	var n int64
	err := r.db.Model(&model.UserAccountSSO{}).
		Where("company_id = ? AND role_id = ? AND is_activated = true", companyID, roleID).
		Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("count active by role: %w", err)
	}
	return n, nil
}

func (r *MembershipRepository) CountConsumedInviteSlots(companyID, ownerRoleID string) (int64, error) {
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return 0, nil
	}
	q := r.db.Model(&model.UserAccountSSO{}).Where("company_id = ?", companyID)
	if s := strings.TrimSpace(ownerRoleID); s != "" {
		q = q.Where("role_id IS NULL OR role_id <> ?", s)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return 0, fmt.Errorf("count consumed invite slots: %w", err)
	}
	return n, nil
}
