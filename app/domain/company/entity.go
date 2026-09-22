// Package domain_company holds the company-settings types (profile + logo edit).
package domain_company

import (
	membership "duluin_invoice/app/domain/membership"
	"duluin_invoice/app/model"
)

// Actor is the authenticated caller of a company-settings action.
type Actor struct {
	UserID          string
	ActiveCompanyID string
	Token           string
}

func (a Actor) ToMembership() membership.Actor {
	return membership.Actor{UserID: a.UserID, ActiveCompanyID: a.ActiveCompanyID, Token: a.Token}
}

// UpdateProfileDTO — the company-settings form. All fields optional; only
// provided keys are applied. company_logo carries a `data:` URI (new upload),
// an existing URL (keep), or "" (remove).
type UpdateProfileDTO struct {
	Name        *string `json:"name"         validate:"omitempty,min=2,max=255"`
	CompanyLogo *string `json:"company_logo" validate:"omitempty"`
	Email       *string `json:"email"        validate:"omitempty"`
	Phone       *string `json:"phone"        validate:"omitempty,max=40"`
	Npwp        *string `json:"npwp"         validate:"omitempty,max=30"`
	Alamat      *string `json:"alamat"       validate:"omitempty,max=255"`
	Kota        *string `json:"kota"         validate:"omitempty,max=100"`
	Provinsi    *string `json:"provinsi"     validate:"omitempty,max=100"`
	KodePos     *string `json:"kode_pos"     validate:"omitempty,max=10"`
	Website     *string `json:"website"      validate:"omitempty,max=255"`

	// JenisUsaha (industry) and JumlahKaryawan (company size) are required for the Free-plan
	// activation checklist's "company profile completed" step (see companyProfileComplete in
	// app/service/activation.service.go). They are set at onboarding but must also be editable
	// here, since a company created before this requirement — or one that never went through the
	// wizard — would otherwise have no way to complete this step.
	JenisUsaha     *string `json:"jenis_usaha"     validate:"omitempty,max=120"`
	JumlahKaryawan *string `json:"jumlah_karyawan" validate:"omitempty,max=20"`
}

// ICompanyService is the application port used by the HTTP layer.
type ICompanyService interface {
	UpdateProfile(a Actor, dto UpdateProfileDTO) (*model.Company, error)
}
