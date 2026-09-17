package domain_onboarding

import (
	membership "duluin_invoice/app/domain/membership"
	"duluin_invoice/app/model"
)

// Actor is the authenticated SSO context driving the onboarding submit.
type Actor struct {
	SSOUserID       string
	Email           string
	Name            string
	ActiveCompanyID string
	Token           string // raw Authorization header, forwarded to SSO
}

func (a Actor) ToMembership() membership.Actor {
	return membership.Actor{
		UserID:          a.SSOUserID,
		Email:           a.Email,
		Name:            a.Name,
		ActiveCompanyID: a.ActiveCompanyID,
		Token:           a.Token,
	}
}

// SubmitInvite is one teammate added on the "Undang Tim" step. Role is an SSO
// role id chosen by the inviter (never a fixed enum).
type SubmitInvite struct {
	Email  string `json:"email"   validate:"required,email,max=150"`
	Name   string `json:"name"    validate:"omitempty,max=255"`
	RoleID string `json:"role_id" validate:"required,max=64"`
}

// SubmitDTO is the whole wizard in one payload (PRD §4). The frontend keeps it
// as a local draft and only sends it here, on the final step.
type SubmitDTO struct {
	NamaPerusahaan string `json:"nama_perusahaan" validate:"required,min=2,max=255"`

	TipeAkun       string `json:"tipe_akun"       validate:"required,oneof=perseorangan enterprise"`
	JenisUsaha     string `json:"jenis_usaha"     validate:"required,max=120"`
	JumlahKaryawan string `json:"jumlah_karyawan" validate:"required,max=20"`
	Telepon        string `json:"telepon"         validate:"required,max=40"`
	Email          string `json:"email"           validate:"omitempty,email,max=150"`
	Alamat         string `json:"alamat"          validate:"omitempty,max=255"`
	Kota           string `json:"kota"            validate:"omitempty,max=100"`
	Provinsi       string `json:"provinsi"        validate:"omitempty,max=100"`
	KodePos        string `json:"kode_pos"        validate:"omitempty,max=10"`
	Npwp           string `json:"npwp"            validate:"omitempty,max=30"`

	KebutuhanUser []string       `json:"kebutuhan_user" validate:"required,min=1,max=20,dive,required,max=60"`
	Invites       []SubmitInvite `json:"invites"        validate:"omitempty,max=9,dive"`
}

// SubmitResult is returned to the frontend after a successful commit.
type SubmitResult struct {
	Company       *model.Company `json:"company"`
	InvitesSent   int            `json:"invites_sent"`
	FailedInvites []string       `json:"failed_invites,omitempty"` // emails that could not be invited
}
