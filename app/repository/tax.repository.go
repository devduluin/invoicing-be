package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/tax"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

var taxColumns = []string{
	"id", "company_id", "name", "kind", "rate", "is_system", "is_active",
	"calc_method", "sales_account_id", "purchase_account_id",
	"is_compound", "component1_id", "component2_id",
	"created_at", "created_by", "updated_at", "updated_by",
}

// taxListColumns — fields the MasterTable may show / sort by.
var taxListColumns = []string{
	"name", "kind", "rate", "calc_method", "is_system", "is_active",
	"is_compound", "sales_account_id", "purchase_account_id",
	"created_at", "updated_at",
}

type TaxRepository struct{ db *gorm.DB }

func NewTaxRepository(db *gorm.DB) domain.IRepository { return &TaxRepository{db: db} }

func (r *TaxRepository) Create(dto *domain.CreateDTO, actorID string) (*model.Tax, error) {
	t := &model.Tax{
		ID:        uuid.NewString(),
		CompanyID: dto.CompanyID,
		Name:      strings.TrimSpace(dto.Name),
		IsActive:  utils.BoolInt(dto.IsActive == nil || *dto.IsActive),
		CreatedBy: actorID,
		UpdatedBy: actorID,
	}

	if dto.IsCompound {
		c1, c2, rate, err := r.resolveComponents(dto.CompanyID, dto.Component1ID, dto.Component2ID)
		if err != nil {
			return nil, err
		}
		t.IsCompound = utils.BoolInt(true)
		t.Kind = model.TaxKindOther
		t.CalcMethod = model.TaxCalcExclusive
		t.Rate = rate
		t.Component1ID = &c1
		t.Component2ID = &c2
	} else {
		if err := r.applySingleFields(t, dto.CompanyID, dto.Kind, dto.CalcMethod, dto.Rate, dto.SalesAccountID, dto.PurchaseAccountID); err != nil {
			return nil, err
		}
	}

	if err := r.db.Create(t).Error; err != nil {
		return nil, fmt.Errorf("create tax: %w", err)
	}
	return r.FindByID(dto.CompanyID, t.ID)
}

func (r *TaxRepository) Update(companyID, id string, dto *domain.UpdateDTO, actorID string) (*model.Tax, error) {
	existing, err := r.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}

	renaming := (dto.Name != "" && dto.Name != existing.Name) ||
		(dto.Kind != "" && model.TaxKind(dto.Kind) != existing.Kind)
	if bool(existing.IsSystem) && renaming {
		return nil, &domain.ErrSystemLocked{}
	}

	updates := map[string]any{"updated_at": time.Now(), "updated_by": actorID}
	if v := strings.TrimSpace(dto.Name); v != "" {
		updates["name"] = v
	}
	if dto.IsActive != nil {
		updates["is_active"] = utils.BoolInt(*dto.IsActive)
	}

	if bool(existing.IsCompound) {
		// Only the pair (and name/active) is mutable on a compound tax.
		c1 := ptrOr(dto.Component1ID, existing.Component1ID)
		c2 := ptrOr(dto.Component2ID, existing.Component2ID)
		if dto.Component1ID != nil || dto.Component2ID != nil {
			r1, r2, rate, err := r.resolveComponents(companyID, c1, c2)
			if err != nil {
				return nil, err
			}
			updates["component1_id"] = r1
			updates["component2_id"] = r2
			updates["rate"] = rate
		}
	} else {
		if dto.Kind != "" {
			updates["kind"] = dto.Kind
		}
		if dto.CalcMethod != "" {
			updates["calc_method"] = dto.CalcMethod
		}
		if dto.Rate != nil {
			updates["rate"] = *dto.Rate
		}
		if dto.SalesAccountID != nil {
			acc := strings.TrimSpace(*dto.SalesAccountID)
			if acc == "" {
				return nil, &domain.ErrValidation{Message: "sales tax account is required"}
			}
			if err := r.mustAccountExist(companyID, acc); err != nil {
				return nil, err
			}
			updates["sales_account_id"] = acc
		}
		if dto.PurchaseAccountID != nil {
			acc := strings.TrimSpace(*dto.PurchaseAccountID)
			if acc == "" {
				return nil, &domain.ErrValidation{Message: "purchase tax account is required"}
			}
			if err := r.mustAccountExist(companyID, acc); err != nil {
				return nil, err
			}
			updates["purchase_account_id"] = acc
		}
	}

	if err := r.db.Model(&model.Tax{}).
		Where("id = ? AND company_id = ?", id, companyID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update tax %s: %w", id, err)
	}

	// A single tax's rate feeds any compound that references it.
	if _, ok := updates["rate"]; ok && !bool(existing.IsCompound) {
		if err := r.refreshCompoundRates(companyID, id); err != nil {
			return nil, err
		}
	}
	return r.FindByID(companyID, id)
}

