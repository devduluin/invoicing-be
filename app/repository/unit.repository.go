package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/unit"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

var unitColumns = []string{
	"id", "company_id", "name", "symbol", "is_system", "is_active",
	"created_at", "created_by", "updated_at", "updated_by",
}

var unitListColumns = []string{"name", "symbol", "is_system", "is_active", "created_at", "updated_at"}

type UnitRepository struct{ db *gorm.DB }

func NewUnitRepository(db *gorm.DB) domain.IRepository { return &UnitRepository{db: db} }

func (r *UnitRepository) Create(dto *domain.CreateDTO, actorID string) (*model.Unit, error) {
	name := strings.TrimSpace(dto.Name)
	if name == "" {
		return nil, &domain.ErrValidation{Message: "name is required"}
	}
	u := &model.Unit{
		ID:        uuid.NewString(),
		CompanyID: dto.CompanyID,
		Name:      name,
		Symbol:    strings.TrimSpace(dto.Symbol),
		IsActive:  utils.BoolInt(dto.IsActive == nil || *dto.IsActive),
		CreatedBy: actorID,
		UpdatedBy: actorID,
	}
	if err := r.db.Create(u).Error; err != nil {
		return nil, fmt.Errorf("create unit: %w", err)
	}
	return r.FindByID(dto.CompanyID, u.ID)
}

func (r *UnitRepository) Update(companyID, id string, dto *domain.UpdateDTO, actorID string) (*model.Unit, error) {
	existing, err := r.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}

	renaming := dto.Name != "" && dto.Name != existing.Name
	if bool(existing.IsSystem) && renaming {
		return nil, &domain.ErrSystemLocked{}
	}

	updates := map[string]any{"updated_at": time.Now(), "updated_by": actorID}
	if v := strings.TrimSpace(dto.Name); v != "" {
		updates["name"] = v
	}
	if dto.Symbol != nil {
		updates["symbol"] = strings.TrimSpace(*dto.Symbol)
	}
	if dto.IsActive != nil {
		updates["is_active"] = utils.BoolInt(*dto.IsActive)
	}

	if err := r.db.Model(&model.Unit{}).
		Where("id = ? AND company_id = ?", id, companyID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update unit %s: %w", id, err)
	}
	return r.FindByID(companyID, id)
}

func (r *UnitRepository) FindByID(companyID, id string) (*model.Unit, error) {
	var u model.Unit
	err := r.db.Select(unitColumns).Where("id = ? AND company_id = ?", id, companyID).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &domain.ErrNotFound{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("find unit %s: %w", id, err)
	}
	return &u, nil
}

func (r *UnitRepository) FindAll(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	q := r.db.Model(&model.Unit{}).Where("company_id = ?", f.CompanyID)
	if s := strings.TrimSpace(f.Search); s != "" {
		q = q.Where("lower(name) LIKE ?", "%"+strings.ToLower(s)+"%")
	}
	order := f.Order
	if strings.TrimSpace(order) == "" {
		order = "ASC"
	}
	return utils.GetPaginatedDataOffset(q, utils.OffsetPaginationOptions{
		Model:                &model.Unit{},
		Page:                 f.Page,
		Limit:                f.PageSize,
		Sort:                 utils.NormalizeSort(f.Sort, unitListColumns, "name"),
		Order:                order,
		Select:               f.Fields,
		ValidColumns:         unitListColumns,
		PreserveAssociations: true,
	})
}

func (r *UnitRepository) Delete(companyID, id string) error {
	existing, err := r.FindByID(companyID, id)
	if err != nil {
		return err
	}
	if bool(existing.IsSystem) {
		return &domain.ErrValidation{Message: "a built-in system unit can't be deleted — deactivate it instead"}
	}
	if err := checkUnitUnused(r.db, companyID, existing.Name); err != nil {
		return err
	}
	if err := r.db.Where("id = ? AND company_id = ?", id, companyID).Delete(&model.Unit{}).Error; err != nil {
		return fmt.Errorf("delete unit %s: %w", id, err)
	}
	return nil
}

// SeedDefaults inserts DefaultUnits for a company that has none yet. Idempotent.
func (r *UnitRepository) SeedDefaults(companyID, actorID string) error {
	var n int64
	if err := r.db.Model(&model.Unit{}).Where("company_id = ?", companyID).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	seeds := model.DefaultUnits()
	rows := make([]model.Unit, 0, len(seeds))
	for _, s := range seeds {
		rows = append(rows, model.Unit{
			ID:        uuid.NewString(),
			CompanyID: companyID,
			Name:      s.Name,
			Symbol:    s.Symbol,
			IsSystem:  utils.BoolInt(true),
			IsActive:  utils.BoolInt(true),
			CreatedBy: actorID,
			UpdatedBy: actorID,
		})
	}
	if err := r.db.Create(&rows).Error; err != nil {
		return fmt.Errorf("seed default units: %w", err)
	}
	return nil
}
