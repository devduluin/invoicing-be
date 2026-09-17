// Package domain_bankaccount — company bank accounts master data (PRD §11).
package domain_bankaccount

import (
	"fmt"

	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type CreateDTO struct {
	CompanyID     string `json:"-"`
	BankName      string `json:"bank_name"       validate:"required,max=120"`
	BankCode      string `json:"bank_code"       validate:"omitempty,max=10"`
	AccountNumber string `json:"account_number"  validate:"required,max=60"`
	AccountHolder string `json:"account_holder"  validate:"required,max=255"`
	Branch        string `json:"branch"          validate:"omitempty,max=120"`
	IsPrimary     *bool  `json:"is_primary"`
	IsActive      *bool  `json:"is_active"`
}

type UpdateDTO struct {
	BankName      string  `json:"bank_name"       validate:"omitempty,max=120"`
	BankCode      *string `json:"bank_code"       validate:"omitempty,max=10"`
	AccountNumber string  `json:"account_number"  validate:"omitempty,max=60"`
	AccountHolder string  `json:"account_holder"  validate:"omitempty,max=255"`
	Branch        *string `json:"branch"`
	IsPrimary     *bool   `json:"is_primary"`
	IsActive      *bool   `json:"is_active"`
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
	Create(dto *CreateDTO, actorID string) (*model.BankAccount, error)
	Update(companyID, id string, dto *UpdateDTO, actorID string) (*model.BankAccount, error)
	FindByID(companyID, id string) (*model.BankAccount, error)
	FindAll(f *Filter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
	// ClearPrimary unsets is_primary on every other row of the company.
	ClearPrimary(companyID, exceptID string) error
}

type IService interface {
	Create(companyID, actorID string, dto *CreateDTO) (*model.BankAccount, error)
	Update(companyID, actorID, id string, dto *UpdateDTO) (*model.BankAccount, error)
	Get(companyID, id string) (*model.BankAccount, error)
	List(f *Filter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
}

type ErrNotFound struct{ ID string }

func (e *ErrNotFound) Error() string { return fmt.Sprintf("bank account %s not found", e.ID) }

type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }
