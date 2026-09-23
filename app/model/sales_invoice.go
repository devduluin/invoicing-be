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

// SalesInvoiceTemplate — which of the seven printable layouts this invoice uses
// (create/edit preview, detail page and the PDF all render the same one).
// Stored per invoice; presentation only, never affects amounts.
type SalesInvoiceTemplate string

const (
	SalesInvoiceTemplate1 SalesInvoiceTemplate = "template_1"
	SalesInvoiceTemplate2 SalesInvoiceTemplate = "template_2"
	SalesInvoiceTemplate3 SalesInvoiceTemplate = "template_3"
	SalesInvoiceTemplate4 SalesInvoiceTemplate = "template_4"
	SalesInvoiceTemplate5 SalesInvoiceTemplate = "template_5"
	SalesInvoiceTemplate6 SalesInvoiceTemplate = "template_6"
	SalesInvoiceTemplate7 SalesInvoiceTemplate = "template_7"

	DefaultSalesInvoiceTemplate = SalesInvoiceTemplate1
)

func IsValidSalesInvoiceTemplate(v string) bool {
	switch SalesInvoiceTemplate(v) {
	case SalesInvoiceTemplate1, SalesInvoiceTemplate2, SalesInvoiceTemplate3, SalesInvoiceTemplate4,
		SalesInvoiceTemplate5, SalesInvoiceTemplate6, SalesInvoiceTemplate7:
		return true
	default:
		return false
	}
}

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
	LinkedInvoiceID *string          `gorm:"type:uuid;index"            json:"linked_invoice_id,omitempty"`
	MitraID         string           `gorm:"type:uuid;not null;index"   json:"mitra_id"`
	Kind            SalesInvoiceKind `gorm:"type:varchar(20);not null;index" json:"kind"`
	Number          string           `gorm:"type:varchar(50);not null"  json:"number"`
	Date            time.Time        `gorm:"type:date;not null"         json:"date"`
	DueDate         *time.Time       `gorm:"type:date"                  json:"due_date,omitempty"`
	RefNo           string           `gorm:"type:varchar(100)"          json:"ref_no,omitempty"`
	Notes           string           `gorm:"type:text"                  json:"notes,omitempty"`
	Terms           string           `gorm:"type:text"                  json:"terms,omitempty"`
	// Template — layout choice, see SalesInvoiceTemplate. Existing rows pick up
	// the column default (template_1) on migration.
	Template string `gorm:"type:varchar(20);not null;default:'template_1'" json:"template"`

	// Contact person the document was made for. The id is a reference; the four contact_* columns
	// are a COPY taken when the document is saved, so an edited or deleted contact never changes it.
	ContactPersonID *string            `gorm:"type:uuid;index"          json:"contact_person_id,omitempty"`
	ContactName     string             `gorm:"type:varchar(255)"        json:"contact_name,omitempty"`
	ContactPosition string             `gorm:"type:varchar(150)"        json:"contact_position,omitempty"`
	ContactPhone    string             `gorm:"type:varchar(50)"         json:"contact_phone,omitempty"`
	ContactEmail    string             `gorm:"type:varchar(150)"        json:"contact_email,omitempty"`
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
	PaidAmount float64 `gorm:"type:numeric(18,2);not null;default:0" json:"paid_amount"`
	// OutstandingAmount = max(GrandTotal - PaidAmount, 0). Computed on every read, never stored.
	OutstandingAmount float64                   `gorm:"-" json:"outstanding_amount"`
	PaymentStatus     SalesInvoicePaymentStatus `gorm:"type:varchar(20);not null;default:'unpaid';index" json:"payment_status"`

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

// AfterFind fills the derived outstanding amount on every read (single, list, after create/update).
func (s *SalesInvoice) AfterFind(tx *gorm.DB) error {
	s.OutstandingAmount = outstandingOf(s.GrandTotal, s.PaidAmount)
	return nil
}

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

// Outstanding returns what is still owed: max(total - paid, 0). Never negative, whatever the
// stored numbers are. Derived, so it can never disagree with PaidAmount / GrandTotal.
func outstandingOf(total, paid float64) float64 {
	if v := total - paid; v > 0 {
		return float64(int64(v*100+0.5)) / 100
	}
	return 0
}
