package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/journalbook"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

var journalBookColumns = []string{
	"id", "company_id", "code", "name", "type",
	"default_account_id", "default_debit_account_id", "default_credit_account_id",
	"is_system", "is_active", "created_at", "created_by", "updated_at", "updated_by",
}

// journalBookListColumns — fields the MasterTable may show / sort by.
var journalBookListColumns = []string{"code", "name", "type", "is_system", "is_active", "created_at"}

type JournalBookRepository struct{ db *gorm.DB }

func NewJournalBookRepository(db *gorm.DB) domain.IRepository { return &JournalBookRepository{db: db} }

func (r *JournalBookRepository) Create(dto *domain.CreateDTO, actorID string) (*model.JournalBook, error) {
	code := strings.TrimSpace(dto.Code)
	if exists, err := r.CodeExists(dto.CompanyID, code, ""); err != nil {
		return nil, err
	} else if exists {
		return nil, &domain.ErrCodeExists{Code: code}
	}

	b := &model.JournalBook{
		ID:                     uuid.NewString(),
		CompanyID:              dto.CompanyID,
		Code:                   code,
		Name:                   strings.TrimSpace(dto.Name),
		Type:                   model.JournalBookType(dto.Type),
		DefaultAccountID:       trimPtr(dto.DefaultAccountID),
		DefaultDebitAccountID:  trimPtr(dto.DefaultDebitAccountID),
		DefaultCreditAccountID: trimPtr(dto.DefaultCreditAccountID),
		IsActive:               utils.BoolInt(dto.IsActive == nil || *dto.IsActive),
		CreatedBy:              actorID,
		UpdatedBy:              actorID,
	}
	if err := r.db.Create(b).Error; err != nil {
		return nil, fmt.Errorf("create journal book: %w", err)
	}
	return r.FindByID(dto.CompanyID, b.ID)
}

func (r *JournalBookRepository) Update(companyID, id string, dto *domain.UpdateDTO, actorID string) (*model.JournalBook, error) {
	existing, err := r.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{"updated_at": time.Now(), "updated_by": actorID}

	if v := strings.TrimSpace(dto.Code); v != "" && v != existing.Code {
		if bool(existing.IsSystem) {
			return nil, &domain.ErrSystemLocked{Msg: "the code of a built-in journal book can't be changed"}
		}
		if exists, err := r.CodeExists(companyID, v, id); err != nil {
			return nil, err
		} else if exists {
			return nil, &domain.ErrCodeExists{Code: v}
		}
		updates["code"] = v
	}
	if dto.Type != "" && model.JournalBookType(dto.Type) != existing.Type {
		if bool(existing.IsSystem) {
			return nil, &domain.ErrSystemLocked{Msg: "the type of a built-in journal book can't be changed"}
		}
		updates["type"] = dto.Type
	}
	if v := strings.TrimSpace(dto.Name); v != "" {
		updates["name"] = v // rename allowed even for system books
	}
	if dto.DefaultAccountID != nil {
		updates["default_account_id"] = trimPtr(dto.DefaultAccountID)
	}
	if dto.DefaultDebitAccountID != nil {
		updates["default_debit_account_id"] = trimPtr(dto.DefaultDebitAccountID)
	}
	if dto.DefaultCreditAccountID != nil {
		updates["default_credit_account_id"] = trimPtr(dto.DefaultCreditAccountID)
	}
	if dto.IsActive != nil {
		updates["is_active"] = utils.BoolInt(*dto.IsActive)
	}

	if err := r.db.Model(&model.JournalBook{}).
		Where("id = ? AND company_id = ?", id, companyID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update journal book %s: %w", id, err)
	}
	return r.FindByID(companyID, id)
}

func (r *JournalBookRepository) FindByID(companyID, id string) (*model.JournalBook, error) {
	var b model.JournalBook
	err := r.db.Select(journalBookColumns).Where("id = ? AND company_id = ?", id, companyID).First(&b).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &domain.ErrNotFound{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("find journal book %s: %w", id, err)
	}
	return &b, nil
}

func (r *JournalBookRepository) FindAll(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	q := r.db.Model(&model.JournalBook{}).Where("company_id = ?", f.CompanyID)
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where("lower(name) LIKE ? OR lower(code) LIKE ?", like, like)
	}
	if f.Type != "" {
		q = q.Where("type = ?", f.Type)
	}

	sortCol := utils.NormalizeSort(f.Sort, journalBookListColumns, "code")
	order := f.Order
	if strings.TrimSpace(order) == "" {
		order = "ASC"
	}
	return utils.GetPaginatedDataOffset(q, utils.OffsetPaginationOptions{
		Model:                &model.JournalBook{},
		Page:                 f.Page,
		Limit:                f.PageSize,
		Sort:                 sortCol,
		Order:                order,
		Select:               f.Fields,
		ValidColumns:         journalBookListColumns,
		PreserveAssociations: true,
	})
}

func (r *JournalBookRepository) Delete(companyID, id string) error {
	existing, err := r.FindByID(companyID, id)
	if err != nil {
		return err
	}
	if bool(existing.IsSystem) {
		return &domain.ErrSystemLocked{Msg: "a built-in journal book can't be deleted — deactivate it instead"}
	}
	if used, err := r.HasEntries(companyID, id); err != nil {
		return err
	} else if used {
		return &domain.ErrHasEntries{}
	}
	if err := r.db.Where("id = ? AND company_id = ?", id, companyID).Delete(&model.JournalBook{}).Error; err != nil {
		return fmt.Errorf("delete journal book %s: %w", id, err)
	}
	return nil
}

func (r *JournalBookRepository) CodeExists(companyID, code, exceptID string) (bool, error) {
	q := r.db.Model(&model.JournalBook{}).Where("company_id = ? AND lower(code) = lower(?)", companyID, code)
	if exceptID != "" {
		q = q.Where("id <> ?", exceptID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return false, fmt.Errorf("check journal book code: %w", err)
	}
	return n > 0, nil
}

// HasEntries — the invoice-service equivalent of acc-master's isJournalUsed
// (simplified: no bank_accounts/company_settings tie-ins here).
func (r *JournalBookRepository) HasEntries(companyID, id string) (bool, error) {
	var n int64
	err := r.db.Model(&model.JournalEntry{}).
		Where("company_id = ? AND journal_book_id = ?", companyID, id).Count(&n).Error
	if err != nil {
		return false, fmt.Errorf("check journal book usage: %w", err)
	}
	return n > 0, nil
}

// SeedDefaults builds the 8-book Indonesia template for a company that has
// none yet (verbatim list from acc-master-service's indonesiaJournals()).
func (r *JournalBookRepository) SeedDefaults(companyID, actorID string) error {
	var n int64
	if err := r.db.Model(&model.JournalBook{}).Where("company_id = ?", companyID).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, d := range model.DefaultJournalBooks() {
			row := model.JournalBook{
				ID:        uuid.NewString(),
				CompanyID: companyID,
				Code:      d.Code,
				Name:      d.Name,
				Type:      d.Type,
				IsSystem:  utils.BoolInt(true),
				IsActive:  utils.BoolInt(true),
				CreatedBy: actorID,
				UpdatedBy: actorID,
			}
			if err := tx.Create(&row).Error; err != nil {
				return fmt.Errorf("seed journal book %s: %w", d.Code, err)
			}
		}
		return nil
	})
}
