package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SalesPaymentStatus — a payment starts pending (recorded, not yet
// verified) and moves to verified exactly once; there's no reject/cancel
// path (matches the seeded permissions: list/create/verify only).
type SalesPaymentStatus string

const (
	SalesPaymentStatusPending  SalesPaymentStatus = "pending"
	SalesPaymentStatusVerified SalesPaymentStatus = "verified"
)

// SalesPayment ("Pembayaran Masuk") — unlike SalesReceipt (a standalone
// proof-of-payment record with no invoice effect), a SalesPayment always
// references an invoice and, once verified, updates that invoice's
// PaidAmount/PaymentStatus. See SalesPaymentRepository.Verify for the
// balance recompute.
type SalesPayment struct {
	ID             string                    `gorm:"type:uuid;primaryKey"       json:"id"`
	CompanyID      string                    `gorm:"type:uuid;not null;index"   json:"company_id"`
	SalesInvoiceID string                    `gorm:"type:uuid;not null;index"   json:"sales_invoice_id"`
	MitraID        string                    `gorm:"type:uuid;not null;index"   json:"mitra_id"`
	Number         string                    `gorm:"type:varchar(50);not null"  json:"number"`
	Date           time.Time                 `gorm:"type:date;not null"         json:"date"`
	Amount         float64                   `gorm:"type:numeric(18,2);not null;default:0" json:"amount"`
	PaymentMethod  SalesReceiptPaymentMethod `gorm:"type:varchar(20);not null;default:'cash'" json:"payment_method"`
	BankAccountID  *string                   `gorm:"type:uuid;index"            json:"bank_account_id,omitempty"`
	RefNo          string                    `gorm:"type:varchar(100)"          json:"ref_no,omitempty"`
	Notes          string                    `gorm:"type:text"                  json:"notes,omitempty"`
	Status         SalesPaymentStatus        `gorm:"type:varchar(20);not null;default:'pending';index" json:"status"`
	VerifiedBy     string                    `gorm:"type:varchar(64)"           json:"verified_by,omitempty"`
	VerifiedAt     *time.Time                `json:"verified_at,omitempty"`

	CreatedAt time.Time      `gorm:"autoCreateTime"   json:"created_at"`
	CreatedBy string         `gorm:"type:varchar(64)" json:"created_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"   json:"updated_at"`
	UpdatedBy string         `gorm:"type:varchar(64)" json:"updated_by,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index"            json:"-"`
}

func (SalesPayment) TableName() string { return "sales_payments" }

func (p *SalesPayment) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	if p.Status == "" {
		p.Status = SalesPaymentStatusPending
	}
	return nil
}
