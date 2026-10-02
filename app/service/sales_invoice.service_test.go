package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	domain "duluin_invoice/app/domain/salesinvoice"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// fakeSalesInvoiceRepo — a small hand-written stand-in for domain.IRepository
// (same style as fakeSalesOrderRepo/fakeReportRepo).
type fakeSalesInvoiceRepo struct {
	existingNumbers map[string]bool
	invoice         *model.SalesInvoice
	taxRates        map[string]utils.TaxRate
	mitraExists     bool
	imported        []domain.ImportItem
}

func (f *fakeSalesInvoiceRepo) Create(dto *domain.CreateDTO, calc *utils.LinesCalc, actorID string) (*model.SalesInvoice, error) {
	return f.invoice, nil
}
func (f *fakeSalesInvoiceRepo) Update(companyID, id string, dto *domain.UpdateDTO, calc *utils.LinesCalc, actorID string) (*model.SalesInvoice, error) {
	return f.invoice, nil
}
func (f *fakeSalesInvoiceRepo) FindByID(companyID, id string) (*model.SalesInvoice, error) {
	if f.invoice == nil {
		return nil, &domain.ErrNotFound{ID: id}
	}
	return f.invoice, nil
}
func (f *fakeSalesInvoiceRepo) FindAll(fl *domain.Filter) (*utils.OffsetPaginationResult, error) {
	return nil, nil
}
func (f *fakeSalesInvoiceRepo) Summary(companyID string) (*domain.Summary, error) {
	return &domain.Summary{}, nil
}
func (f *fakeSalesInvoiceRepo) SetTemplate(companyID, id, actorID, template string) error {
	if f.invoice != nil {
		f.invoice.Template = template
	}
	return nil
}
func (f *fakeSalesInvoiceRepo) Delete(companyID, id string) error { return nil }
func (f *fakeSalesInvoiceRepo) SetStatus(companyID, id, actorID string, status model.SalesInvoiceStatus) error {
	if f.invoice != nil {
		f.invoice.Status = status
	}
	return nil
}
func (f *fakeSalesInvoiceRepo) MitraExists(companyID, mitraID string) (bool, error) {
	return f.mitraExists, nil
}
func (f *fakeSalesInvoiceRepo) TaxRates(companyID string, taxIDs []string) (map[string]utils.TaxRate, error) {
	return f.taxRates, nil
}

func (f *fakeSalesInvoiceRepo) ImportExistingNumbers(companyID string, numbers []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, n := range numbers {
		if f.existingNumbers[strings.ToLower(n)] {
			out[strings.ToLower(n)] = true
		}
	}
	return out, nil
}

func (f *fakeSalesInvoiceRepo) ImportMany(items []domain.ImportItem, actorID string, prior []string) ([]domain.ImportCreated, error) {
	if len(prior) > 0 { // like the real one: checked, rolled back, returned together
		return nil, &utils.ImportErrors{Messages: prior}
	}
	f.imported = items
	out := make([]domain.ImportCreated, len(items))
	for i, it := range items {
		out[i] = domain.ImportCreated{ID: "id", Number: it.DTO.Number}
	}
	return out, nil
}

// fakeTxnLimiter — capacity is how many transactions may still be created this month.
type fakeTxnLimiter struct{ capacity int }

func (f *fakeTxnLimiter) CheckTransactionLimit(companyID string) error {
	return f.CheckTransactionCapacity(companyID, 1)
}
func (f *fakeTxnLimiter) CheckTransactionCapacity(companyID string, adding int) error {
	if adding > f.capacity {
		return errors.New("limit reached")
	}
	return nil
}
func (f *fakeTxnLimiter) Recompute(companyID string) {}

