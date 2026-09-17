// Package domain_salespayment — "Sales Payment" ("Pembayaran Masuk"): a
// payment recorded against a specific, confirmed Sales Invoice, that once
// verified updates that invoice's PaidAmount/PaymentStatus. No Update/
// Delete — matches the seeded permissions exactly (invoice-sales-payment-
// {list,create,verify}).
package domain_salespayment

import (
	"fmt"

	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// CreateDTO — Number is optional; the repository auto-generates one
// (PAY/YYYY/NNNN) when blank. MitraID is derived server-side from the
// invoice, not client-supplied.
type CreateDTO struct {
	CompanyID      string  `json:"-"`
	MitraID        string  `json:"-"`
	SalesInvoiceID string  `json:"sales_invoice_id" validate:"required,uuid4"`
	Number         string  `json:"number"           validate:"omitempty,max=50"`
	Date           string  `json:"date"             validate:"required"` // YYYY-MM-DD
	Amount         float64 `json:"amount"           validate:"gt=0"`
	PaymentMethod  string  `json:"payment_method"   validate:"required,oneof=cash transfer other"`
	BankAccountID  *string `json:"bank_account_id"  validate:"omitempty,uuid4"`
	RefNo          string  `json:"ref_no"           validate:"omitempty,max=100"`
	Notes          string  `json:"notes"            validate:"omitempty"`
}

// Filter drives the paginated list query. SalesInvoiceID scopes the "All
// Payments" tab on a single invoice's detail page.
type Filter struct {
	CompanyID      string
	SalesInvoiceID string
	MitraID        string
	Status         string
	Search         string
	Page           int
	PageSize       int
	Sort           string
	Order          string
	Fields         []string
}

type IRepository interface {
	Create(dto *CreateDTO, actorID string) (*model.SalesPayment, error)
	FindByID(companyID, id string) (*model.SalesPayment, error)
	FindAll(f *Filter) (*utils.OffsetPaginationResult, error)
	Verify(companyID, actorID, id string) (*model.SalesPayment, error)
	InvoiceForPayment(companyID, invoiceID string) (*model.SalesInvoice, error)
}

// IService — no Update/Delete, matching the seeded permissions exactly.
type IService interface {
	Create(companyID, actorID string, dto *CreateDTO) (*model.SalesPayment, error)
	Get(companyID, id string) (*model.SalesPayment, error)
	List(f *Filter) (*utils.OffsetPaginationResult, error)
	Verify(companyID, actorID, id string) (*model.SalesPayment, error)
}

type ErrNotFound struct{ ID string }

func (e *ErrNotFound) Error() string { return fmt.Sprintf("payment %s not found", e.ID) }

type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }

type ErrNumberExists struct{ Number string }

func (e *ErrNumberExists) Error() string {
	return fmt.Sprintf("payment no. %s is already in use", e.Number)
}

// ErrInvalidTransition — verifying a payment that isn't pending.
type ErrInvalidTransition struct{ Message string }

func (e *ErrInvalidTransition) Error() string { return e.Message }

// ErrExceedsBalance — the payment amount would push PaidAmount past
// GrandTotal.
type ErrExceedsBalance struct{ Message string }

func (e *ErrExceedsBalance) Error() string { return e.Message }
