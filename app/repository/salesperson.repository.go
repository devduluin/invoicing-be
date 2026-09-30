package repository

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/salesperson"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

const salespersonCodePrefix = "SLS-"

var salespersonColumns = []string{
	"id", "company_id", "code", "name", "email", "phone", "user_id", "is_active",
	"created_at", "created_by", "updated_at", "updated_by",
}

var salespersonListColumns = []string{"code", "name", "email", "phone", "user_id", "is_active", "created_at", "updated_at"}

type SalespersonRepository struct{ db *gorm.DB }

func NewSalespersonRepository(db *gorm.DB) domain.IRepository {
	return &SalespersonRepository{db: db}
}

func (r *SalespersonRepository) Create(dto *domain.CreateDTO, actorID string) (*model.Salesperson, error) {
	name := strings.TrimSpace(dto.Name)
	if name == "" {
		return nil, &domain.ErrValidation{Message: "name is required"}
	}
	var id string
	err := r.db.Transaction(func(tx *gorm.DB) error {
		code, err := claimSalespersonCode(tx, dto.CompanyID, dto.Code, "")
		if err != nil {
			return err
		}
		userID := strings.TrimSpace(dto.UserID)
		if err := checkSalespersonUser(tx, dto.CompanyID, userID, ""); err != nil {
			return err
		}
		s := &model.Salesperson{
			ID:        uuid.NewString(),
			CompanyID: dto.CompanyID,
			Code:      code,
			Name:      name,
			Email:     strings.TrimSpace(dto.Email),
			Phone:     strings.TrimSpace(dto.Phone),
			UserID:    userID,
			IsActive:  utils.BoolInt(dto.IsActive == nil || *dto.IsActive),
			CreatedBy: actorID,
			UpdatedBy: actorID,
		}
		if err := tx.Create(s).Error; err != nil {
			if isUniqueViolation(err) {
				return &domain.ErrConflict{Message: "this code or team member is already used by another salesperson"}
			}
			return fmt.Errorf("create salesperson: %w", err)
		}
		id = s.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.FindByID(dto.CompanyID, id)
}

func (r *SalespersonRepository) Update(companyID, id string, dto *domain.UpdateDTO, actorID string) (*model.Salesperson, error) {
	existing, err := r.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{"updated_at": time.Now(), "updated_by": actorID}
	err = r.db.Transaction(func(tx *gorm.DB) error {
		if c := strings.TrimSpace(dto.Code); c != "" && c != existing.Code {
			code, err := claimSalespersonCode(tx, companyID, c, id)
			if err != nil {
				return err
			}
			updates["code"] = code
		}
		if v := strings.TrimSpace(dto.Name); v != "" {
			updates["name"] = v
		}
		if dto.Email != nil {
			updates["email"] = strings.TrimSpace(*dto.Email)
		}
		if dto.Phone != nil {
			updates["phone"] = strings.TrimSpace(*dto.Phone)
		}
		if dto.UserID != nil {
			userID := strings.TrimSpace(*dto.UserID)
			if err := checkSalespersonUser(tx, companyID, userID, id); err != nil {
				return err
			}
			updates["user_id"] = userID
		}
		if dto.IsActive != nil {
			updates["is_active"] = utils.BoolInt(*dto.IsActive)
		}
		if err := tx.Model(&model.Salesperson{}).Where("id = ? AND company_id = ?", id, companyID).Updates(updates).Error; err != nil {
			if isUniqueViolation(err) {
				return &domain.ErrConflict{Message: "this code or team member is already used by another salesperson"}
			}
			return fmt.Errorf("update salesperson %s: %w", id, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.FindByID(companyID, id)
}

func (r *SalespersonRepository) FindByID(companyID, id string) (*model.Salesperson, error) {
	var s model.Salesperson
	err := r.db.Select(salespersonColumns).Where("id = ? AND company_id = ?", id, companyID).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &domain.ErrNotFound{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("find salesperson %s: %w", id, err)
	}
	return &s, nil
}

func (r *SalespersonRepository) FindByUser(companyID, userID string) (*model.Salesperson, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, nil
	}
	var s model.Salesperson
	err := r.db.Select(salespersonColumns).Where("company_id = ? AND user_id = ?", companyID, userID).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find salesperson of user: %w", err)
	}
	return &s, nil
}

func (r *SalespersonRepository) FindAll(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	q := r.db.Model(&model.Salesperson{}).Where("company_id = ?", f.CompanyID)
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where("lower(name) LIKE ? OR lower(code) LIKE ? OR lower(email) LIKE ?", like, like, like)
	}
	if f.IsActive != nil {
		q = q.Where("is_active = ?", utils.BoolInt(*f.IsActive).ToInt())
	}
	order := f.Order
	if strings.TrimSpace(order) == "" {
		order = "ASC"
	}
	return utils.GetPaginatedDataOffset(q, utils.OffsetPaginationOptions{
		Model:                &model.Salesperson{},
		Page:                 f.Page,
		Limit:                f.PageSize,
		Sort:                 utils.NormalizeSort(f.Sort, salespersonListColumns, "name"),
		Order:                order,
		Select:               f.Fields,
		ValidColumns:         salespersonListColumns,
		PreserveAssociations: true,
	})
}

// Delete refuses a salesperson still on a live sales order or invoice (deactivate it instead).
func (r *SalespersonRepository) Delete(companyID, id string) error {
	if _, err := r.FindByID(companyID, id); err != nil {
		return err
	}
	_, summary, err := countRefs(r.db, []refQuery{
		directRefs("sales_orders", "salesperson_id", "sales order(s)", companyID, id),
		directRefs("sales_invoices", "salesperson_id", "sales invoice(s)", companyID, id),
	})
	if err != nil {
		return err
	}
	if summary != "" {
		return &utils.ErrInUse{Message: fmt.Sprintf("This salesperson is on %s — deactivate it instead.", summary)}
	}
	if err := r.db.Where("id = ? AND company_id = ?", id, companyID).Delete(&model.Salesperson{}).Error; err != nil {
		return fmt.Errorf("delete salesperson %s: %w", id, err)
	}
	return nil
}

func (r *SalespersonRepository) NextCode(companyID string) (string, error) {
	return nextSalespersonCode(r.db, companyID)
}

// TeamMembers — the company's accepted, not-banned members, with the salesperson linked to each.
func (r *SalespersonRepository) TeamMembers(companyID string) ([]domain.TeamMember, error) {
	var rows []domain.TeamMember
	err := r.db.Raw(`
		SELECT m.user_id, m.name, m.email, m.phone, COALESCE(s.id::text, '') AS salesperson_id
		FROM user_account_sso m
		LEFT JOIN salespersons s ON s.company_id = m.company_id AND s.user_id = m.user_id AND s.deleted_at IS NULL
		WHERE m.company_id = ? AND m.deleted_at IS NULL AND m.user_id <> '' AND NOT m.is_banned
		ORDER BY lower(COALESCE(NULLIF(m.name, ''), m.email))`, companyID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list team members: %w", err)
	}
	return rows, nil
}

// CreateFromMembers creates one salesperson per listed member that has none yet, in one transaction.
func (r *SalespersonRepository) CreateFromMembers(companyID string, userIDs []string, actorID string) ([]model.Salesperson, error) {
	members, err := r.TeamMembers(companyID)
	if err != nil {
		return nil, err
	}
	byUser := make(map[string]domain.TeamMember, len(members))
	for _, m := range members {
		byUser[m.UserID] = m
	}
	var created []model.Salesperson
	err = r.db.Transaction(func(tx *gorm.DB) error {
		seen := map[string]bool{}
		for _, uid := range userIDs {
			uid = strings.TrimSpace(uid)
			m, ok := byUser[uid]
			if uid == "" || seen[uid] {
				continue
			}
			seen[uid] = true
			if !ok {
				return &domain.ErrValidation{Message: fmt.Sprintf("user %s is not a member of this company", uid)}
			}
			if m.SalespersonID != "" {
				continue // already a salesperson
			}
			code, err := claimSalespersonCode(tx, companyID, "", "")
			if err != nil {
				return err
			}
			name := strings.TrimSpace(m.Name)
			if name == "" {
				name = m.Email
			}
			s := model.Salesperson{
				ID: uuid.NewString(), CompanyID: companyID, Code: code, Name: name,
				Email: m.Email, Phone: m.Phone, UserID: uid, IsActive: utils.BoolInt(true),
				CreatedBy: actorID, UpdatedBy: actorID,
			}
			if err := tx.Create(&s).Error; err != nil {
				return fmt.Errorf("create salesperson from member: %w", err)
			}
			created = append(created, s)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// ── codes & links ──────────────────────────────────────────────────────────────────────────────

// claimSalespersonCode — the requested code once checked free among the company's active
// salespersons (exceptID = the one being edited), or the next SLS-NNNN when blank (under a
// per-company transaction lock so concurrent creates can't get the same code).
func claimSalespersonCode(tx *gorm.DB, companyID, requested, exceptID string) (string, error) {
	code := strings.ToUpper(strings.TrimSpace(requested))
	if code == "" {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "salesperson_code:"+companyID).Error; err != nil {
			return "", fmt.Errorf("lock salesperson codes: %w", err)
		}
		return nextSalespersonCode(tx, companyID)
	}
	q := tx.Model(&model.Salesperson{}).Where("company_id = ? AND lower(code) = lower(?)", companyID, code)
	if exceptID != "" {
		q = q.Where("id <> ?", exceptID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return "", fmt.Errorf("check salesperson code: %w", err)
	}
	if n > 0 {
		return "", &domain.ErrConflict{Message: fmt.Sprintf("salesperson code %s is already in use", code)}
	}
	return code, nil
}

// nextSalespersonCode — SLS-<max+1>; deleted salespersons count too, so a code is never reused.
func nextSalespersonCode(db *gorm.DB, companyID string) (string, error) {
	var codes []string
	if err := db.Unscoped().Model(&model.Salesperson{}).
		Where("company_id = ? AND upper(code) LIKE ?", companyID, salespersonCodePrefix+"%").
		Pluck("code", &codes).Error; err != nil {
		return "", fmt.Errorf("scan salesperson codes: %w", err)
	}
	return fmt.Sprintf("%s%04d", salespersonCodePrefix, nextPrefixedSequence(salespersonCodePrefix, codes)), nil
}

// nextPrefixedSequence — max(N)+1 over codes shaped PREFIX<N>; other codes are ignored.
func nextPrefixedSequence(prefix string, codes []string) int {
	max := 0
	for _, c := range codes {
		if len(c) <= len(prefix) || !strings.EqualFold(c[:len(prefix)], prefix) {
			continue
		}
		if n, err := strconv.Atoi(c[len(prefix):]); err == nil && n > max {
			max = n
		}
	}
	return max + 1
}

// checkSalespersonUser — a linked team member must be an accepted member of the company and not
// linked to another salesperson (exceptID = the one being edited). "" = no link, always fine.
func checkSalespersonUser(tx *gorm.DB, companyID, userID, exceptID string) error {
	if userID == "" {
		return nil
	}
	var n int64
	if err := tx.Model(&model.UserAccountSSO{}).
		Where("company_id = ? AND user_id = ?", companyID, userID).Count(&n).Error; err != nil {
		return fmt.Errorf("check team member: %w", err)
	}
	if n == 0 {
		return &domain.ErrValidation{Message: "that user is not a member of this company"}
	}
	var other model.Salesperson
	q := tx.Select("id", "code", "name").Where("company_id = ? AND user_id = ?", companyID, userID)
	if exceptID != "" {
		q = q.Where("id <> ?", exceptID)
	}
	err := q.First(&other).Error
	if err == nil {
		return &domain.ErrConflict{Message: fmt.Sprintf("that team member is already salesperson %s · %s", other.Code, other.Name)}
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("check salesperson link: %w", err)
	}
	return nil
}

// ── documents ──────────────────────────────────────────────────────────────────────────────────

// errSalesperson — the salesperson picked on a document can't be used (the document's repository
// turns it into its own validation error).
type errSalesperson struct{ msg string }

func (e *errSalesperson) Error() string { return e.msg }

// salespersonSnapshot resolves the salesperson of a sales order / invoice: the id to store and the
// name to snapshot. No id = no link, and the free-text name the client sent is kept (older clients).
// A salesperson must belong to the company and be active — unless it is the one the document already
// had (keepID), so an old document can still be saved after its salesperson was deactivated.
func salespersonSnapshot(tx *gorm.DB, companyID string, id *string, text, keepID string) (*string, string, error) {
	sid := strings.TrimSpace(derefStr(id))
	if sid == "" {
		return nil, strings.TrimSpace(text), nil
	}
	var s model.Salesperson
	err := tx.Select("id", "name", "is_active").Where("id = ? AND company_id = ?", sid, companyID).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", &errSalesperson{"salesperson not found"}
	}
	if err != nil {
		return nil, "", fmt.Errorf("find salesperson: %w", err)
	}
	if !bool(s.IsActive) && sid != keepID {
		return nil, "", &errSalesperson{fmt.Sprintf("salesperson %s is inactive", s.Name)}
	}
	return &s.ID, s.Name, nil
}

// ── one-time backfill ──────────────────────────────────────────────────────────────────────────

const salespersonBackfillKey = "salesperson_master_v1"

// BackfillSalespersonsOnce turns the free-text salesperson names already on sales orders and
// invoices into salesperson records (one per company per name, case/space-insensitive, first
// spelling wins) and links those documents to them. One transaction, recorded so it runs once.
func BackfillSalespersonsOnce(db *gorm.DB) error {
	var rec model.MigrationRecord
	err := db.Where("key = ?", salespersonBackfillKey).First(&rec).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("check salesperson backfill record: %w", err)
	}
	created := 0
	err = db.Transaction(func(tx *gorm.DB) error {
		type row struct {
			CompanyID string
			Name      string
		}
		var names []row
		if err := tx.Raw(`
			SELECT company_id, name FROM (
				SELECT company_id, trim(salesperson) AS name, created_at FROM sales_orders WHERE trim(coalesce(salesperson, '')) <> ''
				UNION ALL
				SELECT company_id, trim(salesperson) AS name, created_at FROM sales_invoices WHERE trim(coalesce(salesperson, '')) <> ''
			) t ORDER BY company_id, created_at`).Scan(&names).Error; err != nil {
			return fmt.Errorf("list salesperson names: %w", err)
		}
		key := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
		ids := map[string]string{} // company|key -> salesperson id
		next := map[string]int{}
		for _, n := range names {
			k := n.CompanyID + "|" + key(n.Name)
			if _, ok := ids[k]; ok {
				continue
			}
			if _, ok := next[n.CompanyID]; !ok {
				var codes []string
				if err := tx.Unscoped().Model(&model.Salesperson{}).
					Where("company_id = ? AND upper(code) LIKE ?", n.CompanyID, salespersonCodePrefix+"%").
					Pluck("code", &codes).Error; err != nil {
					return err
				}
				next[n.CompanyID] = nextPrefixedSequence(salespersonCodePrefix, codes)
			}
			s := model.Salesperson{
				ID: uuid.NewString(), CompanyID: n.CompanyID,
				Code:      fmt.Sprintf("%s%04d", salespersonCodePrefix, next[n.CompanyID]),
				Name:      strings.Join(strings.Fields(n.Name), " "),
				IsActive:  utils.BoolInt(true),
				CreatedBy: "system", UpdatedBy: "system",
			}
			next[n.CompanyID]++
			if err := tx.Create(&s).Error; err != nil {
				return fmt.Errorf("create salesperson %q: %w", s.Name, err)
			}
			ids[k] = s.ID
			created++
		}
		for k, id := range ids {
			companyID, name, _ := strings.Cut(k, "|")
			for _, table := range []string{"sales_orders", "sales_invoices"} {
				if err := tx.Exec(fmt.Sprintf(`UPDATE %s SET salesperson_id = ?
					WHERE company_id = ? AND salesperson_id IS NULL
					AND lower(regexp_replace(trim(salesperson), '\s+', ' ', 'g')) = ?`, table), id, companyID, name).Error; err != nil {
					return fmt.Errorf("link %s to salesperson: %w", table, err)
				}
			}
		}
		return tx.Create(&model.MigrationRecord{Key: salespersonBackfillKey}).Error
	})
	if err != nil {
		return err
	}
	if created > 0 {
		log.Printf("✅ Salesperson backfill applied: %d salesperson(s)", created)
	}
	return nil
}
