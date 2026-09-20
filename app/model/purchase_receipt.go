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

// PurchaseReceipt ("Purchase Receipt") is a standalone proof-of-payment
// record — the AP mirror of SalesReceipt — no line items, no draft/confirm
// lifecycle; it can be edited and soft-deleted. It
// optionally references the PurchaseInvoice it's a receipt for, but does
// not update that invoice's paid/outstanding status — there's no
// partial-payment tracking in this scope.
type PurchaseReceipt struct {
	ID                string                       `gorm:"type:uuid;primaryKey"       json:"id"`
	CompanyID         string                       `gorm:"type:uuid;not null;index"   json:"company_id"`
	MitraID           string                       `gorm:"type:uuid;not null;index"   json:"mitra_id"`
	PurchaseInvoiceID *string                      `gorm:"type:uuid;index"            json:"purchase_invoice_id,omitempty"`
	Number            string                       `gorm:"type:varchar(50);not null"  json:"number"`
	Date              time.Time                    `gorm:"type:date;not null"         json:"date"`
	Amount            float64                      `gorm:"type:numeric(18,2);not null;default:0" json:"amount"`
	PaymentMethod     PurchaseReceiptPaymentMethod `gorm:"type:varchar(20);not null;default:'cash'" json:"payment_method"`
	BankAccountID     *string                      `gorm:"type:uuid;index"            json:"bank_account_id,omitempty"`
	Notes             string                       `gorm:"type:text"                  json:"notes,omitempty"`

	CreatedAt time.Time      `gorm:"autoCreateTime"   json:"created_at"`
	CreatedBy string         `gorm:"type:varchar(64)" json:"created_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"   json:"updated_at"`
	UpdatedBy string         `gorm:"type:varchar(64)" json:"updated_by,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index"            json:"-"`
}

func (PurchaseReceipt) TableName() string { return "purchase_receipts" }

func (r *PurchaseReceipt) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}
