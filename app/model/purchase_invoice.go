package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PurchaseInvoiceStatus mirrors PurchaseOrderStatus's shape exactly.
type PurchaseInvoiceStatus string

const (
	PurchaseInvoiceStatusDraft     PurchaseInvoiceStatus = "draft"
	PurchaseInvoiceStatusConfirmed PurchaseInvoiceStatus = "confirmed"
	PurchaseInvoiceStatusCancelled PurchaseInvoiceStatus = "cancelled"
)

// PurchaseInvoice — a vendor's bill, optionally traced back to the Purchase
// Order it was generated from. Product-facing label is "Purchase Invoice";
// gated by the SSO-seeded invoice-bill-* permissions (standard AP term for
// this document — see the Purchase Order/Invoice/Receipt build plan).
// Unlike SalesInvoice there's no Kind split — no down-payment variant exists
// on the purchase side. Does not touch accounts/journal entries or track
// paid/outstanding amounts.
type PurchaseInvoice struct {
	ID              string                `gorm:"type:uuid;primaryKey"       json:"id"`
	CompanyID       string                `gorm:"type:uuid;not null;index"   json:"company_id"`
	PurchaseOrderID *string               `gorm:"type:uuid;index"            json:"purchase_order_id,omitempty"`
	MitraID         string                `gorm:"type:uuid;not null;index"   json:"mitra_id"`
	Number          string                `gorm:"type:varchar(50);not null"  json:"number"`
	Date            time.Time             `gorm:"type:date;not null"         json:"date"`
	DueDate         *time.Time            `gorm:"type:date"                  json:"due_date,omitempty"`
	RefNo           string                `gorm:"type:varchar(100)"          json:"ref_no,omitempty"`
	Notes           string                `gorm:"type:text"                  json:"notes,omitempty"`
	Status          PurchaseInvoiceStatus `gorm:"type:varchar(20);not null;default:'draft';index" json:"status"`
	Subtotal        float64               `gorm:"type:numeric(18,2);not null;default:0" json:"subtotal"`
	DiscountTotal   float64               `gorm:"type:numeric(18,2);not null;default:0" json:"discount_total"`
	TaxTotal        float64               `gorm:"type:numeric(18,2);not null;default:0" json:"tax_total"`
	GrandTotal      float64               `gorm:"type:numeric(18,2);not null;default:0" json:"grand_total"`

	// Document-level discount applied on top of the line totals, in
	// addition to any per-line discount — see utils.CalcLines.
	AdditionalDiscountType   string  `gorm:"type:varchar(10);not null;default:'percent'" json:"additional_discount_type"`
	AdditionalDiscountValue  float64 `gorm:"type:numeric(18,2);not null;default:0" json:"additional_discount_value"`
	AdditionalDiscountAmount float64 `gorm:"type:numeric(18,2);not null;default:0" json:"additional_discount_amount"`

	// Flat shipping/delivery fee added on top of the total — untaxed, added
	// after the additional discount (see purchase_invoice.service.go's calc()).
	ShippingCost float64 `gorm:"type:numeric(18,2);not null;default:0" json:"shipping_cost"`

	// Free-text document meta — no Warehouse entity behind this, a
	// deliberately simple field. No Salesperson analog on the purchase side.
	ShipTo string `gorm:"type:varchar(150)" json:"ship_to,omitempty"`

	// Optional supporting file, stored as a data: URI (same convention as
	// CompanyService.processLogo / LogoUpload) — never selected in list
	// queries, only fetched on FindByID.
	AttachmentData string `gorm:"type:text"         json:"attachment_data,omitempty"`
	AttachmentName string `gorm:"type:varchar(255)" json:"attachment_name,omitempty"`

	// Optional signature (data: URI) + stamp duty flag, captured for layout
	// consistency across all 4 documents — no print template exists for
	// Purchase Invoice yet, so this is stored but not rendered anywhere today.
	SignatureData string `gorm:"type:text"                  json:"signature_data,omitempty"`
	StampDuty     bool   `gorm:"not null;default:false"     json:"stamp_duty"`

	Lines []PurchaseInvoiceLine `gorm:"foreignKey:PurchaseInvoiceID" json:"lines,omitempty"`

	CreatedAt time.Time      `gorm:"autoCreateTime"   json:"created_at"`
	CreatedBy string         `gorm:"type:varchar(64)" json:"created_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"   json:"updated_at"`
	UpdatedBy string         `gorm:"type:varchar(64)" json:"updated_by,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index"            json:"-"`
}

func (PurchaseInvoice) TableName() string { return "purchase_invoices" }

func (s *PurchaseInvoice) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	if s.Status == "" {
		s.Status = PurchaseInvoiceStatusDraft
	}
	return nil
}

// PurchaseInvoiceLine — identical shape to PurchaseOrderLine (free-text
// product, no catalog).
type PurchaseInvoiceLine struct {
	ID                string  `gorm:"type:uuid;primaryKey"       json:"id"`
	PurchaseInvoiceID string  `gorm:"type:uuid;not null;index"   json:"purchase_invoice_id"`
	CompanyID         string  `gorm:"type:uuid;not null;index"   json:"company_id"`
	ProductName       string  `gorm:"type:varchar(255);not null" json:"product_name"`
	Description       string  `gorm:"type:varchar(255)"          json:"description,omitempty"`
	Quantity          float64 `gorm:"type:numeric(18,4);not null;default:0" json:"quantity"`
	UnitPrice         float64 `gorm:"type:numeric(18,2);not null;default:0" json:"unit_price"`
	DiscountType      string  `gorm:"type:varchar(10);not null;default:'percent'"          json:"discount_type"`
	DiscountValue     float64 `gorm:"column:discount_percent;type:numeric(18,2);not null;default:0" json:"discount_value"`
	// TaxIDs is populated manually by the repository from
	// PurchaseInvoiceLineTax join rows — never read/written by GORM directly.
	TaxIDs        []string `gorm:"-" json:"tax_ids,omitempty"`
	LineSubtotal  float64  `gorm:"type:numeric(18,2);not null;default:0" json:"line_subtotal"`
	LineTaxAmount float64  `gorm:"type:numeric(18,2);not null;default:0" json:"line_tax_amount"`
	LineTotal     float64  `gorm:"type:numeric(18,2);not null;default:0" json:"line_total"`
	LineOrder     int      `gorm:"not null;default:0"         json:"line_order"`
}

func (PurchaseInvoiceLine) TableName() string { return "purchase_invoice_lines" }

func (l *PurchaseInvoiceLine) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return nil
}
