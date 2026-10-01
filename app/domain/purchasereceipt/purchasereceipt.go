// Package domain_purchasereceipt — "Purchase Receipt": a standalone
// proof-of-payment record, the AP mirror of Sales Receipt. No line items, no
// draft/confirm lifecycle. It can be edited and soft-deleted.
package domain_purchasereceipt

import (
	"fmt"

	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// AllocationDTO — one "Kepada Invoice" row: how much of this receipt goes to which purchase
// invoice. Each allocation updates that invoice's PaidAmount/PaymentStatus.
type AllocationDTO struct {
	PurchaseInvoiceID string  `json:"purchase_invoice_id" validate:"required,uuid4"`
	Amount            float64 `json:"amount"              validate:"gt=0"`
}

// CreateDTO — Number is optional; the service auto-generates one (PKW/YYYY/NNNN) when blank.
// Amount isn't client-supplied — it's the sum of Allocations, computed by the repository. A
// receipt must allocate to at least one invoice.
type CreateDTO struct {
	CompanyID      string          `json:"-"`
	MitraID        string          `json:"mitra_id"        validate:"required,uuid4"`
	Number         string          `json:"number"          validate:"omitempty,max=50"`
	Date           string          `json:"date"            validate:"required"` // YYYY-MM-DD
	PaymentMethod  string          `json:"payment_method"  validate:"required,oneof=cash transfer other"`
	BankAccountID  *string         `json:"bank_account_id" validate:"omitempty,uuid4"`
	Notes          string          `json:"notes"           validate:"omitempty"`
	AttachmentData string          `json:"attachment_data" validate:"omitempty"`
	AttachmentName string          `json:"attachment_name" validate:"omitempty,max=255"`
	SignatureData  string          `json:"signature_data"  validate:"omitempty"`
	Allocations    []AllocationDTO `json:"allocations"     validate:"required,min=1,dive"`
}

// UpdateDTO is the same shape as CreateDTO (full replace); a blank Number keeps the current one.
type UpdateDTO = CreateDTO

// Filter drives the paginated list query. PurchaseInvoiceID scopes to receipts with an allocation
// against that invoice — the detail page's payment history.
type Filter struct {
	CompanyID         string
	Search            string
	MitraID           string
	PurchaseInvoiceID string
	Page              int
	PageSize          int
	Sort              string
	Order             string
	Fields            []string
	// WithDetails — ?with=details: also load the lines / contact persons (exports).
	WithDetails bool
}

type IRepository interface {
	Create(dto *CreateDTO, actorID string) (*model.PurchaseReceipt, error)
	FindByID(companyID, id string) (*model.PurchaseReceipt, error)
	FindAll(f *Filter) (*utils.OffsetPaginationResult, error)
	MitraExists(companyID, mitraID string) (bool, error)
	PreviewNumber(companyID string) (string, error)
	Update(companyID, id string, dto *UpdateDTO, actorID string) (*model.PurchaseReceipt, error)
	Delete(companyID, id string) error
}

// IService — Update replaces the receipt; Delete is a soft delete.
type IService interface {
	Create(companyID, actorID string, dto *CreateDTO) (*model.PurchaseReceipt, error)
	Get(companyID, id string) (*model.PurchaseReceipt, error)
	List(f *Filter) (*utils.OffsetPaginationResult, error)
	PreviewNumber(companyID string) (string, error)
	Update(companyID, actorID, id string, dto *UpdateDTO) (*model.PurchaseReceipt, error)
	Delete(companyID, id string) error
}

type ErrNotFound struct{ ID string }

func (e *ErrNotFound) Error() string { return fmt.Sprintf("receipt %s not found", e.ID) }

type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }

type ErrNumberExists struct{ Number string }

func (e *ErrNumberExists) Error() string {
	return fmt.Sprintf("receipt no. %s is already in use", e.Number)
}

// ErrExceedsBalance — the payment would push the linked invoice's PaidAmount past its GrandTotal.
type ErrExceedsBalance struct{ Message string }

func (e *ErrExceedsBalance) Error() string { return e.Message }
