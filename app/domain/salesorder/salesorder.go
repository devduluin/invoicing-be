// Package domain_salesorder — pre-invoice sales documents ("Order Penjualan").
// Line items are free-text (no product catalog — invoice-service has no
// Product/Inventory model), scoped the same way Journal Entry was: a
// transactional record with a draft lifecycle, not master data.
package domain_salesorder

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

// CreateDTO — Number is optional; the service auto-generates one (SO/YYYY/NNNN)
// when blank.
type CreateDTO struct {
	CompanyID string `json:"-"`
	MitraID   string `json:"mitra_id" validate:"required,uuid4"`
	Number    string `json:"number"   validate:"omitempty,max=50"`
	Date      string `json:"date"     validate:"required"` // YYYY-MM-DD
	RefNo     string `json:"ref_no"   validate:"omitempty,max=100"`
	Notes     string `json:"notes"    validate:"omitempty"`

	// Document-level discount, on top of any per-line discount.
	AdditionalDiscountType  string  `json:"additional_discount_type"  validate:"omitempty,oneof=percent amount"`
	AdditionalDiscountValue float64 `json:"additional_discount_value" validate:"omitempty,gte=0"`

	// Free-text meta, attachment, and signature/stamp-duty — see
	// app/model/sales_order.go.
	ShipFrom       string `json:"ship_from"       validate:"omitempty,max=150"`
	Salesperson    string `json:"salesperson"     validate:"omitempty,max=120"`
	AttachmentData string `json:"attachment_data" validate:"omitempty"`
	AttachmentName string `json:"attachment_name" validate:"omitempty,max=255"`
	SignatureData  string `json:"signature_data"  validate:"omitempty"`
	StampDuty      bool   `json:"stamp_duty"      validate:"omitempty"`
	Template       string `json:"template" validate:"omitempty,oneof=template_1 template_2 template_3 template_4 template_5 template_6 template_7"`
	// ContactPersonID — a contact of THIS partner; the server copies its details onto the document.
	ContactPersonID *string `json:"contact_person_id" validate:"omitempty,uuid4"`

	Lines []LineDTO `json:"lines" validate:"required,min=1,dive"`
}

// UpdateDTO — a full replace: header fields + the complete new line set.
// Rejected outright once the order is Confirmed or Cancelled.
type UpdateDTO struct {
	MitraID string `json:"mitra_id" validate:"required,uuid4"`
	Number  string `json:"number"   validate:"omitempty,max=50"`
	Date    string `json:"date"     validate:"required"`
	RefNo   string `json:"ref_no"   validate:"omitempty,max=100"`
	Notes   string `json:"notes"    validate:"omitempty"`

	AdditionalDiscountType  string  `json:"additional_discount_type"  validate:"omitempty,oneof=percent amount"`
	AdditionalDiscountValue float64 `json:"additional_discount_value" validate:"omitempty,gte=0"`

	ShipFrom       string `json:"ship_from"       validate:"omitempty,max=150"`
	Salesperson    string `json:"salesperson"     validate:"omitempty,max=120"`
	AttachmentData string `json:"attachment_data" validate:"omitempty"`
	AttachmentName string `json:"attachment_name" validate:"omitempty,max=255"`
	SignatureData  string `json:"signature_data"  validate:"omitempty"`
	StampDuty      bool   `json:"stamp_duty"      validate:"omitempty"`
	Template       string `json:"template" validate:"omitempty,oneof=template_1 template_2 template_3 template_4 template_5 template_6 template_7"`
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
	Page      int
	PageSize  int
	Sort      string
	Order     string
	Fields    []string
}

// TaxInfo — the two fields calcLines needs from a referenced tax.
type TaxInfo struct {
	Rate       float64
	CalcMethod string // "exclusive" | "inclusive"
}

// LineCalc / OrderCalc — calcLines' output (app/service/sales_order.service.go).
// The service owns the tax-aware math (it's the one that fetches TaxRates);
// the repository just persists whatever it's handed.
type LineCalc struct {
	ProductName   string
	Description   string
	Quantity      float64
	UnitPrice     float64
	DiscountType  string
	DiscountValue float64
	TaxIDs        []string
	LineSubtotal  float64
	LineTaxAmount float64
	LineTotal     float64
}

type OrderCalc struct {
	Lines         []LineCalc
	Subtotal      float64
	DiscountTotal float64
	TaxTotal      float64
	GrandTotal    float64

	AdditionalDiscountType   string
	AdditionalDiscountValue  float64
	AdditionalDiscountAmount float64
}

type IRepository interface {
	Create(dto *CreateDTO, calc *OrderCalc, actorID string) (*model.SalesOrder, error)
	Update(companyID, id string, dto *UpdateDTO, calc *OrderCalc, actorID string) (*model.SalesOrder, error)
	FindByID(companyID, id string) (*model.SalesOrder, error)
	FindAll(f *Filter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
	SetStatus(companyID, id, actorID string, status model.SalesOrderStatus) error
	SetTemplate(companyID, id, actorID, template string) error
	MitraExists(companyID, mitraID string) (bool, error)
	// TaxRates resolves each given tax id to its rate/calc_method in one query.
	TaxRates(companyID string, taxIDs []string) (map[string]TaxInfo, error)
	// PreviewNumber returns what the next auto-generated number would be
	// right now — a best-effort preview for the Add page, not a reservation
	// (Create re-checks/re-generates at write time).
	PreviewNumber(companyID string) (string, error)
	// CountCreatedSince — the Free-tier transactions/month limit.
	CountCreatedSince(companyID string, since time.Time) (int64, error)
}

type IService interface {
	// SetTemplate changes only the layout, in any status.
	SetTemplate(companyID, actorID, id, template string) (*model.SalesOrder, error)
	Create(companyID, actorID string, dto *CreateDTO) (*model.SalesOrder, error)
	Update(companyID, actorID, id string, dto *UpdateDTO) (*model.SalesOrder, error)
	Get(companyID, id string) (*model.SalesOrder, error)
	List(f *Filter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
	Confirm(companyID, actorID, id string) (*model.SalesOrder, error)
	BackToDraft(companyID, actorID, id string) (*model.SalesOrder, error)
	Cancel(companyID, actorID, id string) (*model.SalesOrder, error)
	PreviewNumber(companyID string) (string, error)
}

type ErrNotFound struct{ ID string }

func (e *ErrNotFound) Error() string { return fmt.Sprintf("sales order %s not found", e.ID) }

type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }

type ErrNumberExists struct{ Number string }

func (e *ErrNumberExists) Error() string {
	return fmt.Sprintf("order no. %s is already in use", e.Number)
}

// ErrNotEditable — the order isn't a draft, so it can't be edited/deleted.
type ErrNotEditable struct{}

func (e *ErrNotEditable) Error() string {
	return "a confirmed or cancelled sales order can't be edited — move it back to draft first"
}

// ErrInvalidTransition — a status action doesn't apply from the current status.
type ErrInvalidTransition struct{ Message string }

func (e *ErrInvalidTransition) Error() string { return e.Message }
