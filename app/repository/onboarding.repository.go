package repository

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/onboarding"
	"duluin_invoice/app/model"
)

// companyColumns — explicit projection (avoid SELECT *).
var companyColumns = []string{
	"id", "name", "code", "owner_name", "company_logo", "email", "phone", "npwp", "alamat", "kota", "provinsi", "kode_pos",
	"onboarding_status", "tipe_akun", "jenis_usaha", "jumlah_karyawan", "kebutuhan_user",
	"email_verified", "phone_verified", "free_transaction_limit_idr",
	"created_at", "created_by", "updated_at", "updated_by",
}

type OnboardingRepository struct {
	db *gorm.DB
}

func NewOnboardingRepository(db *gorm.DB) *OnboardingRepository {
	return &OnboardingRepository{db: db}
}

var _ domain.IOnboardingRepository = (*OnboardingRepository)(nil)

// FindCompanyByID — 1 point SELECT.
func (r *OnboardingRepository) FindCompanyByID(id string) (*model.Company, error) {
	var c model.Company
	err := r.db.Select(companyColumns).Where("id = ?", strings.TrimSpace(id)).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find company: %w", err)
	}
	return &c, nil
}

// FindByID satisfies domain_membership.CompanyReader.
func (r *OnboardingRepository) FindByID(id string) (*model.Company, error) {
	return r.FindCompanyByID(id)
}

// FindByCode — directory lookup by the shareable Company ID. Only returns
// fully-onboarded companies. (nil, nil) when not found.
func (r *OnboardingRepository) FindByCode(code string) (*model.Company, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, nil
	}
	var c model.Company
	err := r.db.Select(companyColumns).
		Where("lower(code) = lower(?) AND onboarding_status = ?", code, model.OnboardingActive).
		First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find company by code: %w", err)
	}
	return &c, nil
}

// CreateCompany — 1 INSERT.
func (r *OnboardingRepository) CreateCompany(c *model.Company) error {
	if err := r.db.Create(c).Error; err != nil {
		return fmt.Errorf("create company: %w", err)
	}
	return nil
}

// SaveCompany — 1 UPDATE (full row). Used by the company-settings flow.
func (r *OnboardingRepository) SaveCompany(c *model.Company) error {
	if err := r.db.Save(c).Error; err != nil {
		return fmt.Errorf("save company %s: %w", c.ID, err)
	}
	return nil
}

// HardDeleteCompany — permanent delete (compensating action).
func (r *OnboardingRepository) HardDeleteCompany(id string) error {
	if err := r.db.Unscoped().Where("id = ?", strings.TrimSpace(id)).Delete(&model.Company{}).Error; err != nil {
		return fmt.Errorf("hard-delete company %s: %w", id, err)
	}
	return nil
}

// CompanyCodeExists — 1 COUNT on the unique code.
func (r *OnboardingRepository) CompanyCodeExists(code string) (bool, error) {
	var n int64
	err := r.db.Model(&model.Company{}).Where("lower(code) = lower(?)", strings.TrimSpace(code)).Count(&n).Error
	if err != nil {
		return false, fmt.Errorf("check company code: %w", err)
	}
	return n > 0, nil
}
