package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SalesReceiptPaymentMethod — how the payment behind this receipt arrived.
type SalesReceiptPaymentMethod string

const (
	SalesReceiptPaymentCash     SalesReceiptPaymentMethod = "cash"
	SalesReceiptPaymentTransfer SalesReceiptPaymentMethod = "transfer"
	SalesReceiptPaymentOther    SalesReceiptPaymentMethod = "other"
)

func IsValidSalesReceiptPaymentMethod(v string) bool {
	switch SalesReceiptPaymentMethod(v) {
	case SalesReceiptPaymentCash, SalesReceiptPaymentTransfer, SalesReceiptPaymentOther:
		return true
	default:
		return false
	}
}

// SalesReceipt ("Kuitansi Penjualan") is a proof-of-payment record — no
// draft/confirm lifecycle; it can be edited and soft-deleted (both reverse
// the allocations on the invoices first). It allocates its payment across one or more invoices via
// Allocations; each allocation updates that invoice's PaidAmount/
// PaymentStatus (see SalesReceiptRepository.Create /
// repository.applySalesInvoicePayment) — the same balance fields
// SalesPaymentRepository.Verify maintains, via the same shared, row-locked
// primitive so the two paths can never double-count.
type SalesReceipt struct {
	ID        string `gorm:"type:uuid;primaryKey"       json:"id"`
	CompanyID string `gorm:"type:uuid;not null;index"   json:"company_id"`
	MitraID   string `gorm:"type:uuid;not null;index"   json:"mitra_id"`
	// Mitra — preloaded on lists only (partner code + name); never migrated as a FK constraint.
	Mitra  *Mitra    `gorm:"foreignKey:MitraID;-:migration" json:"mitra,omitempty"`
	Number string    `gorm:"type:varchar(50);not null"  json:"number"`
	Date   time.Time `gorm:"type:date;not null"         json:"date"`
	// Amount — the receipt's total, computed once at create time as the
	// sum of Allocations' amounts (materialized for fast list display,
	// same convention as SalesInvoice.GrandTotal). Never entered directly.
	Amount        float64                   `gorm:"type:numeric(18,2);not null;default:0" json:"amount"`
	PaymentMethod SalesReceiptPaymentMethod `gorm:"type:varchar(20);not null;default:'cash'" json:"payment_method"`
	BankAccountID *string                   `gorm:"type:uuid;index"            json:"bank_account_id,omitempty"`
	Notes         string                    `gorm:"type:text"                  json:"notes,omitempty"`

	// Optional supporting file, stored as a data: URI (same convention as sales_order.go) until
	// uploaded to MinIO by the controller.
	AttachmentData string `gorm:"type:text"         json:"attachment_data,omitempty"`
	AttachmentName string `gorm:"type:varchar(255)" json:"attachment_name,omitempty"`
	// Optional signature captured on this receipt, same convention as sales_order.go.
	SignatureData string `gorm:"type:text" json:"signature_data,omitempty"`

	Allocations []SalesReceiptAllocation `gorm:"foreignKey:SalesReceiptID" json:"allocations,omitempty"`

	CreatedAt time.Time      `gorm:"autoCreateTime"   json:"created_at"`
	CreatedBy string         `gorm:"type:varchar(64)" json:"created_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"   json:"updated_at"`
	UpdatedBy string         `gorm:"type:varchar(64)" json:"updated_by,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index"            json:"-"`
	// Relations preloaded by the list endpoint (shown as "<fk>_rel"); never migrated as FK constraints.
	MitraRel       *Mitra       `gorm:"foreignKey:MitraID;-:migration" json:"mitra_id_rel,omitempty"`
	BankAccountRel *BankAccount `gorm:"foreignKey:BankAccountID;-:migration" json:"bank_account_id_rel,omitempty"`
}

func (SalesReceipt) TableName() string { return "sales_receipts" }

func (r *SalesReceipt) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

// SalesReceiptAllocation — one row of "Kepada Invoice": how much of this
// receipt's total goes to which invoice.
type SalesReceiptAllocation struct {
	ID             string  `gorm:"type:uuid;primaryKey"     json:"id"`
	SalesReceiptID string  `gorm:"type:uuid;not null;index" json:"sales_receipt_id"`
	SalesInvoiceID string  `gorm:"type:uuid;not null;index" json:"sales_invoice_id"`
	CompanyID      string  `gorm:"type:uuid;not null;index" json:"company_id"`
	Amount         float64 `gorm:"type:numeric(18,2);not null;default:0" json:"amount"`
	// SalesInvoiceRel — the invoice number, for exports; never migrated as a FK constraint.
	SalesInvoiceRel *SalesInvoice `gorm:"foreignKey:SalesInvoiceID;-:migration" json:"sales_invoice_id_rel,omitempty"`
}

func (SalesReceiptAllocation) TableName() string { return "sales_receipt_allocations" }

func (a *SalesReceiptAllocation) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}
