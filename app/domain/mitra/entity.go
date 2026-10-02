package domain_mitra

import (
	contactdomain "duluin_invoice/app/domain/contactperson"
	"duluin_invoice/app/model"
)

// CreateMitraDTO is the create payload for a Mitra (PRD §8).
type CreateMitraDTO struct {
	CompanyID   string `json:"-"`
	// Code is optional: blank = the next MTR-NNNN.
	Code string `json:"code" validate:"omitempty,max=50"`
	// LinkedCompanyCode — the partner's own Duluin Company Code, when it has one (optional).
	LinkedCompanyCode string `json:"linked_company_code" validate:"omitempty,max=20"`
	Type        string `json:"type" validate:"required,oneof=customer supplier both"`
	Name        string `json:"name" validate:"required,max=255"`
	ContactName string `json:"contact_name" validate:"required,max=255"`
	// PIC email and phone are required: the partner's first contact person is generated from them.
	Email       string `json:"email" validate:"required,email,max=150"`
	Phone       string `json:"phone" validate:"required,max=50"`
	Npwp        string `json:"npwp" validate:"omitempty,max=50"`
	Address     string `json:"address" validate:"omitempty"`
	IsActive    *bool  `json:"is_active"`
	// ContactPersons are saved together with the partner, in the same transaction.
	ContactPersons []contactdomain.SyncInput `json:"contact_persons" validate:"omitempty,max=50,dive"`
	// ContactPerms is filled by the controller from the caller's permissions (never from the body).
	ContactPerms contactdomain.Perms `json:"-"`
}

// ImportMitraRow is one partner from the import file. Row is the spreadsheet row of the partner's
// first line, only used to point error messages at the right place.
type ImportMitraRow struct {
	Row int `json:"row"`
	CreateMitraDTO
}

// ImportMitraDTO — POST /mitra/import. Saved all-or-nothing.
type ImportMitraDTO struct {
	Partners []ImportMitraRow `json:"partners"`
}

// ImportMitraItem — one partner to save in an import, with the label its error messages start with.
type ImportMitraItem struct {
	Label string
	DTO   *CreateMitraDTO
}

// ImportMitraResult — a partner the import saved: created, or (Updated) the existing partner with the
// same code, updated from the file.
type ImportMitraResult struct {
	Mitra   *model.Mitra
	Updated bool
}

// MaxImportPartners caps one import file.
const MaxImportPartners = 500

// UpdateMitraDTO — all fields optional; only provided keys are applied.
type UpdateMitraDTO struct {
	Code string `json:"code" validate:"omitempty,max=50"`
	// LinkedCompanyCode: nil = leave the link alone, "" = unlink, a code = link to that company.
	LinkedCompanyCode *string `json:"linked_company_code" validate:"omitempty,max=20"`
	Type        string  `json:"type" validate:"omitempty,oneof=customer supplier both"`
	Name        string  `json:"name" validate:"omitempty,max=255"`
	ContactName string  `json:"contact_name" validate:"omitempty,max=255"`
	Email       string  `json:"email" validate:"omitempty,email,max=150"`
	Phone       string  `json:"phone" validate:"omitempty,max=50"`
	Npwp        string  `json:"npwp" validate:"omitempty,max=50"`
	Address     *string `json:"address"`
	IsActive    *bool   `json:"is_active"`
	// ContactPersons: nil = leave the partner's contacts alone; a list (even empty) makes the
	// partner's contacts EXACTLY that list (rows with an id are updated, without an id created,
	// missing ones deleted).
	ContactPersons []contactdomain.SyncInput `json:"contact_persons" validate:"omitempty,max=50,dive"`
	// ContactPerms is filled by the controller from the caller's permissions (never from the body).
	ContactPerms contactdomain.Perms `json:"-"`
}

// MitraFilter drives the list query.
type MitraFilter struct {
	CompanyID string
	Search    string
	Type      string
	IsActive  *bool
	Page      int
	PageSize  int
	Sort      string
	Order     string
	Fields    []string
	// WithDetails — ?with=details: also load the lines / contact persons (exports).
	WithDetails bool
}
