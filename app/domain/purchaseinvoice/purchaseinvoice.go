// Package domain_purchaseinvoice — vendor billing documents ("Purchase
// Invoice" / "Bill"). Unlike Sales Invoice there's no Kind split — no
// down-payment variant exists on the purchase side (see
// app/model/purchase_invoice.go).
package domain_purchaseinvoice

import (
	"fmt"
	"time"

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

// CreateDTO — Number is optional; the service auto-generates one
// (BILL/YYYY/NNNN) when blank. PurchaseOrderID is optional, purely for
// traceability when the invoice was generated from a confirmed order.
type CreateDTO struct {
	CompanyID       string  `json:"-"`
	PurchaseOrderID *string `json:"purchase_order_id" validate:"omitempty,uuid4"`
	MitraID         string  `json:"mitra_id"          validate:"required,uuid4"`
	Number          string  `json:"number"            validate:"omitempty,max=50"`
	Date            string  `json:"date"              validate:"required"` // YYYY-MM-DD
	DueDate         string  `json:"due_date"          validate:"omitempty"`
	RefNo           string  `json:"ref_no"            validate:"omitempty,max=100"`
	Notes           string  `json:"notes"             validate:"omitempty"`

	AdditionalDiscountType  string  `json:"additional_discount_type"  validate:"omitempty,oneof=percent amount"`
	AdditionalDiscountValue float64 `json:"additional_discount_value" validate:"omitempty,gte=0"`
	ShippingCost            float64 `json:"shipping_cost"             validate:"omitempty,gte=0"`

	// Free-text meta, attachment, and signature/stamp-duty — see
	// app/model/purchase_invoice.go.
	ShipTo         string `json:"ship_to"         validate:"omitempty,max=150"`
	AttachmentData string `json:"attachment_data" validate:"omitempty"`
	AttachmentName string `json:"attachment_name" validate:"omitempty,max=255"`
	SignatureData  string `json:"signature_data"  validate:"omitempty"`
	StampDuty      bool   `json:"stamp_duty"      validate:"omitempty"`
	Template       string `json:"template" validate:"omitempty,oneof=template_1 template_2 template_3 template_4"`
	// ContactPersonID — a contact of THIS partner; the server copies its details onto the document.
	ContactPersonID *string `json:"contact_person_id" validate:"omitempty,uuid4"`

	Lines []LineDTO `json:"lines" validate:"required,min=1,dive"`
}

// UpdateDTO — a full replace: header fields + the complete new line set.
// Rejected outright once the invoice is Confirmed or Cancelled.
type UpdateDTO struct {
	PurchaseOrderID *string `json:"purchase_order_id" validate:"omitempty,uuid4"`
	MitraID         string  `json:"mitra_id"          validate:"required,uuid4"`
	Number          string  `json:"number"            validate:"omitempty,max=50"`
	Date            string  `json:"date"              validate:"required"`
	DueDate         string  `json:"due_date"          validate:"omitempty"`
	RefNo           string  `json:"ref_no"            validate:"omitempty,max=100"`
	Notes           string  `json:"notes"             validate:"omitempty"`

	AdditionalDiscountType  string  `json:"additional_discount_type"  validate:"omitempty,oneof=percent amount"`
	AdditionalDiscountValue float64 `json:"additional_discount_value" validate:"omitempty,gte=0"`
	ShippingCost            float64 `json:"shipping_cost"             validate:"omitempty,gte=0"`

	ShipTo         string `json:"ship_to"         validate:"omitempty,max=150"`
	AttachmentData string `json:"attachment_data" validate:"omitempty"`
	AttachmentName string `json:"attachment_name" validate:"omitempty,max=255"`
	SignatureData  string `json:"signature_data"  validate:"omitempty"`
	StampDuty      bool   `json:"stamp_duty"      validate:"omitempty"`
	Template       string `json:"template" validate:"omitempty,oneof=template_1 template_2 template_3 template_4"`
	// ContactPersonID — a contact of THIS partner; the server copies its details onto the document.
	ContactPersonID *string `json:"contact_person_id" validate:"omitempty,uuid4"`

	Lines []LineDTO `json:"lines" validate:"required,min=1,dive"`
}

// Filter drives the paginated list query.
type Filter struct {
	CompanyID string
	Search    string
	MitraID   string
	Status    string
	// PaymentStatus may list several values ("unpaid,partially_paid"); Overdue = confirmed, not fully
	// paid, due date before today.
	PaymentStatus string
	Overdue       bool
	Page          int
	PageSize      int
	Sort          string
	Order         string
	Fields        []string
}

type IRepository interface {
	Create(dto *CreateDTO, calc *utils.LinesCalc, actorID string) (*model.PurchaseInvoice, error)
	Update(companyID, id string, dto *UpdateDTO, calc *utils.LinesCalc, actorID string) (*model.PurchaseInvoice, error)
	FindByID(companyID, id string) (*model.PurchaseInvoice, error)
	FindAll(f *Filter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
	SetStatus(companyID, id, actorID string, status model.PurchaseInvoiceStatus) error
	SetTemplate(companyID, id, actorID, template string) error
	Summary(companyID string) (*Summary, error)
	MitraExists(companyID, mitraID string) (bool, error)
	TaxRates(companyID string, taxIDs []string) (map[string]utils.TaxRate, error)
	PreviewNumber(companyID string) (string, error)
	// CountAll / CountCreatedSince — the activation milestone and the Free-tier
	// transactions/month limit.
	CountAll(companyID string) (int64, error)
	CountCreatedSince(companyID string, since time.Time) (int64, error)
}

type IService interface {
	// Summary — the dashboard numbers (same rules as the list's payment / overdue filters).
	Summary(companyID string) (*Summary, error)
	// SetTemplate changes only the layout, in any status.
	SetTemplate(companyID, actorID, id, template string) (*model.PurchaseInvoice, error)
	Create(companyID, actorID string, dto *CreateDTO) (*model.PurchaseInvoice, error)
	Update(companyID, actorID, id string, dto *UpdateDTO) (*model.PurchaseInvoice, error)
	Get(companyID, id string) (*model.PurchaseInvoice, error)
	List(f *Filter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
	Confirm(companyID, actorID, id string) (*model.PurchaseInvoice, error)
	BackToDraft(companyID, actorID, id string) (*model.PurchaseInvoice, error)
	Cancel(companyID, actorID, id string) (*model.PurchaseInvoice, error)
	PreviewNumber(companyID string) (string, error)
}

type ErrNotFound struct{ ID string }

func (e *ErrNotFound) Error() string { return fmt.Sprintf("purchase invoice %s not found", e.ID) }

type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }

type ErrNumberExists struct{ Number string }

func (e *ErrNumberExists) Error() string {
	return fmt.Sprintf("invoice no. %s is already in use", e.Number)
}

// ErrNotEditable — the invoice isn't a draft, so it can't be edited/deleted.
type ErrNotEditable struct{}

func (e *ErrNotEditable) Error() string {
	return "a confirmed or cancelled purchase invoice can't be edited — move it back to draft first"
}

// ErrInvalidTransition — a status action doesn't apply from the current status.
type ErrInvalidTransition struct{ Message string }

func (e *ErrInvalidTransition) Error() string { return e.Message }

// SummaryFigure — a total and how many purchase invoices make it up.
type SummaryFigure struct {
	Amount float64 `json:"amount"`
	Count  int64   `json:"count"`
}

// Summary — the dashboard numbers for purchase invoices, the payables mirror of the sales summary:
//   - Outstanding: issued and not fully paid; amount = total - paid
//   - Overdue: outstanding AND past its due date (the same rule as the list's "overdue" filter)
//   - ThisMonth: issued this month, by invoice total
//   - Drafts: how many are still drafts
//
// Cancelled and deleted purchase invoices are never counted.
type Summary struct {
	Outstanding SummaryFigure `json:"outstanding"`
	Overdue     SummaryFigure `json:"overdue"`
	ThisMonth   SummaryFigure `json:"this_month"`
	Drafts      int64         `json:"drafts"`
}
