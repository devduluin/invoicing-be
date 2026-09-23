package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SalesOrderStatus mirrors JournalEntryStatus's draft/posted shape, with a
// third terminal state a sales order actually needs: a deal can fall through.
type SalesOrderStatus string

const (
	SalesOrderStatusDraft     SalesOrderStatus = "draft"
	SalesOrderStatusConfirmed SalesOrderStatus = "confirmed"
	SalesOrderStatusCancelled SalesOrderStatus = "cancelled"
)

// SalesOrder is a pre-invoice sales document — customer, line items, totals.
// It does not touch accounts/journal entries; that happens later when a
// sales invoice module reads a confirmed order (not built yet).
type SalesOrder struct {
	ID            string           `gorm:"type:uuid;primaryKey"       json:"id"`
	CompanyID     string           `gorm:"type:uuid;not null;index"   json:"company_id"`
	MitraID       string           `gorm:"type:uuid;not null;index"   json:"mitra_id"`
	Number        string           `gorm:"type:varchar(50);not null"  json:"number"`
	Date          time.Time        `gorm:"type:date;not null"         json:"date"`
	RefNo         string           `gorm:"type:varchar(100)"          json:"ref_no,omitempty"`
	Notes         string           `gorm:"type:text"                  json:"notes,omitempty"`
	Status        SalesOrderStatus `gorm:"type:varchar(20);not null;default:'draft';index" json:"status"`
	Subtotal      float64          `gorm:"type:numeric(18,2);not null;default:0" json:"subtotal"`
	DiscountTotal float64          `gorm:"type:numeric(18,2);not null;default:0" json:"discount_total"`
	TaxTotal      float64          `gorm:"type:numeric(18,2);not null;default:0" json:"tax_total"`
	GrandTotal    float64          `gorm:"type:numeric(18,2);not null;default:0" json:"grand_total"`

	// Document-level discount applied on top of the line totals, in
	// addition to any per-line discount — see utils.CalcLines.
	AdditionalDiscountType   string  `gorm:"type:varchar(10);not null;default:'percent'" json:"additional_discount_type"`
	AdditionalDiscountValue  float64 `gorm:"type:numeric(18,2);not null;default:0" json:"additional_discount_value"`
	AdditionalDiscountAmount float64 `gorm:"type:numeric(18,2);not null;default:0" json:"additional_discount_amount"`

	// Free-text document meta — no Warehouse/Member entity behind these,
	// deliberately simple fields.
	ShipFrom    string `gorm:"type:varchar(150)" json:"ship_from,omitempty"`
	Salesperson string `gorm:"type:varchar(120)" json:"salesperson,omitempty"`

	// Optional supporting file, stored as a data: URI (same convention as
	// CompanyService.processLogo / LogoUpload) — never selected in list
	// queries, only fetched on FindByID.
	AttachmentData string `gorm:"type:text"         json:"attachment_data,omitempty"`
	AttachmentName string `gorm:"type:varchar(255)" json:"attachment_name,omitempty"`

	// Optional signature (data: URI) + stamp duty flag, captured for layout
	// consistency across all 4 documents — actually rendered on a print
	// template only where one exists today (Sales Invoice).
	SignatureData string `gorm:"type:text"                  json:"signature_data,omitempty"`
	StampDuty     bool   `gorm:"not null;default:false"     json:"stamp_duty"`

	// Template is the printable layout (template_1..7). New documents start from the company default.
	Template string `gorm:"type:varchar(20);not null;default:'template_1'" json:"template"`

	// Contact person the document was made for. The id is a reference; the four contact_* columns
	// are a COPY taken when the document is saved, so an edited or deleted contact never changes it.
	ContactPersonID *string `gorm:"type:uuid;index"          json:"contact_person_id,omitempty"`
	ContactName     string  `gorm:"type:varchar(255)"        json:"contact_name,omitempty"`
	ContactPosition string  `gorm:"type:varchar(150)"        json:"contact_position,omitempty"`
	ContactPhone    string  `gorm:"type:varchar(50)"         json:"contact_phone,omitempty"`
	ContactEmail    string  `gorm:"type:varchar(150)"        json:"contact_email,omitempty"`

	Lines []SalesOrderLine `gorm:"foreignKey:SalesOrderID" json:"lines,omitempty"`

	CreatedAt time.Time      `gorm:"autoCreateTime"   json:"created_at"`
	CreatedBy string         `gorm:"type:varchar(64)" json:"created_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"   json:"updated_at"`
	UpdatedBy string         `gorm:"type:varchar(64)" json:"updated_by,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index"            json:"-"`
}

func (SalesOrder) TableName() string { return "sales_orders" }

func (s *SalesOrder) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	if s.Status == "" {
		s.Status = SalesOrderStatusDraft
	}
	return nil
}

// SalesOrderLine — a free-text line item (no product catalog: invoice-service
// has no Product/Inventory model). A line can carry any number of taxes
// (see SalesOrderLineTax) summed by calc method; LineTaxAmount is the
// combined result computed at write time.
type SalesOrderLine struct {
	ID            string  `gorm:"type:uuid;primaryKey"       json:"id"`
	SalesOrderID  string  `gorm:"type:uuid;not null;index"   json:"sales_order_id"`
	CompanyID     string  `gorm:"type:uuid;not null;index"   json:"company_id"`
	ProductName   string  `gorm:"type:varchar(255);not null" json:"product_name"`
	Description   string  `gorm:"type:varchar(255)"          json:"description,omitempty"`
	Quantity      float64 `gorm:"type:numeric(18,4);not null;default:0" json:"quantity"`
	UnitPrice     float64 `gorm:"type:numeric(18,2);not null;default:0" json:"unit_price"`
	DiscountType  string  `gorm:"type:varchar(10);not null;default:'percent'"          json:"discount_type"`
	DiscountValue float64 `gorm:"column:discount_percent;type:numeric(18,2);not null;default:0" json:"discount_value"`
	// TaxIDs is populated manually by the repository from SalesOrderLineTax
	// join rows — never read/written by GORM directly.
	TaxIDs        []string `gorm:"-" json:"tax_ids,omitempty"`
	LineSubtotal  float64  `gorm:"type:numeric(18,2);not null;default:0" json:"line_subtotal"`
	LineTaxAmount float64  `gorm:"type:numeric(18,2);not null;default:0" json:"line_tax_amount"`
	LineTotal     float64  `gorm:"type:numeric(18,2);not null;default:0" json:"line_total"`
	LineOrder     int      `gorm:"not null;default:0"         json:"line_order"`
}

func (SalesOrderLine) TableName() string { return "sales_order_lines" }

func (l *SalesOrderLine) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return nil
}
