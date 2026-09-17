// Package domain_journalbook — "Buku Jurnal" master data (Odoo: account.journal).
// The reference UI's "Jurnal" dropdown on the journal entry form.
package domain_journalbook

import (
	"fmt"

	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type CreateDTO struct {
	CompanyID              string  `json:"-"`
	Code                   string  `json:"code"                       validate:"required,max=20"`
	Name                   string  `json:"name"                       validate:"required,max=255"`
	Type                   string  `json:"type"                       validate:"required,oneof=general sale purchase cash bank"`
	DefaultAccountID       *string `json:"default_account_id"         validate:"omitempty,uuid4"`
	DefaultDebitAccountID  *string `json:"default_debit_account_id"   validate:"omitempty,uuid4"`
	DefaultCreditAccountID *string `json:"default_credit_account_id"  validate:"omitempty,uuid4"`
	IsActive               *bool   `json:"is_active"`
}

type UpdateDTO struct {
	Code                   string  `json:"code"                       validate:"omitempty,max=20"`
	Name                   string  `json:"name"                       validate:"omitempty,max=255"`
	Type                   string  `json:"type"                       validate:"omitempty,oneof=general sale purchase cash bank"`
	DefaultAccountID       *string `json:"default_account_id"`
	DefaultDebitAccountID  *string `json:"default_debit_account_id"`
	DefaultCreditAccountID *string `json:"default_credit_account_id"`
	IsActive               *bool   `json:"is_active"`
}

type Filter struct {
	CompanyID string
	Search    string
	Type      string
	Page      int
	PageSize  int
	Sort      string
	Order     string
	Fields    []string
}

type IRepository interface {
	Create(dto *CreateDTO, actorID string) (*model.JournalBook, error)
	Update(companyID, id string, dto *UpdateDTO, actorID string) (*model.JournalBook, error)
	FindByID(companyID, id string) (*model.JournalBook, error)
	FindAll(f *Filter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
	CodeExists(companyID, code, exceptID string) (bool, error)
	HasEntries(companyID, id string) (bool, error)
	SeedDefaults(companyID, actorID string) error
}

type IService interface {
	Create(companyID, actorID string, dto *CreateDTO) (*model.JournalBook, error)
	Update(companyID, actorID, id string, dto *UpdateDTO) (*model.JournalBook, error)
	Get(companyID, id string) (*model.JournalBook, error)
	List(f *Filter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
}

type ErrNotFound struct{ ID string }

func (e *ErrNotFound) Error() string { return fmt.Sprintf("journal book %s not found", e.ID) }

type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }

type ErrCodeExists struct{ Code string }

func (e *ErrCodeExists) Error() string { return fmt.Sprintf("journal book code %s is already in use", e.Code) }

type ErrHasEntries struct{}

func (e *ErrHasEntries) Error() string {
	return "this journal book is already used by journal entries — deactivate it instead"
}

type ErrSystemLocked struct{ Msg string }

func (e *ErrSystemLocked) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	return "this built-in system journal book can't be changed this way"
}
