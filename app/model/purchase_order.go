package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PurchaseOrderStatus mirrors SalesOrderStatus's draft/confirmed/cancelled shape.
type PurchaseOrderStatus string

const (
	PurchaseOrderStatusDraft     PurchaseOrderStatus = "draft"
	PurchaseOrderStatusConfirmed PurchaseOrderStatus = "confirmed"
	PurchaseOrderStatusCancelled PurchaseOrderStatus = "cancelled"
)

// PurchaseOrder is a pre-bill purchase document — supplier, line items,
// totals. It does not touch accounts/journal entries; that happens later
// when a Purchase Invoice ("Bill") reads a confirmed order.
type PurchaseOrder struct {
	ID            string              `gorm:"type:uuid;primaryKey"       json:"id"`
	CompanyID     string              `gorm:"type:uuid;not null;index"   json:"company_id"`
	MitraID       string              `gorm:"type:uuid;not null;index"   json:"mitra_id"`
	Number        string              `gorm:"type:varchar(50);not null"  json:"number"`
	Date          time.Time           `gorm:"type:date;not null"         json:"date"`
	RefNo         string              `gorm:"type:varchar(100)"          json:"ref_no,omitempty"`
	Notes         string              `gorm:"type:text"                  json:"notes,omitempty"`
	Status        PurchaseOrderStatus `gorm:"type:varchar(20);not null;default:'draft';index" json:"status"`
	Subtotal      float64             `gorm:"type:numeric(18,2);not null;default:0" json:"subtotal"`
	DiscountTotal float64             `gorm:"type:numeric(18,2);not null;default:0" json:"discount_total"`
	TaxTotal      float64             `gorm:"type:numeric(18,2);not null;default:0" json:"tax_total"`
	GrandTotal    float64             `gorm:"type:numeric(18,2);not null;default:0" json:"grand_total"`

	// Document-level discount applied on top of the line totals, in
	// addition to any per-line discount — see utils.CalcLines.
	AdditionalDiscountType   string  `gorm:"type:varchar(10);not null;default:'percent'" json:"additional_discount_type"`
	AdditionalDiscountValue  float64 `gorm:"type:numeric(18,2);not null;default:0" json:"additional_discount_value"`
	AdditionalDiscountAmount float64 `gorm:"type:numeric(18,2);not null;default:0" json:"additional_discount_amount"`

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
	// Purchase Order yet, so this is stored but not rendered anywhere today.
	SignatureData string `gorm:"type:text"                  json:"signature_data,omitempty"`
	StampDuty     bool   `gorm:"not null;default:false"     json:"stamp_duty"`

	// Template is the printable layout (template_1..4). New documents start from the company default.
	Template string `gorm:"type:varchar(20);not null;default:'template_1'" json:"template"`

	// Contact person the document was made for. The id is a reference; the four contact_* columns
	// are a COPY taken when the document is saved, so an edited or deleted contact never changes it.
	ContactPersonID *string `gorm:"type:uuid;index"          json:"contact_person_id,omitempty"`
	ContactName     string  `gorm:"type:varchar(255)"        json:"contact_name,omitempty"`
	ContactPosition string  `gorm:"type:varchar(150)"        json:"contact_position,omitempty"`
	ContactPhone    string  `gorm:"type:varchar(50)"         json:"contact_phone,omitempty"`
	ContactEmail    string  `gorm:"type:varchar(150)"        json:"contact_email,omitempty"`

	Lines []PurchaseOrderLine `gorm:"foreignKey:PurchaseOrderID" json:"lines,omitempty"`

	CreatedAt time.Time      `gorm:"autoCreateTime"   json:"created_at"`
	CreatedBy string         `gorm:"type:varchar(64)" json:"created_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"   json:"updated_at"`
	UpdatedBy string         `gorm:"type:varchar(64)" json:"updated_by,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index"            json:"-"`
}

func (PurchaseOrder) TableName() string { return "purchase_orders" }

func (s *PurchaseOrder) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	if s.Status == "" {
		s.Status = PurchaseOrderStatusDraft
	}
	return nil
}

// PurchaseOrderLine — a free-text line item (no product catalog). A line can
// carry any number of taxes (see PurchaseOrderLineTax) summed by calc
// method; LineTaxAmount is the combined result computed at write time.
type PurchaseOrderLine struct {
	ID              string  `gorm:"type:uuid;primaryKey"       json:"id"`
	PurchaseOrderID string  `gorm:"type:uuid;not null;index"   json:"purchase_order_id"`
	CompanyID       string  `gorm:"type:uuid;not null;index"   json:"company_id"`
	ProductName     string  `gorm:"type:varchar(255);not null" json:"product_name"`
	Description     string  `gorm:"type:varchar(255)"          json:"description,omitempty"`
	Quantity        float64 `gorm:"type:numeric(18,4);not null;default:0" json:"quantity"`
	UnitPrice       float64 `gorm:"type:numeric(18,2);not null;default:0" json:"unit_price"`
	DiscountType    string  `gorm:"type:varchar(10);not null;default:'percent'"          json:"discount_type"`
	DiscountValue   float64 `gorm:"column:discount_percent;type:numeric(18,2);not null;default:0" json:"discount_value"`
	// TaxIDs is populated manually by the repository from PurchaseOrderLineTax
	// join rows — never read/written by GORM directly.
	TaxIDs        []string `gorm:"-" json:"tax_ids,omitempty"`
	LineSubtotal  float64  `gorm:"type:numeric(18,2);not null;default:0" json:"line_subtotal"`
	LineTaxAmount float64  `gorm:"type:numeric(18,2);not null;default:0" json:"line_tax_amount"`
	LineTotal     float64  `gorm:"type:numeric(18,2);not null;default:0" json:"line_total"`
	LineOrder     int      `gorm:"not null;default:0"         json:"line_order"`
}

func (PurchaseOrderLine) TableName() string { return "purchase_order_lines" }

func (l *PurchaseOrderLine) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return nil
}
