package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	contactdomain "duluin_invoice/app/domain/contactperson"
	domain "duluin_invoice/app/domain/mitra"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// mitraListColumns — fields the MasterTable may show / sort by.
var mitraListColumns = []string{
	"type", "name", "contact_name", "email", "phone", "npwp", "address",
	"is_active", "created_at", "updated_at",
}

// mitraColumns is the explicit projection for mitra reads (avoids SELECT *).
var mitraColumns = []string{
	"id", "company_id", "type", "name", "contact_name", "email", "phone", "npwp", "address",
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
	mitra := &model.Mitra{
		ID:          uuid.New().String(),
		CompanyID:   dto.CompanyID,
		Type:        model.MitraType(dto.Type),
		Name:        dto.Name,
		ContactName: dto.ContactName,
		Email:       dto.Email,
		Phone:       dto.Phone,
		Npwp:        dto.Npwp,
		Address:     dto.Address,
		IsActive:    utils.BoolInt(true),
		CreatedBy:   actorID,
		UpdatedBy:   actorID,
	}
	if dto.IsActive != nil {
		mitra.IsActive = utils.BoolInt(*dto.IsActive)
	}
	// The partner and its contact persons are saved together or not at all.
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(mitra).Error; err != nil {
			return fmt.Errorf("create mitra: %w", err)
		}
		if len(dto.ContactPersons) == 0 {
			return nil
		}
		// creating a partner already needs invoice-mitra-create; the contacts need the contact permission
		return syncContacts(tx, dto.CompanyID, mitra.ID, actorID, dto.ContactPersons, contactdomain.Perms{Create: dto.ContactPerms.Create})
	})
	if err != nil {
		return nil, mapContactErr(err)
	}
	return r.FindByID(dto.CompanyID, mitra.ID)
}

// Update — 1 SELECT (existence) + 1 UPDATE + 1 SELECT (re-read).
func (r *MitraRepository) Update(companyID, id string, dto *domain.UpdateMitraDTO, actorID string) (*model.Mitra, error) {
	if _, err := r.FindByID(companyID, id); err != nil {
		return nil, err
	}

	updates := mitraUpdateMap(dto, actorID)
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if dto.ContactPersons != nil {
			if err := lockMitra(tx, companyID, id); err != nil {
				return err
			}
		}
		if err := tx.Model(&model.Mitra{}).
			Where("id = ? AND company_id = ?", id, companyID).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("update mitra %s: %w", id, err)
		}
		if dto.ContactPersons != nil {
			return syncContacts(tx, companyID, id, actorID, dto.ContactPersons, dto.ContactPerms)
		}
		return nil
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
	})
}

func (r *MitraRepository) mitraFilterQuery(filter *domain.MitraFilter) *gorm.DB {
	query := r.db.Model(&model.Mitra{}).Where("company_id = ?", filter.CompanyID)
	if filter.Search != "" {
		s := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(email) LIKE ?", s, s)
	}
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
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

// CountActive — how many partners this company has (soft-deleted rows excluded by GORM by
// default). Used by the activation milestone and the Free-tier partner limit.
func (r *MitraRepository) CountActive(companyID string) (int64, error) {
	var n int64
	if err := r.db.Model(&model.Mitra{}).Where("company_id = ?", companyID).Count(&n).Error; err != nil {
		return 0, fmt.Errorf("count partners: %w", err)
	}
	return n, nil
}
