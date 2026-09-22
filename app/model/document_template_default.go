package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Document types that have a printable template. Only these can carry a company default;
// receipts, delivery notes and goods receipts have no template, so they are not listed.
const (
	DocTypeSalesInvoice    = "sales_invoice"
	DocTypeDownPayment     = "down_payment"
	DocTypeSalesOrder      = "sales_order"
	DocTypePurchaseOrder   = "purchase_order"
	DocTypePurchaseInvoice = "purchase_invoice"
)

// DocumentTemplateDefaultTypes — every doc type a default can be stored for.
var DocumentTemplateDefaultTypes = []string{DocTypeSalesInvoice, DocTypeDownPayment, DocTypeSalesOrder, DocTypePurchaseInvoice, DocTypePurchaseOrder}

func IsValidDocumentTemplateType(v string) bool {
	for _, t := range DocumentTemplateDefaultTypes {
		if t == v {
			return true
		}
	}
	return false
}

// DocumentTemplateDefault is a company's default template for one document type. It is only ever
// read when a NEW document is created; existing documents keep the template stored on them.
type DocumentTemplateDefault struct {
	ID        string    `gorm:"type:uuid;primaryKey"                                json:"id"`
	CompanyID string    `gorm:"type:uuid;not null;uniqueIndex:idx_doc_tpl_default"  json:"company_id"`
	DocType   string    `gorm:"type:varchar(30);not null;uniqueIndex:idx_doc_tpl_default" json:"doc_type"`
	Template  string    `gorm:"type:varchar(20);not null"                           json:"template"`
	UpdatedBy string    `gorm:"type:varchar(64)"                                    json:"updated_by,omitempty"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"                                      json:"updated_at"`
}

func (DocumentTemplateDefault) TableName() string { return "document_template_defaults" }

func (d *DocumentTemplateDefault) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	return nil
}
