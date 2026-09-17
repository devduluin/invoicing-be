package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SalesInvoiceKind — one domain/table covers both "Invoice Penjualan" and
// "Invoice Uang Muka" (down payment): they're structurally identical (mitra,
// optional sales-order reference, line items, tax calc, draft/confirmed/
// cancelled lifecycle), differing only in number prefix and which nav page
// lists which. See app/domain/salesinvoice for the shared CRUD.
type SalesInvoiceKind string

const (
	SalesInvoiceKindInvoice     SalesInvoiceKind = "invoice"
	SalesInvoiceKindDownPayment SalesInvoiceKind = "down_payment"
)

func IsValidSalesInvoiceKind(v string) bool {
	switch SalesInvoiceKind(v) {
	case SalesInvoiceKindInvoice, SalesInvoiceKindDownPayment:
		return true
	default:
		return false
	}
}

// SalesInvoiceStatus mirrors SalesOrderStatus's shape exactly.
type SalesInvoiceStatus string

const (
	SalesInvoiceStatusDraft     SalesInvoiceStatus = "draft"
	SalesInvoiceStatusConfirmed SalesInvoiceStatus = "confirmed"
	SalesInvoiceStatusCancelled SalesInvoiceStatus = "cancelled"
)

// SalesInvoicePaymentStatus — derived from PaidAmount vs GrandTotal,
// recomputed only by SalesPaymentRepository.Verify (never set through the
// invoice's own Create/Update DTOs).
type SalesInvoicePaymentStatus string

const (
	SalesInvoicePaymentUnpaid        SalesInvoicePaymentStatus = "unpaid"
	SalesInvoicePaymentPartiallyPaid SalesInvoicePaymentStatus = "partially_paid"
	SalesInvoicePaymentPaid          SalesInvoicePaymentStatus = "paid"
)

// SalesInvoice — a billing document, optionally traced back to the Sales
// Order it was generated from. Does not touch accounts/journal entries.
type SalesInvoice struct {
	ID           string  `gorm:"type:uuid;primaryKey"       json:"id"`
	CompanyID    string  `gorm:"type:uuid;not null;index"   json:"company_id"`
	SalesOrderID *string `gorm:"type:uuid;index"            json:"sales_order_id,omitempty"`
	// LinkedInvoiceID — for a down-payment invoice (Kind=down_payment),
	// the regular invoice it's a down payment against. Purely a reference,
	// like SalesOrderID — no automatic balance/amount coupling.
	LinkedInvoiceID *string            `gorm:"type:uuid;index"            json:"linked_invoice_id,omitempty"`
	MitraID         string             `gorm:"type:uuid;not null;index"   json:"mitra_id"`
	Kind            SalesInvoiceKind   `gorm:"type:varchar(20);not null;index" json:"kind"`
	Number          string             `gorm:"type:varchar(50);not null"  json:"number"`
	Date            time.Time          `gorm:"type:date;not null"         json:"date"`
	DueDate         *time.Time         `gorm:"type:date"                  json:"due_date,omitempty"`
	RefNo           string             `gorm:"type:varchar(100)"          json:"ref_no,omitempty"`
	Notes           string             `gorm:"type:text"                  json:"notes,omitempty"`
	Terms           string             `gorm:"type:text"                  json:"terms,omitempty"`
	Status          SalesInvoiceStatus `gorm:"type:varchar(20);not null;default:'draft';index" json:"status"`
	Subtotal        float64            `gorm:"type:numeric(18,2);not null;default:0" json:"subtotal"`
	DiscountTotal   float64            `gorm:"type:numeric(18,2);not null;default:0" json:"discount_total"`
	TaxTotal        float64            `gorm:"type:numeric(18,2);not null;default:0" json:"tax_total"`
	GrandTotal      float64            `gorm:"type:numeric(18,2);not null;default:0" json:"grand_total"`

	// Document-level discount applied on top of the line totals, in
	// addition to any per-line discount — see utils.CalcLines.
	AdditionalDiscountType   string  `gorm:"type:varchar(10);not null;default:'percent'" json:"additional_discount_type"`
	AdditionalDiscountValue  float64 `gorm:"type:numeric(18,2);not null;default:0" json:"additional_discount_value"`
	AdditionalDiscountAmount float64 `gorm:"type:numeric(18,2);not null;default:0" json:"additional_discount_amount"`

	// Flat shipping/delivery fee added on top of the total — untaxed, added
	// after the additional discount (see sales_invoice.service.go's calc()).
	ShippingCost float64 `gorm:"type:numeric(18,2);not null;default:0" json:"shipping_cost"`

	// Recomputed only by SalesPaymentRepository.Verify — never written
	// through this invoice's own Create/Update DTOs.
	PaidAmount    float64                   `gorm:"type:numeric(18,2);not null;default:0" json:"paid_amount"`
	PaymentStatus SalesInvoicePaymentStatus `gorm:"type:varchar(20);not null;default:'unpaid';index" json:"payment_status"`

	// Free-text document meta — no Warehouse/Member entity behind these,
	// deliberately simple fields.
	ShipFrom    string `gorm:"type:varchar(150)" json:"ship_from,omitempty"`
	Salesperson string `gorm:"type:varchar(120)" json:"salesperson,omitempty"`

	// Optional supporting file, stored as a data: URI (same convention as
	// CompanyService.processLogo / LogoUpload) — never selected in list
	// queries, only fetched on FindByID.
	AttachmentData string `gorm:"type:text"         json:"attachment_data,omitempty"`
	AttachmentName string `gorm:"type:varchar(255)" json:"attachment_name,omitempty"`

	// Optional signature (data: URI) + stamp duty flag — the only document
	// with a print template today (InvoiceDocument.tsx), so this is the one
	// place it's actually rendered, not just captured.
	SignatureData string `gorm:"type:text"                  json:"signature_data,omitempty"`
	StampDuty     bool   `gorm:"not null;default:false"     json:"stamp_duty"`

	Lines []SalesInvoiceLine `gorm:"foreignKey:SalesInvoiceID" json:"lines,omitempty"`

	CreatedAt time.Time      `gorm:"autoCreateTime"   json:"created_at"`
	CreatedBy string         `gorm:"type:varchar(64)" json:"created_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"   json:"updated_at"`
	UpdatedBy string         `gorm:"type:varchar(64)" json:"updated_by,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index"            json:"-"`
}

