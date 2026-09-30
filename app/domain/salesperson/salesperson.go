// Package domain_salesperson — sales person master data picked on Sales Orders and Sales Invoices.
package domain_salesperson

import (
	"fmt"

	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type CreateDTO struct {
	CompanyID string `json:"-"`
	// Code is optional: blank = the next SLS-NNNN.
	Code  string `json:"code"  validate:"omitempty,max=50"`
	Name  string `json:"name"  validate:"required,max=120"`
	Email string `json:"email" validate:"omitempty,email,max=150"`
	Phone string `json:"phone" validate:"omitempty,max=50"`
	// UserID links the salesperson to a team member (SSO user id); "" = not a user of the app.
	UserID   string `json:"user_id" validate:"omitempty,max=64"`
	IsActive *bool  `json:"is_active"`
}

// UpdateDTO — only provided keys are applied. UserID: nil = unchanged, "" = unlink.
type UpdateDTO struct {
	Code     string  `json:"code"  validate:"omitempty,max=50"`
	Name     string  `json:"name"  validate:"omitempty,max=120"`
	Email    *string `json:"email" validate:"omitempty,max=150"`
	Phone    *string `json:"phone" validate:"omitempty,max=50"`
	UserID   *string `json:"user_id" validate:"omitempty,max=64"`
	IsActive *bool   `json:"is_active"`
}

// FromMembersDTO — POST /salespersons/from-members: one salesperson per team member.
type FromMembersDTO struct {
	UserIDs []string `json:"user_ids" validate:"required,min=1,max=200"`
}

type Filter struct {
	CompanyID string
	Search    string
	IsActive  *bool
	Page      int
	PageSize  int
	Sort      string
	Order     string
	Fields    []string
}

// TeamMember — an active member of the company, and the salesperson already linked to it (if any).
type TeamMember struct {
	UserID        string `json:"user_id"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	Phone         string `json:"phone,omitempty"`
	SalespersonID string `json:"salesperson_id,omitempty"`
}

type IRepository interface {
	Create(dto *CreateDTO, actorID string) (*model.Salesperson, error)
	Update(companyID, id string, dto *UpdateDTO, actorID string) (*model.Salesperson, error)
	FindByID(companyID, id string) (*model.Salesperson, error)
	FindAll(f *Filter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
	NextCode(companyID string) (string, error)
	// FindByUser — the salesperson linked to this team member, or nil.
	FindByUser(companyID, userID string) (*model.Salesperson, error)
	TeamMembers(companyID string) ([]TeamMember, error)
	// CreateFromMembers — one salesperson per listed member not linked yet; returns the created ones.
	CreateFromMembers(companyID string, userIDs []string, actorID string) ([]model.Salesperson, error)
}

type IService interface {
	Create(companyID, actorID string, dto *CreateDTO) (*model.Salesperson, error)
	Update(companyID, actorID, id string, dto *UpdateDTO) (*model.Salesperson, error)
	Get(companyID, id string) (*model.Salesperson, error)
	List(f *Filter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
	NextCode(companyID string) (string, error)
	Mine(companyID, userID string) (*model.Salesperson, error)
	TeamMembers(companyID string) ([]TeamMember, error)
	CreateFromMembers(companyID, actorID string, userIDs []string) ([]model.Salesperson, error)
}

type ErrNotFound struct{ ID string }

func (e *ErrNotFound) Error() string { return fmt.Sprintf("salesperson %s not found", e.ID) }

type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }

// ErrConflict — the code, or the linked team member, is already taken by another salesperson.
type ErrConflict struct{ Message string }

func (e *ErrConflict) Error() string { return e.Message }
