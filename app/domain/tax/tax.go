// Package domain_tax — configurable taxes master data (PRD §9).
package domain_tax

import (
	"fmt"

	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// CreateDTO covers both a single tax and a compound tax (IsCompound). The
// service enforces which fields are required for each shape — go-playground
// validate can't express "required unless".
type CreateDTO struct {
	CompanyID  string `json:"-"`
	Name       string `json:"name"        validate:"required,max=120"`
	IsCompound bool   `json:"is_compound"`
	IsActive   *bool  `json:"is_active"`

	// Single-tax fields.
	Kind              string  `json:"kind"                validate:"omitempty,oneof=ppn pph other"`
	CalcMethod        string  `json:"calc_method"         validate:"omitempty,oneof=exclusive inclusive"`
	Rate              float64 `json:"rate"                validate:"gte=0,lte=100"`
	SalesAccountID    string  `json:"sales_account_id"    validate:"omitempty,max=64"`
	PurchaseAccountID string  `json:"purchase_account_id" validate:"omitempty,max=64"`

	// Compound-tax fields.
	Component1ID string `json:"component1_id" validate:"omitempty,max=64"`
	Component2ID string `json:"component2_id" validate:"omitempty,max=64"`
}

type UpdateDTO struct {
	Name              string   `json:"name"                validate:"omitempty,max=120"`
	Kind              string   `json:"kind"                validate:"omitempty,oneof=ppn pph other"`
	CalcMethod        string   `json:"calc_method"         validate:"omitempty,oneof=exclusive inclusive"`
	Rate              *float64 `json:"rate"                validate:"omitempty,gte=0,lte=100"`
	SalesAccountID    *string  `json:"sales_account_id"`
	PurchaseAccountID *string  `json:"purchase_account_id"`
	Component1ID      *string  `json:"component1_id"`
	Component2ID      *string  `json:"component2_id"`
	IsActive          *bool    `json:"is_active"`
}

type Filter struct {
	CompanyID string
	Search    string
	Page      int
	PageSize  int
	Sort      string
	Order     string
	Fields    []string
}

type IRepository interface {
	Create(dto *CreateDTO, actorID string) (*model.Tax, error)
	Update(companyID, id string, dto *UpdateDTO, actorID string) (*model.Tax, error)
	FindByID(companyID, id string) (*model.Tax, error)
	FindAll(f *Filter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
	SeedDefaults(companyID, actorID string) error
}

type IService interface {
	Create(companyID, actorID string, dto *CreateDTO) (*model.Tax, error)
	Update(companyID, actorID, id string, dto *UpdateDTO) (*model.Tax, error)
	Get(companyID, id string) (*model.Tax, error)
	List(f *Filter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
}

type ErrNotFound struct{ ID string }

func (e *ErrNotFound) Error() string { return fmt.Sprintf("tax %s not found", e.ID) }

type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }

// ErrSystemLocked — a seeded default's name/kind can't be changed (rate,
// calc method and accounts can).
type ErrSystemLocked struct{}

func (e *ErrSystemLocked) Error() string {
	return "built-in system tax: only rate, method, accounts, and status can be changed"
}

// ErrComponentInUse — a single tax can't be deleted while a compound tax
// still references it.
type ErrComponentInUse struct{}

func (e *ErrComponentInUse) Error() string {
	return "this tax is used in a compound tax — remove it from the compound tax first"
}
