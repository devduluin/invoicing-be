// Package domain_contactperson — the people at a partner (Mitra). A partner has many. Deleting is
// soft, so documents that used a contact keep their history. (The partner's own "Contact Name
// (PIC)" is a company field and is NOT a contact person.)
package domain_contactperson

import (
	"fmt"

	"duluin_invoice/app/model"
)

// Input is the create / update payload (a full replace on update).
type Input struct {
	Name     string `json:"name"     validate:"required,max=255"`
	Position string `json:"position" validate:"omitempty,max=150"`
	Phone    string `json:"phone"    validate:"omitempty,max=50"`
	Email    string `json:"email"    validate:"omitempty,email,max=150"`
}

// SyncInput is one row of the contact list submitted together with a partner. A row with an ID is
// an existing contact (updated); a row without one is new; existing contacts missing from the list
// are deleted (soft).
type SyncInput struct {
	ID       string `json:"id"       validate:"omitempty,uuid4"`
	Name     string `json:"name"     validate:"required,max=255"`
	Position string `json:"position" validate:"omitempty,max=150"`
	Phone    string `json:"phone"    validate:"omitempty,max=50"`
	Email    string `json:"email"    validate:"omitempty,email,max=150"`
}

// Perms — what the caller may do to contacts; a sync that needs more than that is refused as a whole.
type Perms struct{ Create, Update, Delete bool }

// Summary — one row per partner for the partner table.
type Summary struct {
	MitraID string `json:"mitra_id"`
	Count   int    `json:"count"`
}

type IRepository interface {
	List(companyID, mitraID, search string) ([]model.ContactPerson, error)
	Get(companyID, mitraID, id string) (*model.ContactPerson, error)
	Create(companyID, mitraID, actorID string, in *Input) (*model.ContactPerson, error)
	Update(companyID, mitraID, id, actorID string, in *Input) (*model.ContactPerson, error)
	Delete(companyID, mitraID, id string) error
	Summaries(companyID string) ([]Summary, error)
}

type IService interface {
	List(companyID, mitraID, search string) ([]model.ContactPerson, error)
	Get(companyID, mitraID, id string) (*model.ContactPerson, error)
	Create(companyID, mitraID, actorID string, in *Input) (*model.ContactPerson, error)
	Update(companyID, mitraID, id, actorID string, in *Input) (*model.ContactPerson, error)
	Delete(companyID, mitraID, id string) error
	Summaries(companyID string) ([]Summary, error)
}

type ErrNotFound struct{ ID string }

func (e *ErrNotFound) Error() string { return fmt.Sprintf("contact person %s not found", e.ID) }

type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }

// ErrDuplicate — the same contact (name + email + phone) already exists under this partner.
type ErrDuplicate struct{}

func (e *ErrDuplicate) Error() string { return "this contact person already exists for the partner" }

// ErrForbidden — the change needs a contact permission the caller does not have.
type ErrForbidden struct{ Action string }

func (e *ErrForbidden) Error() string {
	return "You don't have permission to " + e.Action + " contact persons"
}
