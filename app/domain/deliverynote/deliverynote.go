// Package domain_deliverynote — "Delivery Note" (Surat Jalan): a standalone
// physical-shipment log. No price/tax, no draft/confirm lifecycle. It can be
// edited and soft-deleted.
package domain_deliverynote

import (
	"fmt"

	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type LineDTO struct {
	ProductName string  `json:"product_name" validate:"required,max=255"`
	Description string  `json:"description"  validate:"omitempty,max=255"`
	Quantity    float64 `json:"quantity"      validate:"gt=0"`
	Unit        string  `json:"unit"          validate:"omitempty,max=20"`
}

// CreateDTO — Number is optional; the service auto-generates one
// (DN/YYYY/NNNN) when blank. SalesOrderID is optional traceability only —
// creating a delivery note does not track remaining quantity to ship
// against the order (no partial-fulfillment tracking exists here).
type CreateDTO struct {
	CompanyID      string  `json:"-"`
	MitraID        string  `json:"mitra_id"         validate:"required,uuid4"`
	SalesOrderID   *string `json:"sales_order_id"   validate:"omitempty,uuid4"`
	SalesInvoiceID *string `json:"sales_invoice_id" validate:"omitempty,uuid4"`
	Number         string  `json:"number"           validate:"omitempty,max=50"`
	Date           string  `json:"date"           validate:"required"` // YYYY-MM-DD
	Notes          string  `json:"notes"          validate:"omitempty"`

	// Optional shipping/logistics detail — none required.
	ShippingMethod string   `json:"shipping_method" validate:"omitempty,max=100"`
	TrackingNo     string   `json:"tracking_no"     validate:"omitempty,max=100"`
	VehicleNo      string   `json:"vehicle_no"      validate:"omitempty,max=100"`
	DriverName     string   `json:"driver_name"     validate:"omitempty,max=150"`
	TotalWeight    *float64 `json:"total_weight"    validate:"omitempty,gte=0"`

	// Optional supporting file — see app/model/delivery_note.go.
	AttachmentData string `json:"attachment_data" validate:"omitempty"`
	AttachmentName string `json:"attachment_name" validate:"omitempty,max=255"`

	Lines []LineDTO `json:"lines" validate:"required,min=1,dive"`
}

// UpdateDTO is the same shape as CreateDTO (a full replace of header and lines);
// a blank Number keeps the current one.
type UpdateDTO = CreateDTO

// Filter drives the paginated list query. SalesOrderID scopes the Related
// Documents sidebar on a sales invoice's detail page.
type Filter struct {
	CompanyID    string
	Search       string
	MitraID      string
	SalesOrderID string
	Page         int
	PageSize     int
	Sort         string
	Order        string
	Fields       []string
}

type IRepository interface {
	Create(dto *CreateDTO, actorID string) (*model.DeliveryNote, error)
	FindByID(companyID, id string) (*model.DeliveryNote, error)
	FindAll(f *Filter) (*utils.OffsetPaginationResult, error)
	Update(companyID, id string, dto *UpdateDTO, actorID string) (*model.DeliveryNote, error)
	Delete(companyID, id string) error
	MitraExists(companyID, mitraID string) (bool, error)
}

// IService — Update replaces the whole record; Delete is a soft delete.
type IService interface {
	Create(companyID, actorID string, dto *CreateDTO) (*model.DeliveryNote, error)
	Get(companyID, id string) (*model.DeliveryNote, error)
	List(f *Filter) (*utils.OffsetPaginationResult, error)
	Update(companyID, actorID, id string, dto *UpdateDTO) (*model.DeliveryNote, error)
	Delete(companyID, id string) error
}

type ErrNotFound struct{ ID string }

func (e *ErrNotFound) Error() string { return fmt.Sprintf("delivery note %s not found", e.ID) }

type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }

type ErrNumberExists struct{ Number string }

func (e *ErrNumberExists) Error() string {
	return fmt.Sprintf("delivery note no. %s is already in use", e.Number)
}
