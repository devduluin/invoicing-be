package repository

import (
	"testing"

	contactdomain "duluin_invoice/app/domain/contactperson"
	mitradomain "duluin_invoice/app/domain/mitra"
	"duluin_invoice/app/model"
)

func (f *flow) contacts(mitraID string) []model.ContactPerson {
	var rows []model.ContactPerson
	if err := f.db.Where("mitra_id = ?", mitraID).Find(&rows).Error; err != nil {
		f.t.Fatal(err)
	}
	return rows
}

// The PIC becomes exactly one contact person on create, and PIC edits / resubmits never add another.
func TestPartnerPICContact(t *testing.T) {
	f := newFlow(t)
	repo := &MitraRepository{db: f.db}
	m, err := repo.Create(&mitradomain.CreateMitraDTO{
		CompanyID: f.company, Type: "customer", Name: "PT Maju", ContactName: "Budi", Email: "budi@maju.id", Phone: "628111",
		// the same person typed by hand too: must not become a second row
		ContactPersons: []contactdomain.SyncInput{
			{Name: "Budi", Email: "budi@maju.id", Phone: "628111"},
			{Name: "Sari", Email: "sari@maju.id", Phone: "628222"},
		},
		ContactPerms: contactdomain.Perms{Create: true},
	}, "u")
	if err != nil {
		t.Fatal(err)
	}
	cs := f.contacts(m.ID)
	if len(cs) != 2 {
		t.Fatalf("want PIC + Sari = 2 contacts, got %d", len(cs))
	}
	var pic *model.ContactPerson
	for i := range cs {
		if cs[i].IsPIC {
			pic = &cs[i]
		}
	}
	if pic == nil || pic.Name != "Budi" || pic.Email != "budi@maju.id" || pic.Phone != "628111" {
		t.Fatalf("PIC contact wrong: %+v", pic)
	}

	// an unrelated save (same PIC values) adds nothing and touches nothing
	if _, err := repo.Update(f.company, m.ID, &mitradomain.UpdateMitraDTO{Name: "PT Maju Jaya", ContactName: "Budi", Email: "budi@maju.id", Phone: "628111"}, "u"); err != nil {
		t.Fatal(err)
	}
	if n := len(f.contacts(m.ID)); n != 2 {
		t.Fatalf("resave created a duplicate: %d contacts", n)
	}

	// changing the PIC email updates the same row
	if _, err := repo.Update(f.company, m.ID, &mitradomain.UpdateMitraDTO{ContactName: "Budi", Email: "baru@maju.id", Phone: "628111"}, "u"); err != nil {
		t.Fatal(err)
	}
	cs = f.contacts(m.ID)
	if len(cs) != 2 {
		t.Fatalf("PIC edit created a duplicate: %d contacts", len(cs))
	}
	for _, c := range cs {
		if c.IsPIC && c.Email != "baru@maju.id" {
			t.Fatalf("PIC contact not updated: %+v", c)
		}
	}
}

// A partner saved before this feature existed has a PIC (name/email/phone on the mitra row) but no
// is_pic contact person at all. Simply editing and re-saving it — even without touching the PIC
// fields — must backfill that missing contact, not silently skip because "nothing changed".
func TestPartnerPICContact_BackfillsOnUnrelatedEdit(t *testing.T) {
	f := newFlow(t)
	repo := &MitraRepository{db: f.db}
	m, err := repo.Create(&mitradomain.CreateMitraDTO{
		CompanyID: f.company, Type: "customer", Name: "PT Lama", ContactName: "Rudi", Email: "rudi@lama.id", Phone: "628999",
		ContactPerms: contactdomain.Perms{Create: true},
	}, "u")
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a pre-feature partner: delete the auto-created PIC contact, as if it never existed.
	if err := f.db.Where("mitra_id = ?", m.ID).Delete(&model.ContactPerson{}).Error; err != nil {
		t.Fatal(err)
	}
	if n := len(f.contacts(m.ID)); n != 0 {
		t.Fatalf("setup: want 0 contacts, got %d", n)
	}

	// An unrelated edit that leaves the PIC fields exactly as they are.
	if _, err := repo.Update(f.company, m.ID, &mitradomain.UpdateMitraDTO{Address: strPtr("Jl. Baru No. 1")}, "u"); err != nil {
		t.Fatal(err)
	}
	cs := f.contacts(m.ID)
	if len(cs) != 1 || !cs[0].IsPIC || cs[0].Name != "Rudi" || cs[0].Email != "rudi@lama.id" || cs[0].Phone != "628999" {
		t.Fatalf("PIC contact was not backfilled: %+v", cs)
	}

	// Saving again must not duplicate it.
	if _, err := repo.Update(f.company, m.ID, &mitradomain.UpdateMitraDTO{Address: strPtr("Jl. Baru No. 2")}, "u"); err != nil {
		t.Fatal(err)
	}
	if n := len(f.contacts(m.ID)); n != 1 {
		t.Fatalf("second edit created a duplicate: %d contacts", n)
	}
}

func strPtr(s string) *string { return &s }
