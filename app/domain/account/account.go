// Package domain_account — Chart of Accounts master data (PRD §7).
package domain_account

import (
	"fmt"

	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type CreateDTO struct {
	CompanyID string  `json:"-"`
	Code      string  `json:"code"      validate:"required,max=20"`
	Name      string  `json:"name"      validate:"required,max=255"`
	Group     string  `json:"group"     validate:"required,oneof=asset liability equity income expense"`
	ParentID  *string `json:"parent_id" validate:"omitempty,max=64"`
	IsActive  *bool   `json:"is_active"`
}

type UpdateDTO struct {
	Code     string  `json:"code"      validate:"omitempty,max=20"`
	Name     string  `json:"name"      validate:"omitempty,max=255"`
	Group    string  `json:"group"     validate:"omitempty,oneof=asset liability equity income expense"`
	ParentID *string `json:"parent_id"`
	IsActive *bool   `json:"is_active"`
}

type Filter struct {
	CompanyID string
	Search    string
	Group     string
	Page      int
	PageSize  int
	Sort      string
	Order     string
	Fields    []string
}

type IRepository interface {
	Create(dto *CreateDTO, actorID string) (*model.Account, error)
	Update(companyID, id string, dto *UpdateDTO, actorID string) (*model.Account, error)
	FindByID(companyID, id string) (*model.Account, error)
	FindAll(f *Filter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
	CodeExists(companyID, code, exceptID string) (bool, error)
	HasChildren(companyID, id string) (bool, error)
	SeedDefaults(companyID, actorID string) error
}

type IService interface {
	Create(companyID, actorID string, dto *CreateDTO) (*model.Account, error)
	Update(companyID, actorID, id string, dto *UpdateDTO) (*model.Account, error)
	Get(companyID, id string) (*model.Account, error)
	List(f *Filter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
}

type ErrNotFound struct{ ID string }

func (e *ErrNotFound) Error() string { return fmt.Sprintf("account %s not found", e.ID) }

type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }

type ErrCodeExists struct{ Code string }

func (e *ErrCodeExists) Error() string { return fmt.Sprintf("account code %s is already in use", e.Code) }

type ErrHasChildren struct{}

func (e *ErrHasChildren) Error() string { return "this account has sub-accounts — delete them first" }

type ErrSystemLocked struct{ Msg string }

func (e *ErrSystemLocked) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	return "this built-in system account can't be changed this way"
}
