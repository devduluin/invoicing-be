package domain_onboarding

import (
	"crypto/rand"
	"strings"

	"duluin_invoice/app/model"
)

// Pure onboarding rules (PRD §4). No I/O — deterministic, unit-tested in isolation.

// ValidateSubmit enforces the cross-field business rules the struct tags can't:
// NPWP is mandatory for enterprise, the employee-count bucket must be known.
func ValidateSubmit(dto SubmitDTO) error {
	if !model.IsValidAccountType(dto.TipeAkun) {
		return &ErrValidation{Message: "tipe_akun must be perseorangan or enterprise"}
	}
	if !model.IsValidEmployeeCount(dto.JumlahKaryawan) {
		return &ErrValidation{Message: "employee count is required"}
	}
	if model.AccountType(dto.TipeAkun) == model.AccountTypeEnterprise &&
		strings.TrimSpace(dto.Npwp) == "" {
		return &ErrValidation{Message: "npwp is required for enterprise accounts"}
	}
	if _, err := NormalizeNeeds(dto.KebutuhanUser); err != nil {
		return err
	}
	return nil
}

// NormalizeNeeds trims, drops blanks and de-duplicates the "Kebutuhan User"
// selection.
func NormalizeNeeds(raw []string) ([]string, error) {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		t := strings.TrimSpace(v)
		if t == "" {
			continue
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil, &ErrValidation{Message: "select at least one need"}
	}
	return out, nil
}

// companyCodeAlphabet — Crockford-ish: no 0/O/1/I/L to keep the code readable
// when shared verbally or typed (it's the Company ID partners quote back).
const companyCodeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// NewCompanyCode returns a random n-char uppercase code. Uniqueness is the
// caller's job via the exists probe.
func NewCompanyCode(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		for i := range buf {
			buf[i] = byte(i * 7)
		}
	}
	out := make([]byte, n)
	for i, b := range buf {
		out[i] = companyCodeAlphabet[int(b)%len(companyCodeAlphabet)]
	}
	return string(out)
}
