package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/bankaccount"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

var bankAccountColumns = []string{
	"id", "company_id", "bank_name", "bank_code", "account_number", "account_holder", "branch",
	"is_primary", "is_active", "created_at", "created_by", "updated_at", "updated_by",
}

// bankAccountListColumns — fields the MasterTable may show / sort by.
var bankAccountListColumns = []string{
	"bank_name", "bank_code", "account_number", "account_holder", "branch",
	"is_primary", "is_active", "created_at", "updated_at",
}

type BankAccountRepository struct{ db *gorm.DB }

func NewBankAccountRepository(db *gorm.DB) domain.IRepository {
	return &BankAccountRepository{db: db}
}

func (r *BankAccountRepository) Create(dto *domain.CreateDTO, actorID string) (*model.BankAccount, error) {
	b := &model.BankAccount{
		ID:            uuid.NewString(),
		CompanyID:     dto.CompanyID,
		BankName:      strings.TrimSpace(dto.BankName),
		BankCode:      strings.TrimSpace(dto.BankCode),
		AccountNumber: strings.TrimSpace(dto.AccountNumber),
		AccountHolder: strings.TrimSpace(dto.AccountHolder),
		Branch:        strings.TrimSpace(dto.Branch),
		IsPrimary:     utils.BoolInt(dto.IsPrimary != nil && *dto.IsPrimary),
		IsActive:      utils.BoolInt(dto.IsActive == nil || *dto.IsActive),
		CreatedBy:     actorID,
		UpdatedBy:     actorID,
	}
	if err := r.db.Create(b).Error; err != nil {
		return nil, fmt.Errorf("create bank account: %w", err)
	}
	return r.FindByID(dto.CompanyID, b.ID)
}

func (r *BankAccountRepository) Update(companyID, id string, dto *domain.UpdateDTO, actorID string) (*model.BankAccount, error) {
	if _, err := r.FindByID(companyID, id); err != nil {
		return nil, err
	}
	updates := map[string]any{"updated_at": time.Now(), "updated_by": actorID}
	if v := strings.TrimSpace(dto.BankName); v != "" {
		updates["bank_name"] = v
	}
	if dto.BankCode != nil {
		updates["bank_code"] = strings.TrimSpace(*dto.BankCode)
	}
	if v := strings.TrimSpace(dto.AccountNumber); v != "" {
		updates["account_number"] = v
	}
	if v := strings.TrimSpace(dto.AccountHolder); v != "" {
		updates["account_holder"] = v
	}
	if dto.Branch != nil {
		updates["branch"] = strings.TrimSpace(*dto.Branch)
	}
	if dto.IsPrimary != nil {
		updates["is_primary"] = utils.BoolInt(*dto.IsPrimary)
	}
	if dto.IsActive != nil {
		updates["is_active"] = utils.BoolInt(*dto.IsActive)
	}
	if err := r.db.Model(&model.BankAccount{}).
		Where("id = ? AND company_id = ?", id, companyID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update bank account %s: %w", id, err)
	}
	return r.FindByID(companyID, id)
}

func (r *BankAccountRepository) FindByID(companyID, id string) (*model.BankAccount, error) {
	var b model.BankAccount
	err := r.db.Select(bankAccountColumns).
		Where("id = ? AND company_id = ?", id, companyID).First(&b).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &domain.ErrNotFound{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("find bank account %s: %w", id, err)
	}
	return &b, nil
}

func (r *BankAccountRepository) FindAll(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	q := r.db.Model(&model.BankAccount{}).Where("company_id = ?", f.CompanyID)
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where("lower(bank_name) LIKE ? OR lower(account_holder) LIKE ? OR account_number LIKE ?", like, like, like)
	}
	order := f.Order
	if strings.TrimSpace(order) == "" {
		order = "ASC"
	}
	return utils.GetPaginatedDataOffset(q, utils.OffsetPaginationOptions{
		Model:                &model.BankAccount{},
		Page:                 f.Page,
		Limit:                f.PageSize,
		Sort:                 utils.NormalizeSort(f.Sort, bankAccountListColumns, "bank_name"),
		Order:                order,
		Select:               f.Fields,
		ValidColumns:         bankAccountListColumns,
		PreserveAssociations: true,
	})
}

func (r *BankAccountRepository) Delete(companyID, id string) error {
	res := r.db.Where("id = ? AND company_id = ?", id, companyID).Delete(&model.BankAccount{})
	if res.Error != nil {
		return fmt.Errorf("delete bank account %s: %w", id, res.Error)
	}
	if res.RowsAffected == 0 {
		return &domain.ErrNotFound{ID: id}
	}
	return nil
}

func (r *BankAccountRepository) ClearPrimary(companyID, exceptID string) error {
	err := r.db.Model(&model.BankAccount{}).
		Where("company_id = ? AND id <> ? AND is_primary = 1", companyID, exceptID).
		Update("is_primary", 0).Error
	if err != nil {
		return fmt.Errorf("clear primary bank account: %w", err)
	}
	return nil
}
