package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	contactdomain "duluin_invoice/app/domain/contactperson"
	domain "duluin_invoice/app/domain/mitra"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// mitraListColumns — fields the MasterTable may show / sort by.
var mitraListColumns = []string{
	"code", "linked_company_code", "type", "name", "contact_name", "email", "phone", "npwp", "address", "is_active", "created_at", "updated_at", "created_by", "linked_company_id", "updated_by",
}

// mitraColumns is the explicit projection for mitra reads (avoids SELECT *).
var mitraColumns = []string{
	"id", "company_id", "code", "linked_company_id", "linked_company_code", "type", "name", "contact_name", "email", "phone", "npwp", "address",
	"is_active", "created_at", "created_by", "updated_at", "updated_by",
}

type MitraRepository struct {
	db *gorm.DB
}

func NewMitraRepository(db *gorm.DB) domain.IMitraRepository {
	return &MitraRepository{db: db}
}

// Create — 1 INSERT + 1 SELECT (re-read for the response).
func (r *MitraRepository) Create(dto *domain.CreateMitraDTO, actorID string) (*model.Mitra, error) {
	var mitra *model.Mitra
	// The partner and its contact persons are saved together or not at all.
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var err error
		mitra, err = createMitraTx(tx, dto, actorID)
		return err
	})
	if err != nil {
		return nil, mapContactErr(err)
	}
	return r.FindByID(dto.CompanyID, mitra.ID)
}

