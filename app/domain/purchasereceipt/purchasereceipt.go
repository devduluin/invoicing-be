// Package domain_purchasereceipt — "Purchase Receipt": a standalone
// proof-of-payment record, the AP mirror of Sales Receipt. No line items, no
// draft/confirm lifecycle. It can be edited and soft-deleted.
package domain_purchasereceipt

import (
	"fmt"

	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// CreateDTO — Number is optional; the service auto-generates one
// (PKW/YYYY/NNNN) when blank. PurchaseInvoiceID is optional traceability
// only — creating a receipt does not update the invoice's paid/outstanding
// status (no partial-payment tracking exists here).
type CreateDTO struct {
	CompanyID         string  `json:"-"`
	MitraID           string  `json:"mitra_id"            validate:"required,uuid4"`
	PurchaseInvoiceID *string `json:"purchase_invoice_id" validate:"omitempty,uuid4"`
	Number            string  `json:"number"              validate:"omitempty,max=50"`
	Date              string  `json:"date"                validate:"required"` // YYYY-MM-DD
	Amount            float64 `json:"amount"              validate:"gt=0"`
	PaymentMethod     string  `json:"payment_method"      validate:"required,oneof=cash transfer other"`
	BankAccountID     *string `json:"bank_account_id"     validate:"omitempty,uuid4"`
	Notes             string  `json:"notes"               validate:"omitempty"`
}

// UpdateDTO is the same shape as CreateDTO (full replace); a blank Number keeps the current one.
type UpdateDTO = CreateDTO

// Filter drives the paginated list query.
type Filter struct {
	CompanyID string
	Search    string
	MitraID   string
	// PurchaseInvoiceID scopes to the payments applied to one bill.
	PurchaseInvoiceID string
	Page              int
	PageSize          int
	Sort              string
	Order             string
	Fields            []string
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
