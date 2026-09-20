// Package domain_unit — configurable unit-of-measure master data.
package domain_unit

import (
	"fmt"

	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type CreateDTO struct {
	CompanyID string `json:"-"`
	Name      string `json:"name"      validate:"required,max=50"`
	Symbol    string `json:"symbol"    validate:"omitempty,max=30"`
	IsActive  *bool  `json:"is_active"`
}

type UpdateDTO struct {
	Name     string  `json:"name"      validate:"omitempty,max=50"`
	Symbol   *string `json:"symbol"`
	IsActive *bool   `json:"is_active"`
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
	Create(dto *CreateDTO, actorID string) (*model.Unit, error)
	Update(companyID, id string, dto *UpdateDTO, actorID string) (*model.Unit, error)
	FindByID(companyID, id string) (*model.Unit, error)
	FindAll(f *Filter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
	SeedDefaults(companyID, actorID string) error
}

type IService interface {
	Create(companyID, actorID string, dto *CreateDTO) (*model.Unit, error)
	Update(companyID, actorID, id string, dto *UpdateDTO) (*model.Unit, error)
	Get(companyID, id string) (*model.Unit, error)
	List(f *Filter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
}

type ErrNotFound struct{ ID string }

func (e *ErrNotFound) Error() string { return fmt.Sprintf("unit %s not found", e.ID) }

type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }

// ErrSystemLocked — a seeded default unit's name can't be changed (active
// status can).
type ErrSystemLocked struct{}

func (e *ErrSystemLocked) Error() string {
	return "built-in system unit: only status can be changed"
}
