package repository

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	domain "duluin_invoice/app/domain/documenttemplate"
	"duluin_invoice/app/model"
)

type DocumentTemplateRepository struct{ db *gorm.DB }

func NewDocumentTemplateRepository(db *gorm.DB) domain.IRepository {
	return &DocumentTemplateRepository{db: db}
}

// List returns every supported document type with its effective default for this company.
func (r *DocumentTemplateRepository) List(companyID string) ([]domain.Item, error) {
	var rows []model.DocumentTemplateDefault
	if err := r.db.Where("company_id = ?", companyID).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list document template defaults: %w", err)
	}
	byType := make(map[string]string, len(rows))
	for _, row := range rows {
		byType[row.DocType] = row.Template
	}
	out := make([]domain.Item, 0, len(model.DocumentTemplateDefaultTypes))
	for _, t := range model.DocumentTemplateDefaultTypes {
		tpl := byType[t]
		if !model.IsValidSalesInvoiceTemplate(tpl) {
			tpl = string(model.DefaultSalesInvoiceTemplate)
		}
		out = append(out, domain.Item{DocType: t, Template: tpl})
	}
	return out, nil
}

func (r *DocumentTemplateRepository) Set(companyID, docType, template, actorID string) (*domain.Item, error) {
	row := model.DocumentTemplateDefault{CompanyID: companyID, DocType: docType, Template: template, UpdatedBy: actorID}
	err := r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "company_id"}, {Name: "doc_type"}},
		DoUpdates: clause.AssignmentColumns([]string{"template", "updated_by", "updated_at"}),
	}).Create(&row).Error
	if err != nil {
		return nil, fmt.Errorf("set document template default: %w", err)
	}
	return &domain.Item{DocType: docType, Template: template}, nil
}

// documentTemplateFor is what a NEW document starts with: the company's default for that type, or
// template_1. Existing documents never call this; they keep the template saved on them.
func documentTemplateFor(db *gorm.DB, companyID, docType string) string {
	var row model.DocumentTemplateDefault
	err := db.Where("company_id = ? AND doc_type = ?", companyID, docType).First(&row).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return string(model.DefaultSalesInvoiceTemplate)
		}
		return string(model.DefaultSalesInvoiceTemplate)
	}
	if !model.IsValidSalesInvoiceTemplate(row.Template) {
		return string(model.DefaultSalesInvoiceTemplate)
	}
	return row.Template
}

// templateForDoc: an explicit choice wins; otherwise the company's default for that document type.
func templateForDoc(db *gorm.DB, companyID, docType, explicit string) string {
	if v := strings.TrimSpace(explicit); v != "" {
		return v
	}
	return documentTemplateFor(db, companyID, docType)
}
