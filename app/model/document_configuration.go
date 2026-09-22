package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DocumentConfigTypes — every document type that has its own PDF/document configuration
// (names, labels, visible fields/columns, notes/terms defaults, signature, language).
var DocumentConfigTypes = []string{
	"sales_order", "down_payment", "sales_invoice", "sales_receipt", "delivery_note",
	"purchase_order", "purchase_invoice", "purchase_receipt", "goods_receipt",
}

func IsValidDocumentConfigType(v string) bool {
	for _, t := range DocumentConfigTypes {
		if t == v {
			return true
		}
	}
	return false
}

// DocumentConfiguration is one company's configuration for ONE document type. Config holds the
// validated JSON the document renderer (preview, print and PDF) reads; a missing row means the
// built-in defaults. Kept separate per (company, document type): changing one never touches another.
type DocumentConfiguration struct {
	ID        string    `gorm:"type:uuid;primaryKey"                              json:"id"`
	CompanyID string    `gorm:"type:uuid;not null;uniqueIndex:idx_doc_cfg"        json:"company_id"`
	DocType   string    `gorm:"type:varchar(30);not null;uniqueIndex:idx_doc_cfg" json:"doc_type"`
	Config    string    `gorm:"type:jsonb;not null;default:'{}'"                  json:"-"`
	UpdatedBy string    `gorm:"type:varchar(64)"                                  json:"updated_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (DocumentConfiguration) TableName() string { return "document_configurations" }

func (d *DocumentConfiguration) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	return nil
}
