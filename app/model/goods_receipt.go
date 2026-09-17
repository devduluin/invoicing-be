package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GoodsReceipt ("Penerimaan Barang") is a standalone physical-receiving log
// — the AP mirror of DeliveryNote. No price/tax, no draft/confirm lifecycle
// (matches its seeded permissions: invoice-goods-receipt-list/create only,
// no update/delete — a goods receipt is create-once). It optionally
// references the PurchaseOrder it fulfills, purely for traceability and to
// power the frontend's "pre-fill from order" convenience — no quantity-
// remaining tracking across multiple receipts against one order.
type GoodsReceipt struct {
	ID              string    `gorm:"type:uuid;primaryKey"       json:"id"`
	CompanyID       string    `gorm:"type:uuid;not null;index"   json:"company_id"`
	MitraID         string    `gorm:"type:uuid;not null;index"   json:"mitra_id"`
	PurchaseOrderID *string   `gorm:"type:uuid;index"            json:"purchase_order_id,omitempty"`
	Number          string    `gorm:"type:varchar(50);not null"  json:"number"`
	Date            time.Time `gorm:"type:date;not null"         json:"date"`
	Notes           string    `gorm:"type:text"                  json:"notes,omitempty"`

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

	Lines []GoodsReceiptLine `gorm:"foreignKey:GoodsReceiptID" json:"lines,omitempty"`

	CreatedAt time.Time      `gorm:"autoCreateTime"   json:"created_at"`
	CreatedBy string         `gorm:"type:varchar(64)" json:"created_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"   json:"updated_at"`
	UpdatedBy string         `gorm:"type:varchar(64)" json:"updated_by,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index"            json:"-"`
}

func (GoodsReceipt) TableName() string { return "goods_receipts" }

func (r *GoodsReceipt) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

// GoodsReceiptLine — a free-text line item (no product catalog, no price/
// tax: this is a receiving log, not a billing document).
type GoodsReceiptLine struct {
	ID             string  `gorm:"type:uuid;primaryKey"       json:"id"`
	GoodsReceiptID string  `gorm:"type:uuid;not null;index"   json:"goods_receipt_id"`
	CompanyID      string  `gorm:"type:uuid;not null;index"   json:"company_id"`
	ProductName    string  `gorm:"type:varchar(255);not null" json:"product_name"`
	Description    string  `gorm:"type:varchar(255)"          json:"description,omitempty"`
	Quantity       float64 `gorm:"type:numeric(18,4);not null;default:0" json:"quantity"`
	Unit           string  `gorm:"type:varchar(20)"           json:"unit,omitempty"`
	LineOrder      int     `gorm:"not null;default:0"         json:"line_order"`
}

func (GoodsReceiptLine) TableName() string { return "goods_receipt_lines" }

func (l *GoodsReceiptLine) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return nil
}
