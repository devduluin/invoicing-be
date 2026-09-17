// Package domain_goodsreceipt — "Goods Receipt" (Penerimaan Barang): a
// standalone physical-receiving log, the AP mirror of Delivery Note. No
// price/tax, no draft/confirm lifecycle — its seeded permissions
// (invoice-goods-receipt-list/create only, no update/delete) say a goods
// receipt is create-once, never edited or deleted through the API.
package domain_goodsreceipt

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
// (GR/YYYY/NNNN) when blank. PurchaseOrderID is optional traceability
// only — creating a goods receipt does not track remaining quantity to
// receive against the order (no partial-fulfillment tracking exists here).
type CreateDTO struct {
	CompanyID       string  `json:"-"`
	MitraID         string  `json:"mitra_id"          validate:"required,uuid4"`
	PurchaseOrderID *string `json:"purchase_order_id" validate:"omitempty,uuid4"`
	Number          string  `json:"number"            validate:"omitempty,max=50"`
	Date            string  `json:"date"              validate:"required"` // YYYY-MM-DD
	Notes           string  `json:"notes"             validate:"omitempty"`

	// Optional shipping/logistics detail — none required.
	ShippingMethod string   `json:"shipping_method" validate:"omitempty,max=100"`
	TrackingNo     string   `json:"tracking_no"     validate:"omitempty,max=100"`
	VehicleNo      string   `json:"vehicle_no"      validate:"omitempty,max=100"`
	DriverName     string   `json:"driver_name"     validate:"omitempty,max=150"`
	TotalWeight    *float64 `json:"total_weight"    validate:"omitempty,gte=0"`

	// Optional supporting file — see app/model/goods_receipt.go.
	AttachmentData string `json:"attachment_data" validate:"omitempty"`
	AttachmentName string `json:"attachment_name" validate:"omitempty,max=255"`

	Lines []LineDTO `json:"lines" validate:"required,min=1,dive"`
}

// Filter drives the paginated list query.
type Filter struct {
	CompanyID string
	Search    string
	MitraID   string
	Page      int
	PageSize  int
	Sort      string
	Order     string
	Fields    []string
}

type IRepository interface {
	Create(dto *CreateDTO, actorID string) (*model.GoodsReceipt, error)
	FindByID(companyID, id string) (*model.GoodsReceipt, error)
	FindAll(f *Filter) (*utils.OffsetPaginationResult, error)
	MitraExists(companyID, mitraID string) (bool, error)
}

// IService — no Update/Delete, matching the seeded permissions exactly.
type IService interface {
	Create(companyID, actorID string, dto *CreateDTO) (*model.GoodsReceipt, error)
	Get(companyID, id string) (*model.GoodsReceipt, error)
	List(f *Filter) (*utils.OffsetPaginationResult, error)
}

type ErrNotFound struct{ ID string }

func (e *ErrNotFound) Error() string { return fmt.Sprintf("goods receipt %s not found", e.ID) }

type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }

type ErrNumberExists struct{ Number string }

func (e *ErrNumberExists) Error() string {
	return fmt.Sprintf("goods receipt no. %s is already in use", e.Number)
}
