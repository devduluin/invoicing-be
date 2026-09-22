package repository

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/audit"
	"duluin_invoice/app/model"
)

var auditColumns = []string{
	"id", "company_id", "actor_user_id", "actor_name", "actor_email",
	"action", "module", "entity_type", "entity_id", "entity_name",
	"description", "changes", "ip_address", "user_agent", "created_at",
}

type AuditRepository struct {
	db *gorm.DB
}

func NewAuditRepository(db *gorm.DB) domain.IRepository {
	return &AuditRepository{db: db}
}

// Insert is the only write this repository offers — audit rows are append-only.
func (r *AuditRepository) Insert(rec *model.AuditLog) error {
	if err := r.db.Create(rec).Error; err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

func (r *AuditRepository) List(f domain.Filter) ([]model.AuditLog, int64, error) {
	q := r.db.Model(&model.AuditLog{}).Where("company_id = ?", strings.TrimSpace(f.CompanyID))
	if len(f.Actions) > 0 {
		q = q.Where("action IN ?", f.Actions)
	}
	if len(f.Modules) > 0 {
		q = q.Where("module IN ?", f.Modules)
	}
	if s := strings.TrimSpace(f.UserID); s != "" {
		q = q.Where("actor_user_id = ?", s)
	}
	if f.From != nil {
		q = q.Where("created_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("created_at <= ?", *f.To)
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where(
			"lower(actor_name) LIKE ? OR lower(entity_name) LIKE ? OR lower(entity_id) LIKE ? OR lower(description) LIKE ?",
			like, like, like, like,
		)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	page, size := normalisePage(f.Page, f.PageSize)
	var rows []model.AuditLog
	err := q.Select(auditColumns).Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	return rows, total, nil
}

func (r *AuditRepository) FindByID(companyID, id string) (*model.AuditLog, error) {
	var rec model.AuditLog
	err := r.db.Select(auditColumns).
		Where("company_id = ? AND id = ?", strings.TrimSpace(companyID), strings.TrimSpace(id)).
		First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find audit log: %w", err)
	}
	return &rec, nil
}