func TestSalesInvoiceImport(t *testing.T) {
	row := func(r int, number string) domain.ImportRow {
		return domain.ImportRow{Row: r, CreateDTO: domain.CreateDTO{
			MitraID: "m1", Number: number, Date: "2026-09-01", DueDate: "2026-09-30",
			Lines: []domain.LineDTO{siLine("A", 2, 1000)},
		}}
	}

	t.Run("saves every invoice as a regular invoice", func(t *testing.T) {
		repo := &fakeSalesInvoiceRepo{mitraExists: true}
		svc := &SalesInvoiceService{repo: repo, activation: &fakeTxnLimiter{capacity: 10}}
		in := row(2, "INV/1")
		in.Kind = "down_payment"
		created, err := svc.Import("c1", "actor", []domain.ImportRow{in, row(4, "INV/2")})
		if err != nil || len(created) != 2 {
			t.Fatalf("want 2 created, got %v, %v", created, err)
		}
		if repo.imported[0].DTO.Kind != "invoice" || repo.imported[0].Calc.GrandTotal != 2000 {
			t.Fatalf("want a regular invoice of 2000, got %s %v", repo.imported[0].DTO.Kind, repo.imported[0].Calc.GrandTotal)
		}
	})

	t.Run("names the row of a duplicate number or a missing partner", func(t *testing.T) {
		svc := &SalesInvoiceService{repo: &fakeSalesInvoiceRepo{mitraExists: true}, activation: &fakeTxnLimiter{capacity: 10}}
		_, err := svc.Import("c1", "actor", []domain.ImportRow{row(2, "INV/1"), row(5, "inv/1")})
		if err == nil || err.Error() != "Row 5: invoice no. inv/1 is also used on row 2" {
			t.Fatalf("got %v", err)
		}
		svc = &SalesInvoiceService{repo: &fakeSalesInvoiceRepo{}, activation: &fakeTxnLimiter{capacity: 10}}
		if _, err := svc.Import("c1", "actor", []domain.ImportRow{row(3, "")}); err == nil || err.Error() != "Row 3: partner not found" {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("reports every problem of the file together and saves nothing", func(t *testing.T) {
		repo := &fakeSalesInvoiceRepo{}
		svc := &SalesInvoiceService{repo: repo, activation: &fakeTxnLimiter{capacity: 10}}
		_, err := svc.Import("c1", "actor", []domain.ImportRow{row(3, "INV/1"), row(4, "inv/1")})
		var all *utils.ImportErrors
		if !errors.As(err, &all) || len(all.Messages) != 3 || repo.imported != nil {
			t.Fatalf("want 3 problems (2 missing partners + 1 duplicate number) and nothing saved, got %v", err)
		}
	})

	t.Run("a number the company already has is an update, not counted against the monthly cap", func(t *testing.T) {
		repo := &fakeSalesInvoiceRepo{mitraExists: true, existingNumbers: map[string]bool{"inv/1": true}}
		svc := &SalesInvoiceService{repo: repo, activation: &fakeTxnLimiter{capacity: 1}}
		if _, err := svc.Import("c1", "actor", []domain.ImportRow{row(2, "INV/1"), row(3, "INV/2")}); err != nil {
			t.Fatalf("1 update + 1 new fits a cap of 1, got %v", err)
		}
	})

	t.Run("checks the monthly cap for the whole file", func(t *testing.T) {
		repo := &fakeSalesInvoiceRepo{mitraExists: true}
		svc := &SalesInvoiceService{repo: repo, activation: &fakeTxnLimiter{capacity: 1}}
		if _, err := svc.Import("c1", "actor", []domain.ImportRow{row(2, ""), row(3, "")}); err == nil || repo.imported != nil {
			t.Fatalf("want the cap to refuse the file before saving, got %v", err)
		}
	})
}

func siLine(product string, qty, price float64) domain.LineDTO {
	return domain.LineDTO{ProductName: product, Quantity: qty, UnitPrice: price}
}

func TestSalesInvoiceStatusTransitions(t *testing.T) {
	newInvoice := func(status model.SalesInvoiceStatus) *model.SalesInvoice {
		return &model.SalesInvoice{
			ID: "inv-1", Status: status,
			Lines: []model.SalesInvoiceLine{{ProductName: "A", Quantity: 1, UnitPrice: 1000}},
		}
	}

	t.Run("confirm from draft succeeds", func(t *testing.T) {
		repo := &fakeSalesInvoiceRepo{invoice: newInvoice(model.SalesInvoiceStatusDraft)}
		svc := &SalesInvoiceService{repo: repo}
		updated, err := svc.Confirm("c1", "actor", "inv-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Status != model.SalesInvoiceStatusConfirmed {
			t.Errorf("expected confirmed, got %s", updated.Status)
		}
	})

	t.Run("confirm from confirmed rejected", func(t *testing.T) {
		repo := &fakeSalesInvoiceRepo{invoice: newInvoice(model.SalesInvoiceStatusConfirmed)}
		svc := &SalesInvoiceService{repo: repo}
		_, err := svc.Confirm("c1", "actor", "inv-1")
		var it *domain.ErrInvalidTransition
		if !errors.As(err, &it) {
			t.Fatalf("expected ErrInvalidTransition, got %v", err)
		}
	})

	t.Run("back to draft from confirmed succeeds", func(t *testing.T) {
		repo := &fakeSalesInvoiceRepo{invoice: newInvoice(model.SalesInvoiceStatusConfirmed)}
		svc := &SalesInvoiceService{repo: repo}
		updated, err := svc.BackToDraft("c1", "actor", "inv-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Status != model.SalesInvoiceStatusDraft {
			t.Errorf("expected draft, got %s", updated.Status)
		}
	})

	t.Run("cancel from draft succeeds", func(t *testing.T) {
		repo := &fakeSalesInvoiceRepo{invoice: newInvoice(model.SalesInvoiceStatusDraft)}
		svc := &SalesInvoiceService{repo: repo}
		updated, err := svc.Cancel("c1", "actor", "inv-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Status != model.SalesInvoiceStatusCancelled {
			t.Errorf("expected cancelled, got %s", updated.Status)
		}
	})

	t.Run("cancel twice rejected", func(t *testing.T) {
		repo := &fakeSalesInvoiceRepo{invoice: newInvoice(model.SalesInvoiceStatusCancelled)}
		svc := &SalesInvoiceService{repo: repo}
		_, err := svc.Cancel("c1", "actor", "inv-1")
		var it *domain.ErrInvalidTransition
		if !errors.As(err, &it) {
			t.Fatalf("expected ErrInvalidTransition, got %v", err)
		}
	})

	t.Run("update allowed once confirmed", func(t *testing.T) {
		repo := &fakeSalesInvoiceRepo{invoice: newInvoice(model.SalesInvoiceStatusConfirmed), mitraExists: true}
		svc := &SalesInvoiceService{repo: repo}
		_, err := svc.Update("c1", "actor", "inv-1", &domain.UpdateDTO{
			MitraID: "m1", Date: "2026-01-01",
			Lines: []domain.LineDTO{siLine("A", 1, 1000)},
		})
		if err != nil {
			t.Fatalf("status must not make a document immutable, got %v", err)
		}
	})

	t.Run("delete allowed once confirmed", func(t *testing.T) {
		repo := &fakeSalesInvoiceRepo{invoice: newInvoice(model.SalesInvoiceStatusConfirmed)}
		svc := &SalesInvoiceService{repo: repo}
		err := svc.Delete("c1", "inv-1")
		if err != nil {
			t.Fatalf("status must not make a document immutable, got %v", err)
		}
	})

	t.Run("create rejects invalid kind", func(t *testing.T) {
		repo := &fakeSalesInvoiceRepo{mitraExists: true}
		svc := &SalesInvoiceService{repo: repo}
		_, err := svc.Create("c1", "actor", &domain.CreateDTO{
			Kind: "bogus", MitraID: "m1", Date: "2026-01-01",
			Lines: []domain.LineDTO{siLine("A", 1, 1000)},
		})
		var v *domain.ErrValidation
		if !errors.As(err, &v) {
			t.Fatalf("expected ErrValidation for invalid kind, got %v", err)
		}
	})
}

func (f *fakeSalesInvoiceRepo) PreviewNumber(companyID, kind string) (string, error) {
	return "INV/2026/0001", nil
}

func (f *fakeSalesInvoiceRepo) CountByKind(companyID, kind string) (int64, error) { return 0, nil }
func (f *fakeSalesInvoiceRepo) CountCreatedSince(companyID string, since time.Time) (int64, error) {
	return 0, nil
}
