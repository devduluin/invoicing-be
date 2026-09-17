package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/account"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

var accountColumns = []string{
	"id", "company_id", "parent_id", "code", "name", "group", "is_system", "is_active",
	"created_at", "created_by", "updated_at", "updated_by",
}

// accountListColumns — fields the MasterTable may show / sort by.
var accountListColumns = []string{
	"code", "name", "group", "parent_id", "is_system", "is_active", "created_at", "updated_at",
}

type AccountRepository struct{ db *gorm.DB }

func NewAccountRepository(db *gorm.DB) domain.IRepository { return &AccountRepository{db: db} }

func (r *AccountRepository) Create(dto *domain.CreateDTO, actorID string) (*model.Account, error) {
	code := strings.TrimSpace(dto.Code)
	if exists, err := r.CodeExists(dto.CompanyID, code, ""); err != nil {
		return nil, err
	} else if exists {
		return nil, &domain.ErrCodeExists{Code: code}
	}
	if err := r.validateParent(dto.CompanyID, dto.ParentID); err != nil {
		return nil, err
	}

	a := &model.Account{
		ID:        uuid.NewString(),
		CompanyID: dto.CompanyID,
		ParentID:  trimPtr(dto.ParentID),
		Code:      code,
		Name:      strings.TrimSpace(dto.Name),
		Group:     model.AccountGroup(dto.Group),
		IsActive:  utils.BoolInt(dto.IsActive == nil || *dto.IsActive),
		CreatedBy: actorID,
		UpdatedBy: actorID,
	}
	if err := r.db.Create(a).Error; err != nil {
		return nil, fmt.Errorf("create account: %w", err)
	}
	return r.FindByID(dto.CompanyID, a.ID)
}

func (r *AccountRepository) Update(companyID, id string, dto *domain.UpdateDTO, actorID string) (*model.Account, error) {
	existing, err := r.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{"updated_at": time.Now(), "updated_by": actorID}

	if v := strings.TrimSpace(dto.Code); v != "" && v != existing.Code {
		if bool(existing.IsSystem) {
			return nil, &domain.ErrSystemLocked{Msg: "the code of a built-in account can't be changed"}
		}
		if exists, err := r.CodeExists(companyID, v, id); err != nil {
			return nil, err
		} else if exists {
			return nil, &domain.ErrCodeExists{Code: v}
		}
		updates["code"] = v
	}
	if dto.Group != "" && model.AccountGroup(dto.Group) != existing.Group {
		if bool(existing.IsSystem) {
			return nil, &domain.ErrSystemLocked{Msg: "the classification of a built-in account can't be changed"}
		}
		updates["group"] = dto.Group
	}
	if v := strings.TrimSpace(dto.Name); v != "" {
		updates["name"] = v // rename allowed even for system accounts
	}
	if dto.ParentID != nil {
		p := trimPtr(dto.ParentID)
		if p != nil && *p == id {
			return nil, &domain.ErrValidation{Message: "an account can't be its own parent"}
		}
		if err := r.validateParent(companyID, dto.ParentID); err != nil {
			return nil, err
		}
		updates["parent_id"] = p
	}
	if dto.IsActive != nil {
		updates["is_active"] = utils.BoolInt(*dto.IsActive)
	}

	if err := r.db.Model(&model.Account{}).
		Where("id = ? AND company_id = ?", id, companyID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update account %s: %w", id, err)
	}
	return r.FindByID(companyID, id)
}

func (r *AccountRepository) FindByID(companyID, id string) (*model.Account, error) {
	var a model.Account
	err := r.db.Select(accountColumns).Where("id = ? AND company_id = ?", id, companyID).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &domain.ErrNotFound{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("find account %s: %w", id, err)
	}
	return &a, nil
}

// FindAll — the COA list, paginated + sortable like every other master-data
// table (the FE renders it flat, not as a tree).
func (r *AccountRepository) FindAll(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	q := r.db.Model(&model.Account{}).Where("company_id = ?", f.CompanyID)
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where("lower(name) LIKE ? OR code LIKE ?", like, like)
	}
	if f.Group != "" {
		q = q.Where("\"group\" = ?", f.Group)
	}

	sortCol := utils.NormalizeSort(f.Sort, accountListColumns, "code")
	if sortCol == "group" {
		sortCol = `"group"` // reserved word
	}
	order := f.Order
	if strings.TrimSpace(order) == "" {
		order = "ASC"
	}
	return utils.GetPaginatedDataOffset(q, utils.OffsetPaginationOptions{
		Model:                &model.Account{},
		Page:                 f.Page,
		Limit:                f.PageSize,
		Sort:                 sortCol,
		Order:                order,
		Select:               f.Fields,
		ValidColumns:         accountListColumns,
		PreserveAssociations: true,
	})
}

func (r *AccountRepository) Delete(companyID, id string) error {
	existing, err := r.FindByID(companyID, id)
	if err != nil {
		return err
	}
	if bool(existing.IsSystem) {
		return &domain.ErrSystemLocked{Msg: "a built-in account can't be deleted — deactivate it instead"}
	}
	if kids, err := r.HasChildren(companyID, id); err != nil {
		return err
	} else if kids {
		return &domain.ErrHasChildren{}
	}
	if err := r.db.Where("id = ? AND company_id = ?", id, companyID).Delete(&model.Account{}).Error; err != nil {
		return fmt.Errorf("delete account %s: %w", id, err)
	}
	return nil
}

func (r *AccountRepository) CodeExists(companyID, code, exceptID string) (bool, error) {
	q := r.db.Model(&model.Account{}).Where("company_id = ? AND lower(code) = lower(?)", companyID, code)
	if exceptID != "" {
		q = q.Where("id <> ?", exceptID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return false, fmt.Errorf("check account code: %w", err)
	}
	return n > 0, nil
}

func (r *AccountRepository) HasChildren(companyID, id string) (bool, error) {
	var n int64
	err := r.db.Model(&model.Account{}).Where("company_id = ? AND parent_id = ?", companyID, id).Count(&n).Error
	if err != nil {
		return false, fmt.Errorf("check account children: %w", err)
	}
	return n > 0, nil
}

func (r *AccountRepository) validateParent(companyID string, parentID *string) error {
	p := trimPtr(parentID)
	if p == nil {
		return nil
	}
	if _, err := r.FindByID(companyID, *p); err != nil {
		return &domain.ErrValidation{Message: "parent account not found"}
	}
	return nil
}

// SeedDefaults builds the generic COA template for a company that has none yet.
func (r *AccountRepository) SeedDefaults(companyID, actorID string) error {
	var n int64
	if err := r.db.Model(&model.Account{}).Where("company_id = ?", companyID).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	seeds := model.DefaultAccounts()
	byCode := make(map[string]string, len(seeds)) // code → id

	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, s := range seeds {
			row := model.Account{
				ID:        uuid.NewString(),
				CompanyID: companyID,
				Code:      s.Code,
				Name:      s.Name,
				Group:     s.Group,
				IsSystem:  utils.BoolInt(true),
				IsActive:  utils.BoolInt(true),
				CreatedBy: actorID,
				UpdatedBy: actorID,
			}
			if s.ParentCode != "" {
				if pid, ok := byCode[s.ParentCode]; ok {
					row.ParentID = &pid
				}
			}
			if err := tx.Create(&row).Error; err != nil {
				return fmt.Errorf("seed account %s: %w", s.Code, err)
			}
			byCode[s.Code] = row.ID
		}
		return nil
	})
}

func trimPtr(p *string) *string {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	if v == "" {
		return nil
	}
	return &v
}