func (r *TaxRepository) FindByID(companyID, id string) (*model.Tax, error) {
	var t model.Tax
	err := r.db.Select(taxColumns).Where("id = ? AND company_id = ?", id, companyID).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &domain.ErrNotFound{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("find tax %s: %w", id, err)
	}
	return &t, nil
}

func (r *TaxRepository) FindAll(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	q := r.db.Model(&model.Tax{}).Where("company_id = ?", f.CompanyID)
	if s := strings.TrimSpace(f.Search); s != "" {
		q = q.Where("lower(name) LIKE ?", "%"+strings.ToLower(s)+"%")
	}
	order := f.Order
	if strings.TrimSpace(order) == "" {
		order = "ASC"
	}
	return utils.GetPaginatedDataOffset(q, utils.OffsetPaginationOptions{
		Model:                &model.Tax{},
		Page:                 f.Page,
		Limit:                f.PageSize,
		Sort:                 utils.NormalizeSort(f.Sort, taxListColumns, "name"),
		Order:                order,
		Select:               f.Fields,
		ValidColumns:         taxListColumns,
		PreserveAssociations: true,
	})
}

func (r *TaxRepository) Delete(companyID, id string) error {
	existing, err := r.FindByID(companyID, id)
	if err != nil {
		return err
	}
	if bool(existing.IsSystem) {
		return &domain.ErrValidation{Message: "a built-in system tax can't be deleted — deactivate it instead"}
	}
	if !bool(existing.IsCompound) {
		var n int64
		if err := r.db.Model(&model.Tax{}).
			Where("company_id = ? AND (component1_id = ? OR component2_id = ?)", companyID, id, id).
			Count(&n).Error; err != nil {
			return fmt.Errorf("check tax component usage: %w", err)
		}
		if n > 0 {
			return &domain.ErrComponentInUse{}
		}
	}
	if err := checkTaxUnused(r.db, companyID, id); err != nil {
		return err
	}
	if err := r.db.Where("id = ? AND company_id = ?", id, companyID).Delete(&model.Tax{}).Error; err != nil {
		return fmt.Errorf("delete tax %s: %w", id, err)
	}
	return nil
}

// SeedDefaults inserts DefaultTaxes for a company that has none yet. Idempotent.
// Account codes are resolved to the company's own account ids (best-effort — a
// company self-healing taxes before its chart exists gets null accounts).
func (r *TaxRepository) SeedDefaults(companyID, actorID string) error {
	var n int64
	if err := r.db.Model(&model.Tax{}).Where("company_id = ?", companyID).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	codeToID, err := r.accountIDsByCode(companyID)
	if err != nil {
		return err
	}
	pick := func(code string) *string {
		if id, ok := codeToID[code]; ok {
			return &id
		}
		return nil
	}

	seeds := model.DefaultTaxes()
	rows := make([]model.Tax, 0, len(seeds))
	for _, s := range seeds {
		rows = append(rows, model.Tax{
			ID:                uuid.NewString(),
			CompanyID:         companyID,
			Name:              s.Name,
			Kind:              s.Kind,
			CalcMethod:        s.CalcMethod,
			Rate:              s.Rate,
			IsSystem:          utils.BoolInt(true),
			IsActive:          utils.BoolInt(true),
			SalesAccountID:    pick(s.SalesAccountCode),
			PurchaseAccountID: pick(s.PurchaseAccountCode),
			CreatedBy:         actorID,
			UpdatedBy:         actorID,
		})
	}
	if err := r.db.Create(&rows).Error; err != nil {
		return fmt.Errorf("seed default taxes: %w", err)
	}
	return nil
}

// --- helpers ---------------------------------------------------------------