// CreateMany — the import: every partner (with its contacts) in ONE transaction, so a file either
// lands completely or not at all. A failing partner is named in the error.
func (r *MitraRepository) CreateMany(dtos []*domain.CreateMitraDTO, actorID string) ([]*model.Mitra, error) {
	out := make([]*model.Mitra, 0, len(dtos))
	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, dto := range dtos {
			m, err := createMitraTx(tx, dto, actorID)
			if err != nil {
				var dup *domain.ErrCodeExists
				var linked *domain.ErrCompanyLinked
				var invalid *domain.ErrValidation
				if mapped := mapContactErr(err); mapped != err || errors.As(err, &dup) || errors.As(err, &linked) || errors.As(err, &invalid) {
					return &domain.ErrValidation{Message: dto.Name + ": " + mapped.Error()}
				}
				return err
			}
			out = append(out, m)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// createMitraTx inserts one partner, its PIC contact and its extra contacts inside tx.
func createMitraTx(tx *gorm.DB, dto *domain.CreateMitraDTO, actorID string) (*model.Mitra, error) {
	code, err := claimMitraCode(tx, dto.CompanyID, dto.Code, "")
	if err != nil {
		return nil, err
	}
	linkedID, linkedCode, err := resolveLinkedCompany(tx, dto.CompanyID, dto.LinkedCompanyCode, "")
	if err != nil {
		return nil, err
	}
	mitra := &model.Mitra{
		ID:                uuid.New().String(),
		CompanyID:         dto.CompanyID,
		Code:              code,
		LinkedCompanyID:   linkedID,
		LinkedCompanyCode: linkedCode,
		Type:              model.MitraType(dto.Type),
		Name:              dto.Name,
		ContactName:       dto.ContactName,
		Email:             dto.Email,
		Phone:             dto.Phone,
		Npwp:              dto.Npwp,
		Address:           dto.Address,
		IsActive:          utils.BoolInt(true),
		CreatedBy:         actorID,
		UpdatedBy:         actorID,
	}
	if dto.IsActive != nil {
		mitra.IsActive = utils.BoolInt(*dto.IsActive)
	}
	if err := tx.Create(mitra).Error; err != nil {
		if uniqueViolationOn(err, "uq_mitra_company_linked_active") {
			return nil, &domain.ErrCompanyLinked{Code: linkedCode}
		}
		if isUniqueViolation(err) {
			return nil, &domain.ErrCodeExists{Code: code}
		}
		return nil, fmt.Errorf("create mitra: %w", err)
	}
	// The PIC becomes the partner's first contact person, automatically.
	pn, pe, pp := picName(dto.ContactName, dto.Name), strings.TrimSpace(dto.Email), strings.TrimSpace(dto.Phone)
	if err := createPICContact(tx, dto.CompanyID, mitra.ID, actorID, pn, pe, pp); err != nil {
		return nil, err
	}
	// A contact the user also typed by hand that equals the PIC is the same person: skip it.
	extra := make([]contactdomain.SyncInput, 0, len(dto.ContactPersons))
	for _, c := range dto.ContactPersons {
		if strings.EqualFold(strings.TrimSpace(c.Name), pn) && strings.EqualFold(strings.TrimSpace(c.Email), pe) && strings.TrimSpace(c.Phone) == pp {
			continue
		}
		extra = append(extra, c)
	}
	// Creating a partner already needs invoice-mitra-create; extra contacts need the contact permission.
	// (Created one by one, not through syncContacts, which would treat the PIC row as "missing".)
	for _, c := range extra {
		if !dto.ContactPerms.Create {
			return nil, &contactdomain.ErrForbidden{Action: "add"}
		}
		if _, err := createContact(tx, dto.CompanyID, mitra.ID, actorID, contactdomain.Input{Name: c.Name, Position: c.Position, Phone: c.Phone, Email: c.Email}); err != nil {
			return nil, err
		}
	}
	return mitra, nil
}

// Update — 1 SELECT (existence) + 1 UPDATE + 1 SELECT (re-read).
func (r *MitraRepository) Update(companyID, id string, dto *domain.UpdateMitraDTO, actorID string) (*model.Mitra, error) {
	before, err := r.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}

	updates := mitraUpdateMap(dto, actorID)
	err = r.db.Transaction(func(tx *gorm.DB) error {
		if c := strings.TrimSpace(dto.Code); c != "" && c != before.Code {
			code, err := claimMitraCode(tx, companyID, c, id)
			if err != nil {
				return err
			}
			updates["code"] = code
		}
		if dto.LinkedCompanyCode != nil && !strings.EqualFold(strings.TrimSpace(*dto.LinkedCompanyCode), before.LinkedCompanyCode) {
			linkedID, linkedCode, err := resolveLinkedCompany(tx, companyID, *dto.LinkedCompanyCode, id)
			if err != nil {
				return err
			}
			updates["linked_company_id"] = linkedID
			updates["linked_company_code"] = linkedCode
		}
		if dto.ContactPersons != nil {
			if err := lockMitra(tx, companyID, id); err != nil {
				return err
			}
		}
		if err := tx.Model(&model.Mitra{}).
			Where("id = ? AND company_id = ?", id, companyID).
			Updates(updates).Error; err != nil {
			if uniqueViolationOn(err, "uq_mitra_company_linked_active") {
				return &domain.ErrCompanyLinked{Code: fmt.Sprint(updates["linked_company_code"])}
			}
			if isUniqueViolation(err) {
				return &domain.ErrCodeExists{Code: fmt.Sprint(updates["code"])}
			}
			return fmt.Errorf("update mitra %s: %w", id, err)
		}
		if dto.ContactPersons != nil {
			if err := syncContacts(tx, companyID, id, actorID, dto.ContactPersons, dto.ContactPerms); err != nil {
				return err
			}
		}
		// Changed PIC details flow into the PIC contact (after the list sync, so it wins over stale rows).
		return syncPICContact(tx, companyID, id, actorID, before, dto.ContactName, dto.Email, dto.Phone)
	})
	if err != nil {
		return nil, mapContactErr(err)
	}
	return r.FindByID(companyID, id)
}

func mitraUpdateMap(dto *domain.UpdateMitraDTO, actorID string) map[string]interface{} {
	updates := map[string]interface{}{"updated_at": time.Now(), "updated_by": actorID}
	if dto.Type != "" {
		updates["type"] = dto.Type
	}
	if dto.Name != "" {
		updates["name"] = dto.Name
	}
	if dto.ContactName != "" {
		updates["contact_name"] = dto.ContactName
	}
	if dto.Email != "" {
		updates["email"] = dto.Email
	}
	if dto.Phone != "" {
		updates["phone"] = dto.Phone
	}
	if dto.Npwp != "" {
		updates["npwp"] = dto.Npwp
	}
	if dto.Address != nil {
		updates["address"] = *dto.Address
	}
	if dto.IsActive != nil {
		updates["is_active"] = utils.BoolInt(*dto.IsActive)
	}
	return updates
}

// FindByID — 1 SELECT, company-scoped.
func (r *MitraRepository) FindByID(companyID, id string) (*model.Mitra, error) {
	var mitra model.Mitra
	err := r.db.Select(mitraColumns).
		Where("id = ? AND company_id = ?", id, companyID).
		First(&mitra).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &domain.ErrNotFound{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("find mitra %s: %w", id, err)
	}
	return &mitra, nil
}

// FindAll — 1 COUNT + 1 paginated SELECT. No relations, no N+1.
func (r *MitraRepository) FindAll(filter *domain.MitraFilter) (*utils.OffsetPaginationResult, error) {
	return utils.GetPaginatedDataOffset(r.mitraFilterQuery(filter), utils.OffsetPaginationOptions{
		Model:                &model.Mitra{},
		Page:                 filter.Page,
		Limit:                filter.PageSize,
		Sort:                 utils.NormalizeSort(filter.Sort, mitraListColumns, "created_at"),
		Order:                filter.Order,
		Select:               filter.Fields,
		ValidColumns:         mitraListColumns,
		PreserveAssociations: true,
		PreloadRelations: []utils.PreloadRelation{
			{Name: "LinkedCompanyRel", Columns: []string{"id", "code", "name"}},
		},
		KeepColumns: []string{"linked_company_id", "created_by", "updated_by"},
		ExternalJoins: []utils.ExternalJoin{
			{Key: "created_by", As: "created_by_rel", Resolver: memberNameResolver(r.db, filter.CompanyID)},
			{Key: "updated_by", As: "updated_by_rel", Resolver: memberNameResolver(r.db, filter.CompanyID)},
		},
	})
}

func (r *MitraRepository) mitraFilterQuery(filter *domain.MitraFilter) *gorm.DB {
	query := r.db.Model(&model.Mitra{}).Where("company_id = ?", filter.CompanyID)
	if filter.Search != "" {
		s := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(email) LIKE ? OR LOWER(code) LIKE ?", s, s, s)
	}
	if filter.Type != "" {
		// A partner marked "both" is valid on either side, so a caller asking for "customer" or
		// "supplier" also gets the "both" ones — only an explicit "both" filter means exactly that.
		switch filter.Type {
		case "customer", "supplier":
			query = query.Where("type IN ?", []string{filter.Type, "both"})
		default:
			query = query.Where("type = ?", filter.Type)
		}
	}
	if filter.IsActive != nil {
		query = query.Where("is_active = ?", utils.BoolInt(*filter.IsActive).ToInt())
	}
	return query
}

func normalisePage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	return page, pageSize
}

