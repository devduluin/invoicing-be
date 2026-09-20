package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DeliveryNote ("Surat Jalan") is a standalone physical-shipment log — no
// price/tax, no draft/confirm lifecycle; it can be edited and soft-deleted.
// It optionally references the SalesOrder it
// fulfills, purely for traceability and to power the frontend's
// "pre-fill from order" convenience — no quantity-remaining tracking
// across multiple notes against one order.
type DeliveryNote struct {
	ID           string  `gorm:"type:uuid;primaryKey"       json:"id"`
	CompanyID    string  `gorm:"type:uuid;not null;index"   json:"company_id"`
	MitraID      string  `gorm:"type:uuid;not null;index"   json:"mitra_id"`
	SalesOrderID *string `gorm:"type:uuid;index"            json:"sales_order_id,omitempty"`
	// SalesInvoiceID — the same free-form traceability as SalesOrderID, for
	// a delivery note raised from an already-billed invoice instead of an
	// order. Either, both, or neither may be set.
	SalesInvoiceID *string   `gorm:"type:uuid;index"            json:"sales_invoice_id,omitempty"`
	Number         string    `gorm:"type:varchar(50);not null"  json:"number"`
	Date           time.Time `gorm:"type:date;not null"         json:"date"`
	Notes          string    `gorm:"type:text"                  json:"notes,omitempty"`

	// Optional shipping/logistics detail — each surfaced behind its own
	// checkbox on the frontend ("More Information"); none are required.
	ShippingMethod string   `gorm:"type:varchar(100)"    json:"shipping_method,omitempty"`
	TrackingNo     string   `gorm:"type:varchar(100)"    json:"tracking_no,omitempty"`
	VehicleNo      string   `gorm:"type:varchar(100)"    json:"vehicle_no,omitempty"`
	DriverName     string   `gorm:"type:varchar(150)"    json:"driver_name,omitempty"`
	TotalWeight    *float64 `gorm:"type:numeric(18,3)"   json:"total_weight,omitempty"`

	// Optional supporting file, stored as a data: URI (same convention as
	// CompanyService.processLogo / LogoUpload) — never selected in list
	// queries, only fetched on FindByID.
	AttachmentData string `gorm:"type:text"         json:"attachment_data,omitempty"`
	AttachmentName string `gorm:"type:varchar(255)" json:"attachment_name,omitempty"`

	Lines []DeliveryNoteLine `gorm:"foreignKey:DeliveryNoteID" json:"lines,omitempty"`

	CreatedAt time.Time      `gorm:"autoCreateTime"   json:"created_at"`
	CreatedBy string         `gorm:"type:varchar(64)" json:"created_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"   json:"updated_at"`
	UpdatedBy string         `gorm:"type:varchar(64)" json:"updated_by,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index"            json:"-"`
}

func (DeliveryNote) TableName() string { return "delivery_notes" }

func (n *DeliveryNote) BeforeCreate(tx *gorm.DB) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	return nil
}

// DeliveryNoteLine — a free-text line item (no product catalog, no price/
// tax: this is a packing list, not a billing document).
type DeliveryNoteLine struct {
	ID             string  `gorm:"type:uuid;primaryKey"       json:"id"`
	DeliveryNoteID string  `gorm:"type:uuid;not null;index"   json:"delivery_note_id"`
	CompanyID      string  `gorm:"type:uuid;not null;index"   json:"company_id"`
	ProductName    string  `gorm:"type:varchar(255);not null" json:"product_name"`
	Description    string  `gorm:"type:varchar(255)"          json:"description,omitempty"`
	Quantity       float64 `gorm:"type:numeric(18,4);not null;default:0" json:"quantity"`
	Unit           string  `gorm:"type:varchar(20)"           json:"unit,omitempty"`
	LineOrder      int     `gorm:"not null;default:0"         json:"line_order"`
}

func (DeliveryNoteLine) TableName() string { return "delivery_note_lines" }

func (l *DeliveryNoteLine) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return nil
}
