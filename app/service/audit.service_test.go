package service

import (
	"testing"

	domain "duluin_invoice/app/domain/audit"
	"duluin_invoice/app/model"
)

type fakeAuditRepo struct {
	rows []*model.AuditLog
}

func (f *fakeAuditRepo) Insert(rec *model.AuditLog) error {
	f.rows = append(f.rows, rec)
	return nil
}

func (f *fakeAuditRepo) List(filter domain.Filter) ([]model.AuditLog, int64, error) {
	var out []model.AuditLog
	for _, r := range f.rows {
		if r.CompanyID != filter.CompanyID {
			continue // company isolation: only ever compare against the requested company
		}
		out = append(out, *r)
	}
	return out, int64(len(out)), nil
}

func (f *fakeAuditRepo) FindByID(companyID, id string) (*model.AuditLog, error) {
	for _, r := range f.rows {
		if r.CompanyID == companyID && r.ID == id {
			return r, nil
		}
	}
	return nil, nil
}

func TestAuditLog_WritesScopedRow(t *testing.T) {
	repo := &fakeAuditRepo{}
	svc := NewAuditService(repo)

	svc.Log(domain.Actor{CompanyID: "c1", UserID: "u1", Name: "Tiara Firdausa", Email: "tiara@x.com", IPAddress: "10.0.0.1"}, domain.Entry{
		Action:      domain.ActionUpdated,
		Module:      domain.ModuleSalesInvoice,
		EntityType:  "sales_invoice",
		EntityID:    "inv-1",
		EntityName:  "INV/2026/0024",
		Description: "Updated invoice details",
		Changes: map[string]domain.Change{
			"payment_status": {Before: "unpaid", After: "partially_paid"},
		},
	})

	if len(repo.rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(repo.rows))
	}
	got := repo.rows[0]
	if got.CompanyID != "c1" || got.ActorName != "Tiara Firdausa" || got.Action != domain.ActionUpdated || got.EntityName != "INV/2026/0024" {
		t.Fatalf("unexpected row: %+v", got)
	}
	if got.Changes == "" || got.Changes == "{}" {
		t.Fatalf("expected changes to be encoded, got %q", got.Changes)
	}
}

func TestAuditLog_DropsEntryWithoutCompany(t *testing.T) {
	repo := &fakeAuditRepo{}
	svc := NewAuditService(repo)

	svc.Log(domain.Actor{UserID: "u1"}, domain.Entry{Action: domain.ActionUpdated, Module: domain.ModuleSalesInvoice})

	if len(repo.rows) != 0 {
		t.Fatalf("want no row written without a company, got %d", len(repo.rows))
	}
}

func TestAuditList_CompanyIsolation(t *testing.T) {
	repo := &fakeAuditRepo{}
	svc := NewAuditService(repo)

	svc.Log(domain.Actor{CompanyID: "c1", Name: "A"}, domain.Entry{Action: domain.ActionCreated, Module: domain.ModuleSalesInvoice, Description: "x"})
	svc.Log(domain.Actor{CompanyID: "c2", Name: "B"}, domain.Entry{Action: domain.ActionCreated, Module: domain.ModuleSalesInvoice, Description: "y"})

	res, err := svc.List(domain.Filter{CompanyID: "c1", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Data) != 1 {
		t.Fatalf("company A must not see company B's activity: got %d rows", len(res.Data))
	}
}