// Delete — 1 DELETE (soft), company-scoped.
func (r *MitraRepository) Delete(companyID, id string) error {
	if _, err := r.FindByID(companyID, id); err != nil {
		return err
	}
	if err := checkMitraUnused(r.db, companyID, id); err != nil {
		return err
	}
	result := r.db.Where("id = ? AND company_id = ?", id, companyID).Delete(&model.Mitra{})
	if result.Error != nil {
		return fmt.Errorf("delete mitra %s: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return &domain.ErrNotFound{ID: id}
	}
	return nil
}

// mapContactErr turns contact-person errors raised while saving a partner into the partner
// domain's own validation error (permission errors stay as they are, so the controller answers 403).
func mapContactErr(err error) error {
	var v *contactdomain.ErrValidation
	var d *contactdomain.ErrDuplicate
	var n *contactdomain.ErrNotFound
	if errors.As(err, &v) || errors.As(err, &d) || errors.As(err, &n) {
		return &domain.ErrValidation{Message: err.Error()}
	}
	return err
}

// ── linked Duluin company ──────────────────────────────────────────────────────────────────────

// resolveLinkedCompany turns a Duluin Company Code into the company to link: "" = no link. The company
// must be an active Duluin company other than the caller's own, and not already linked to another of
// the caller's partners (exceptID = the partner being edited).
func resolveLinkedCompany(tx *gorm.DB, companyID, rawCode, exceptID string) (*string, string, error) {
	code := strings.TrimSpace(rawCode)
	if code == "" {
		return nil, "", nil
	}
	var c model.Company
	err := tx.Select("id", "code").
		Where("lower(code) = lower(?) AND onboarding_status = ?", code, model.OnboardingActive).
		First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && c.ID == companyID) {
		return nil, "", &domain.ErrValidation{Message: fmt.Sprintf("Duluin Company Code %s not found", code)}
	}
	if err != nil {
		return nil, "", fmt.Errorf("find linked company: %w", err)
	}
	var other model.Mitra
	q := tx.Select("id", "code", "name").Where("company_id = ? AND linked_company_id = ?", companyID, c.ID)
	if exceptID != "" {
		q = q.Where("id <> ?", exceptID)
	}
	err = q.First(&other).Error
	if err == nil {
		label := other.Name
		if other.Code != "" {
			label = other.Code + " · " + other.Name
		}
		return nil, "", &domain.ErrCompanyLinked{Code: c.Code, Partner: label}
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", fmt.Errorf("check linked company: %w", err)
	}
	id := c.ID
	return &id, c.Code, nil
}