func (SalesInvoice) TableName() string { return "sales_invoices" }

func (s *SalesInvoice) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	if s.Status == "" {
		s.Status = SalesInvoiceStatusDraft
	}
	if s.PaymentStatus == "" {
		s.PaymentStatus = SalesInvoicePaymentUnpaid
	}
	return nil
}

// SalesInvoiceLine — identical shape to SalesOrderLine (free-text product,
// no catalog).
type SalesInvoiceLine struct {
	ID             string  `gorm:"type:uuid;primaryKey"       json:"id"`
	SalesInvoiceID string  `gorm:"type:uuid;not null;index"   json:"sales_invoice_id"`
	CompanyID      string  `gorm:"type:uuid;not null;index"   json:"company_id"`
	ProductName    string  `gorm:"type:varchar(255);not null" json:"product_name"`
	Description    string  `gorm:"type:varchar(255)"          json:"description,omitempty"`
	Quantity       float64 `gorm:"type:numeric(18,4);not null;default:0" json:"quantity"`
	UnitPrice      float64 `gorm:"type:numeric(18,2);not null;default:0" json:"unit_price"`
	DiscountType   string  `gorm:"type:varchar(10);not null;default:'percent'"          json:"discount_type"`
	DiscountValue  float64 `gorm:"column:discount_percent;type:numeric(18,2);not null;default:0" json:"discount_value"`
	// TaxIDs is populated manually by the repository from SalesInvoiceLineTax
	// join rows — never read/written by GORM directly.
	TaxIDs        []string `gorm:"-" json:"tax_ids,omitempty"`
	LineSubtotal  float64  `gorm:"type:numeric(18,2);not null;default:0" json:"line_subtotal"`
	LineTaxAmount float64  `gorm:"type:numeric(18,2);not null;default:0" json:"line_tax_amount"`
	LineTotal     float64  `gorm:"type:numeric(18,2);not null;default:0" json:"line_total"`
	LineOrder     int      `gorm:"not null;default:0"         json:"line_order"`
}

func (SalesInvoiceLine) TableName() string { return "sales_invoice_lines" }

func (l *SalesInvoiceLine) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return nil
}
