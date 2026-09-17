// Package domain_salesinvoice — billing documents. One domain covers both
// "Invoice Penjualan" (kind=invoice) and "Invoice Uang Muka" (kind=
// down_payment): they're structurally identical, differing only in number
// prefix and which nav page lists which (see app/model/sales_invoice.go).
package domain_salesinvoice

import (
	"fmt"

	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type LineDTO struct {
	ProductName   string   `json:"product_name"    validate:"required,max=255"`
	Description   string   `json:"description"     validate:"omitempty,max=255"`
	Quantity      float64  `json:"quantity"        validate:"gt=0"`
	UnitPrice     float64  `json:"unit_price"      validate:"gte=0"`
	DiscountType  string   `json:"discount_type"   validate:"omitempty,oneof=percent amount"`
	DiscountValue float64  `json:"discount_value"  validate:"gte=0"`
	TaxIDs        []string `json:"tax_ids"         validate:"omitempty,dive,uuid4"`
}

// CreateDTO — Number is optional; the service auto-generates one (prefix
// depends on Kind) when blank. SalesOrderID is optional, purely for
// traceability when the invoice was generated from a confirmed order.
type CreateDTO struct {
	CompanyID       string  `json:"-"`
	Kind            string  `json:"kind"              validate:"required,oneof=invoice down_payment"`
	SalesOrderID    *string `json:"sales_order_id"    validate:"omitempty,uuid4"`
	LinkedInvoiceID *string `json:"linked_invoice_id" validate:"omitempty,uuid4"`
	MitraID         string  `json:"mitra_id"          validate:"required,uuid4"`
	Number          string  `json:"number"         validate:"omitempty,max=50"`
	Date            string  `json:"date"           validate:"required"` // YYYY-MM-DD
	DueDate         string  `json:"due_date"       validate:"required"`
	RefNo           string  `json:"ref_no"         validate:"omitempty,max=100"`
	Notes           string  `json:"notes"          validate:"omitempty"`
	Terms           string  `json:"terms"          validate:"omitempty"`

	AdditionalDiscountType  string  `json:"additional_discount_type"  validate:"omitempty,oneof=percent amount"`
	AdditionalDiscountValue float64 `json:"additional_discount_value" validate:"omitempty,gte=0"`
	ShippingCost            float64 `json:"shipping_cost"             validate:"omitempty,gte=0"`

	// Free-text meta, attachment, and signature/stamp-duty — see
	// app/model/sales_invoice.go.
	ShipFrom       string `json:"ship_from"       validate:"omitempty,max=150"`
	Salesperson    string `json:"salesperson"     validate:"omitempty,max=120"`
	AttachmentData string `json:"attachment_data" validate:"omitempty"`
	AttachmentName string `json:"attachment_name" validate:"omitempty,max=255"`
	SignatureData  string `json:"signature_data"  validate:"omitempty"`
	StampDuty      bool   `json:"stamp_duty"      validate:"omitempty"`

	Lines []LineDTO `json:"lines" validate:"required,min=1,dive"`
}

// UpdateDTO — a full replace: header fields + the complete new line set.
// Rejected outright once the invoice is Confirmed or Cancelled. Kind isn't
// editable — an invoice doesn't switch between "invoice" and "down_payment".
type UpdateDTO struct {
	SalesOrderID    *string `json:"sales_order_id"    validate:"omitempty,uuid4"`
	LinkedInvoiceID *string `json:"linked_invoice_id" validate:"omitempty,uuid4"`
	MitraID         string  `json:"mitra_id"          validate:"required,uuid4"`
	Number          string  `json:"number"         validate:"omitempty,max=50"`
	Date            string  `json:"date"           validate:"required"`
	DueDate         string  `json:"due_date"       validate:"required"`
	RefNo           string  `json:"ref_no"         validate:"omitempty,max=100"`
	Notes           string  `json:"notes"          validate:"omitempty"`
	Terms           string  `json:"terms"          validate:"omitempty"`

	AdditionalDiscountType  string  `json:"additional_discount_type"  validate:"omitempty,oneof=percent amount"`
	AdditionalDiscountValue float64 `json:"additional_discount_value" validate:"omitempty,gte=0"`
	ShippingCost            float64 `json:"shipping_cost"             validate:"omitempty,gte=0"`

	ShipFrom       string `json:"ship_from"       validate:"omitempty,max=150"`
	Salesperson    string `json:"salesperson"     validate:"omitempty,max=120"`
	AttachmentData string `json:"attachment_data" validate:"omitempty"`
	AttachmentName string `json:"attachment_name" validate:"omitempty,max=255"`
	SignatureData  string `json:"signature_data"  validate:"omitempty"`
	StampDuty      bool   `json:"stamp_duty"      validate:"omitempty"`

	Lines []LineDTO `json:"lines" validate:"required,min=1,dive"`
}

// Filter drives the paginated list query. Kind scopes the list to whichever
// nav page is asking (Invoice Penjualan vs Invoice Uang Muka).
type Filter struct {
	CompanyID string
	Kind      string
	Search    string
	MitraID   string
	Status    string
	Page      int
	PageSize  int
	Sort      string
	Order     string
	Fields    []string
}

type IRepository interface {
	Create(dto *CreateDTO, calc *utils.LinesCalc, actorID string) (*model.SalesInvoice, error)
	Update(companyID, id string, dto *UpdateDTO, calc *utils.LinesCalc, actorID string) (*model.SalesInvoice, error)
	FindByID(companyID, id string) (*model.SalesInvoice, error)
	FindAll(f *Filter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
	SetStatus(companyID, id, actorID string, status model.SalesInvoiceStatus) error
	MitraExists(companyID, mitraID string) (bool, error)
	TaxRates(companyID string, taxIDs []string) (map[string]utils.TaxRate, error)
	// PreviewNumber returns what the next auto-generated number would be for
	// this kind right now — a preview for the Add page, not a reservation.
	PreviewNumber(companyID, kind string) (string, error)
}

type IService interface {
	Create(companyID, actorID string, dto *CreateDTO) (*model.SalesInvoice, error)
	Update(companyID, actorID, id string, dto *UpdateDTO) (*model.SalesInvoice, error)
	Get(companyID, id string) (*model.SalesInvoice, error)
	List(f *Filter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
	Confirm(companyID, actorID, id string) (*model.SalesInvoice, error)
	BackToDraft(companyID, actorID, id string) (*model.SalesInvoice, error)
	Cancel(companyID, actorID, id string) (*model.SalesInvoice, error)
	PreviewNumber(companyID, kind string) (string, error)
}

type ErrNotFound struct{ ID string }

func (e *ErrNotFound) Error() string { return fmt.Sprintf("invoice %s not found", e.ID) }

type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }

type ErrNumberExists struct{ Number string }

func (e *ErrNumberExists) Error() string {
	return fmt.Sprintf("invoice no. %s is already in use", e.Number)
}

// ErrNotEditable — the invoice isn't a draft, so it can't be edited/deleted.
type ErrNotEditable struct{}

func (e *ErrNotEditable) Error() string {
	return "a confirmed or cancelled invoice can't be edited — move it back to draft first"
}

// ErrInvalidTransition — a status action doesn't apply from the current status.
type ErrInvalidTransition struct{ Message string }

func (e *ErrInvalidTransition) Error() string { return e.Message }
