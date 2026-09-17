package domain_onboarding

import (
	"strings"
	"testing"
)

func validSubmit() SubmitDTO {
	return SubmitDTO{
		NamaPerusahaan: "Toko Maju",
		TipeAkun:       "perseorangan",
		JenisUsaha:     "Retail",
		JumlahKaryawan: "6-10",
		Telepon:        "0811",
		KebutuhanUser:  []string{"invoicing"},
	}
}

func TestValidateSubmit(t *testing.T) {
	cases := []struct {
		name    string
		mut     func(*SubmitDTO)
		wantErr bool
	}{
		{"perseorangan ok", func(*SubmitDTO) {}, false},
		{"enterprise ok with npwp", func(d *SubmitDTO) { d.TipeAkun = "enterprise"; d.Npwp = "01.234" }, false},
		{"enterprise missing npwp", func(d *SubmitDTO) { d.TipeAkun = "enterprise" }, true},
		{"bad tipe_akun", func(d *SubmitDTO) { d.TipeAkun = "pt" }, true},
		{"missing jumlah_karyawan", func(d *SubmitDTO) { d.JumlahKaryawan = "" }, true},
		{"invalid jumlah_karyawan", func(d *SubmitDTO) { d.JumlahKaryawan = "banyak" }, true},
		{"empty kebutuhan", func(d *SubmitDTO) { d.KebutuhanUser = []string{" ", ""} }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dto := validSubmit()
			tc.mut(&dto)
			if err := ValidateSubmit(dto); (err != nil) != tc.wantErr {
				t.Fatalf("wantErr=%v got %v", tc.wantErr, err)
			}
		})
	}
}

func TestNormalizeNeeds(t *testing.T) {
	got, err := NormalizeNeeds([]string{" invoicing ", "invoicing", "", "reporting"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 2 || got[0] != "invoicing" || got[1] != "reporting" {
		t.Fatalf("dedupe/trim failed: %v", got)
	}
	if _, err := NormalizeNeeds([]string{"", "  "}); err == nil {
		t.Fatal("empty selection must error")
	}
}

func TestNewCompanyCode(t *testing.T) {
	seen := make(map[string]struct{}, 200)
	for i := 0; i < 200; i++ {
		code := NewCompanyCode(6)
		if len(code) != 6 {
			t.Fatalf("length = %d, want 6 (%q)", len(code), code)
		}
		if strings.ContainsAny(code, "01OIL") {
			t.Fatalf("code %q contains an ambiguous character", code)
		}
		seen[code] = struct{}{}
	}
	if len(seen) < 190 {
		t.Fatalf("too many collisions in 200 draws: %d unique", len(seen))
	}
}