func (r *TaxRepository) applySingleFields(t *model.Tax, companyID, kind, calc string, rate float64, salesAcc, purchaseAcc string) error {
	salesAcc, purchaseAcc = strings.TrimSpace(salesAcc), strings.TrimSpace(purchaseAcc)
	if salesAcc == "" || purchaseAcc == "" {
		return &domain.ErrValidation{Message: "sales and purchase tax accounts are required"}
	}
	if err := r.mustAccountExist(companyID, salesAcc); err != nil {
		return err
	}
	if err := r.mustAccountExist(companyID, purchaseAcc); err != nil {
		return err
	}
	if kind == "" {
		kind = string(model.TaxKindOther)
	}
	if calc == "" {
		calc = string(model.TaxCalcExclusive)
	}
	t.Kind = model.TaxKind(kind)
	t.CalcMethod = model.TaxCalcMethod(calc)
	t.Rate = rate
	t.SalesAccountID = &salesAcc
	t.PurchaseAccountID = &purchaseAcc
	return nil
}

// resolveComponents validates the two component ids and returns them trimmed
// plus their summed rate.
func (r *TaxRepository) resolveComponents(companyID, c1, c2 string) (string, string, float64, error) {
	c1, c2 = strings.TrimSpace(c1), strings.TrimSpace(c2)
	if c1 == "" || c2 == "" {
		return "", "", 0, &domain.ErrValidation{Message: "a compound tax needs two single taxes"}
	}
	if c1 == c2 {
		return "", "", 0, &domain.ErrValidation{Message: "the two single taxes must be different"}
	}
	var comps []model.Tax
	if err := r.db.Select("id", "rate", "is_compound").
		Where("company_id = ? AND id IN ?", companyID, []string{c1, c2}).
		Find(&comps).Error; err != nil {
		return "", "", 0, fmt.Errorf("load tax components: %w", err)
	}
	if len(comps) != 2 {
		return "", "", 0, &domain.ErrValidation{Message: "single tax not found"}
	}
	var rate float64
	for _, c := range comps {
		if bool(c.IsCompound) {
			return "", "", 0, &domain.ErrValidation{Message: "a compound tax component must be a single tax"}
		}
		rate += c.Rate
	}
	return c1, c2, rate, nil
}

func (r *TaxRepository) mustAccountExist(companyID, accountID string) error {
	var n int64
	if err := r.db.Table("accounts").
		Where("id = ? AND company_id = ? AND deleted_at IS NULL", accountID, companyID).
		Count(&n).Error; err != nil {
		return fmt.Errorf("check account %s: %w", accountID, err)
	}
	if n == 0 {
		return &domain.ErrValidation{Message: "the selected account was not found in the chart of accounts"}
	}
	return nil
}

func (r *TaxRepository) accountIDsByCode(companyID string) (map[string]string, error) {
	var rows []struct {
		Code string
		ID   string
	}
	if err := r.db.Table("accounts").Select("code", "id").
		Where("company_id = ? AND deleted_at IS NULL", companyID).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("load accounts for tax seed: %w", err)
	}
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		out[row.Code] = row.ID
	}
	return out, nil
}

// refreshCompoundRates re-sums, in one set-based statement, every compound tax
// that references the given single tax.
func (r *TaxRepository) refreshCompoundRates(companyID, singleID string) error {
	const q = `
UPDATE taxes AS parent
SET rate = COALESCE(s1.rate, 0) + COALESCE(s2.rate, 0),
    updated_at = now()
FROM taxes AS ref
LEFT JOIN taxes AS s1 ON s1.id = ref.component1_id AND s1.deleted_at IS NULL
LEFT JOIN taxes AS s2 ON s2.id = ref.component2_id AND s2.deleted_at IS NULL
WHERE parent.id = ref.id
  AND parent.company_id = ?
  AND parent.is_compound = 1
  AND parent.deleted_at IS NULL
  AND (ref.component1_id = ? OR ref.component2_id = ?)`
	if err := r.db.Exec(q, companyID, singleID, singleID).Error; err != nil {
		return fmt.Errorf("refresh compound rates: %w", err)
	}
	return nil
}

func ptrOr(v *string, fallback *string) string {
	if v != nil {
		return strings.TrimSpace(*v)
	}
	if fallback != nil {
		return *fallback
	}
	return ""
}
