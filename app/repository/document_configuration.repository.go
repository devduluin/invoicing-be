package repository

import (
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	domain "duluin_invoice/app/domain/documentconfig"
	"duluin_invoice/app/model"
)

type DocumentConfigurationRepository struct{ db *gorm.DB }

func NewDocumentConfigurationRepository(db *gorm.DB) domain.IRepository {
	return &DocumentConfigurationRepository{db: db}
}

func toItem(row *model.DocumentConfiguration) domain.Item {
	ts := row.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	return domain.Item{DocType: row.DocType, Config: json.RawMessage(row.Config), UpdatedAt: &ts}
}

func defaultItem(docType string) domain.Item {
	return domain.Item{DocType: docType, Config: json.RawMessage(`{}`), IsDefault: true}
}

// List returns every document type; a type the company never configured comes back as the
// (empty) default so the caller renders the built-in defaults.
func (r *DocumentConfigurationRepository) List(companyID string) ([]domain.Item, error) {
	var rows []model.DocumentConfiguration
	if err := r.db.Where("company_id = ?", companyID).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list document configurations: %w", err)
	}
	byType := make(map[string]model.DocumentConfiguration, len(rows))
	for _, row := range rows {
		byType[row.DocType] = row
	}
	out := make([]domain.Item, 0, len(model.DocumentConfigTypes))
	for _, t := range model.DocumentConfigTypes {
		if row, ok := byType[t]; ok {
			out = append(out, toItem(&row))
		} else {
			out = append(out, defaultItem(t))
		}
	}
	return out, nil
}

func (r *DocumentConfigurationRepository) Get(companyID, docType string) (*domain.Item, error) {
	var row model.DocumentConfiguration
	err := r.db.Where("company_id = ? AND doc_type = ?", companyID, docType).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		d := defaultItem(docType)
		return &d, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get document configuration: %w", err)
	}
	it := toItem(&row)
	return &it, nil
}

func (r *DocumentConfigurationRepository) Save(companyID, docType, configJSON, actorID string) (*domain.Item, error) {
	row := model.DocumentConfiguration{CompanyID: companyID, DocType: docType, Config: configJSON, UpdatedBy: actorID}
	err := r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "company_id"}, {Name: "doc_type"}},
		DoUpdates: clause.AssignmentColumns([]string{"config", "updated_by", "updated_at"}),
	}).Create(&row).Error
	if err != nil {
		return nil, fmt.Errorf("save document configuration: %w", err)
	}
	return r.Get(companyID, docType)
}

// Reset removes the stored row: the document type falls back to the built-in defaults.
func (r *DocumentConfigurationRepository) Reset(companyID, docType string) error {
	if err := r.db.Where("company_id = ? AND doc_type = ?", companyID, docType).Delete(&model.DocumentConfiguration{}).Error; err != nil {
		return fmt.Errorf("reset document configuration: %w", err)
	}
	return nil
}
