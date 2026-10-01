package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PurchaseReceiptPaymentMethod — how the payment behind this receipt was made.
type PurchaseReceiptPaymentMethod string

const (
	PurchaseReceiptPaymentCash     PurchaseReceiptPaymentMethod = "cash"
	PurchaseReceiptPaymentTransfer PurchaseReceiptPaymentMethod = "transfer"
	PurchaseReceiptPaymentOther    PurchaseReceiptPaymentMethod = "other"
)

func IsValidPurchaseReceiptPaymentMethod(v string) bool {
	switch PurchaseReceiptPaymentMethod(v) {
	case PurchaseReceiptPaymentCash, PurchaseReceiptPaymentTransfer, PurchaseReceiptPaymentOther:
		return true
	default:
		return false
	}
}

// PurchaseReceipt ("Purchase Receipt") is a standalone proof-of-payment record — the AP mirror of
// SalesReceipt — no draft/confirm lifecycle; it can be edited and soft-deleted (both reverse the
// allocations on the invoices first). It allocates its payment across one or more purchase
// invoices via Allocations; each allocation updates that invoice's PaidAmount/PaymentStatus (see
// PurchaseReceiptRepository.Create / applyPurchaseInvoicePayment).
type PurchaseReceipt struct {
	ID        string `gorm:"type:uuid;primaryKey"       json:"id"`
	CompanyID string `gorm:"type:uuid;not null;index"   json:"company_id"`
	MitraID   string `gorm:"type:uuid;not null;index"   json:"mitra_id"`
	// Mitra — preloaded on lists only (partner code + name); never migrated as a FK constraint.
	Mitra  *Mitra    `gorm:"foreignKey:MitraID;-:migration" json:"mitra,omitempty"`
	Number string    `gorm:"type:varchar(50);not null"  json:"number"`
	Date   time.Time `gorm:"type:date;not null"         json:"date"`
	// Amount — the receipt's total, computed once at create time as the sum of Allocations'
	// amounts (materialized for fast list display, same convention as SalesReceipt.Amount).
	Amount        float64                      `gorm:"type:numeric(18,2);not null;default:0" json:"amount"`
	PaymentMethod PurchaseReceiptPaymentMethod `gorm:"type:varchar(20);not null;default:'cash'" json:"payment_method"`
	BankAccountID *string                      `gorm:"type:uuid;index"            json:"bank_account_id,omitempty"`
	Notes         string                       `gorm:"type:text"                  json:"notes,omitempty"`

	// Optional supporting file, stored as a data: URI (same convention as sales_order.go) until
	// uploaded to MinIO by the controller.
	AttachmentData string `gorm:"type:text"         json:"attachment_data,omitempty"`
	AttachmentName string `gorm:"type:varchar(255)" json:"attachment_name,omitempty"`
	// Optional signature captured on this receipt, same convention as sales_order.go.
	SignatureData string `gorm:"type:text" json:"signature_data,omitempty"`

	Allocations []PurchaseReceiptAllocation `gorm:"foreignKey:PurchaseReceiptID" json:"allocations,omitempty"`

	CreatedAt time.Time      `gorm:"autoCreateTime"   json:"created_at"`
	CreatedBy string         `gorm:"type:varchar(64)" json:"created_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"   json:"updated_at"`
	UpdatedBy string         `gorm:"type:varchar(64)" json:"updated_by,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index"            json:"-"`
	// Relations preloaded by the list endpoint (shown as "<fk>_rel"); never migrated as FK constraints.
	MitraRel       *Mitra       `gorm:"foreignKey:MitraID;-:migration" json:"mitra_id_rel,omitempty"`
	BankAccountRel *BankAccount `gorm:"foreignKey:BankAccountID;-:migration" json:"bank_account_id_rel,omitempty"`
}

func (PurchaseReceipt) TableName() string { return "purchase_receipts" }

func (r *PurchaseReceipt) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

// PurchaseReceiptAllocation — one row of "Kepada Invoice": how much of this receipt's total goes
// to which purchase invoice. The AP mirror of SalesReceiptAllocation.
type PurchaseReceiptAllocation struct {
	ID                string  `gorm:"type:uuid;primaryKey"     json:"id"`
	PurchaseReceiptID string  `gorm:"type:uuid;not null;index" json:"purchase_receipt_id"`
	PurchaseInvoiceID string  `gorm:"type:uuid;not null;index" json:"purchase_invoice_id"`
	CompanyID         string  `gorm:"type:uuid;not null;index" json:"company_id"`
	Amount            float64 `gorm:"type:numeric(18,2);not null;default:0" json:"amount"`
	// PurchaseInvoiceRel — the invoice number, for exports; never migrated as a FK constraint.
	PurchaseInvoiceRel *PurchaseInvoice `gorm:"foreignKey:PurchaseInvoiceID;-:migration" json:"purchase_invoice_id_rel,omitempty"`
}

func (PurchaseReceiptAllocation) TableName() string { return "purchase_receipt_allocations" }

func (a *PurchaseReceiptAllocation) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}