// uniqueViolationOn — a unique violation of that particular index.
func uniqueViolationOn(err error, index string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == index
}

// ── partner codes ──────────────────────────────────────────────────────────────────────────────

const mitraCodePrefix = "MTR-"

// NextCode previews the code a new partner would get now (not reserved: another create may take it).
func (r *MitraRepository) NextCode(companyID string) (string, error) {
	return nextMitraCode(r.db, companyID)
}

// claimMitraCode returns the code to store: the requested one (trimmed) once checked free among the
// company's active partners (exceptID = the partner being edited), or the next MTR-NNNN when blank.
// Generation takes a per-company transaction lock so two concurrent creates can't get the same code.
func claimMitraCode(tx *gorm.DB, companyID, requested, exceptID string) (string, error) {
	code := strings.TrimSpace(requested)
	if code == "" {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "mitra_code:"+companyID).Error; err != nil {
			return "", fmt.Errorf("lock partner codes: %w", err)
		}
		return nextMitraCode(tx, companyID)
	}
	q := tx.Model(&model.Mitra{}).Where("company_id = ? AND lower(code) = lower(?)", companyID, code)
	if exceptID != "" {
		q = q.Where("id <> ?", exceptID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return "", fmt.Errorf("check partner code: %w", err)
	}
	if n > 0 {
		return "", &domain.ErrCodeExists{Code: code}
	}
	return code, nil
}

// nextMitraCode — MTR-<max+1>, zero-padded to 4 digits. Deleted partners count too, so a generated
// code is never handed out twice.
func nextMitraCode(db *gorm.DB, companyID string) (string, error) {
	var codes []string
	if err := db.Unscoped().Model(&model.Mitra{}).
		Where("company_id = ? AND upper(code) LIKE ?", companyID, mitraCodePrefix+"%").
		Pluck("code", &codes).Error; err != nil {
		return "", fmt.Errorf("scan partner codes: %w", err)
	}
	return fmt.Sprintf("%s%04d", mitraCodePrefix, nextMitraSequence(codes)), nil
}

func nextMitraSequence(codes []string) int { return nextPrefixedSequence(mitraCodePrefix, codes) }

// CountActive — how many partners this company has (soft-deleted rows excluded by GORM by
// default). Used by the activation milestone and the Free-tier partner limit.
func (r *MitraRepository) CountActive(companyID string) (int64, error) {
	var n int64
	if err := r.db.Model(&model.Mitra{}).Where("company_id = ?", companyID).Count(&n).Error; err != nil {
		return 0, fmt.Errorf("count partners: %w", err)
	}
	return n, nil
}
