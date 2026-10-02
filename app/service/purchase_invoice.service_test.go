package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	domain "duluin_invoice/app/domain/purchaseinvoice"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// fakePurchaseInvoiceRepo — a small hand-written stand-in for
// domain.IRepository (same style as fakeSalesInvoiceRepo).
type fakePurchaseInvoiceRepo struct {
	existingNumbers map[string]bool
	invoice         *model.PurchaseInvoice
	taxRates        map[string]utils.TaxRate
	mitraExists     bool
	imported        []domain.ImportItem
}

func (f *fakePurchaseInvoiceRepo) ImportExistingNumbers(companyID string, numbers []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, n := range numbers {
		if f.existingNumbers[strings.ToLower(n)] {
			out[strings.ToLower(n)] = true
		}
	}
	return out, nil
}

func (f *fakePurchaseInvoiceRepo) ImportMany(items []domain.ImportItem, actorID string, prior []string) ([]domain.ImportCreated, error) {
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

func TestPurchaseInvoiceImport(t *testing.T) {
	row := func(r int, number string) domain.ImportRow {
		return domain.ImportRow{Row: r, CreateDTO: domain.CreateDTO{
			MitraID: "m1", Number: number, Date: "2026-09-01",
			Lines: []domain.LineDTO{{ProductName: "A", Quantity: 3, UnitPrice: 1000}},
		}}
	}
	repo := &fakePurchaseInvoiceRepo{mitraExists: true}
	svc := &PurchaseInvoiceService{repo: repo, activation: &fakeTxnLimiter{capacity: 10}}
	if created, err := svc.Import("c1", "actor", []domain.ImportRow{row(2, "B/1"), row(3, "B/2")}); err != nil || len(created) != 2 || repo.imported[0].Calc.GrandTotal != 3000 {
		t.Fatalf("want 2 invoices of 3000, got %v, %v", created, err)
	}
	if _, err := svc.Import("c1", "actor", []domain.ImportRow{row(2, "B/1"), row(7, "b/1")}); err == nil || err.Error() != "Row 7: invoice no. b/1 is also used on row 2" {
		t.Fatalf("got %v", err)
	}
	svc = &PurchaseInvoiceService{repo: &fakePurchaseInvoiceRepo{mitraExists: true}, activation: &fakeTxnLimiter{capacity: 1}}
	if _, err := svc.Import("c1", "actor", []domain.ImportRow{row(2, ""), row(3, "")}); err == nil {
		t.Fatal("want the monthly cap to refuse the whole file")
	}
}

func (f *fakePurchaseInvoiceRepo) Create(dto *domain.CreateDTO, calc *utils.LinesCalc, actorID string) (*model.PurchaseInvoice, error) {
	return f.invoice, nil
}
func (f *fakePurchaseInvoiceRepo) Update(companyID, id string, dto *domain.UpdateDTO, calc *utils.LinesCalc, actorID string) (*model.PurchaseInvoice, error) {
	return f.invoice, nil
}
func (f *fakePurchaseInvoiceRepo) FindByID(companyID, id string) (*model.PurchaseInvoice, error) {
	if f.invoice == nil {
		return nil, &domain.ErrNotFound{ID: id}
	}
	return f.invoice, nil
}
func (f *fakePurchaseInvoiceRepo) FindAll(fl *domain.Filter) (*utils.OffsetPaginationResult, error) {
	return nil, nil
}
func (f *fakePurchaseInvoiceRepo) Delete(companyID, id string) error { return nil }
func (f *fakePurchaseInvoiceRepo) SetTemplate(companyID, id, actorID, template string) error {
	return nil
}
func (f *fakePurchaseInvoiceRepo) Summary(companyID string) (*domain.Summary, error) {
	return &domain.Summary{}, nil
}
func (f *fakePurchaseInvoiceRepo) SetStatus(companyID, id, actorID string, status model.PurchaseInvoiceStatus) error {
	if f.invoice != nil {
		f.invoice.Status = status
	}
	return nil
}
func (f *fakePurchaseInvoiceRepo) MitraExists(companyID, mitraID string) (bool, error) {
	return f.mitraExists, nil
}
func (f *fakePurchaseInvoiceRepo) TaxRates(companyID string, taxIDs []string) (map[string]utils.TaxRate, error) {
	return f.taxRates, nil
}

func piLine(product string, qty, price float64) domain.LineDTO {
	return domain.LineDTO{ProductName: product, Quantity: qty, UnitPrice: price}
}

func TestPurchaseInvoiceStatusTransitions(t *testing.T) {
	newInvoice := func(status model.PurchaseInvoiceStatus) *model.PurchaseInvoice {
		return &model.PurchaseInvoice{
			ID: "bill-1", Status: status,
			Lines: []model.PurchaseInvoiceLine{{ProductName: "A", Quantity: 1, UnitPrice: 1000}},
		}
	}

	t.Run("confirm from draft succeeds", func(t *testing.T) {
		repo := &fakePurchaseInvoiceRepo{invoice: newInvoice(model.PurchaseInvoiceStatusDraft)}
		svc := &PurchaseInvoiceService{repo: repo}
		updated, err := svc.Confirm("c1", "actor", "bill-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Status != model.PurchaseInvoiceStatusConfirmed {
			t.Errorf("expected confirmed, got %s", updated.Status)
		}
	})

	t.Run("confirm from confirmed rejected", func(t *testing.T) {
		repo := &fakePurchaseInvoiceRepo{invoice: newInvoice(model.PurchaseInvoiceStatusConfirmed)}
		svc := &PurchaseInvoiceService{repo: repo}
		_, err := svc.Confirm("c1", "actor", "bill-1")
		var it *domain.ErrInvalidTransition
		if !errors.As(err, &it) {
			t.Fatalf("expected ErrInvalidTransition, got %v", err)
		}
	})

	t.Run("back to draft from confirmed succeeds", func(t *testing.T) {
		repo := &fakePurchaseInvoiceRepo{invoice: newInvoice(model.PurchaseInvoiceStatusConfirmed)}
		svc := &PurchaseInvoiceService{repo: repo}
		updated, err := svc.BackToDraft("c1", "actor", "bill-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Status != model.PurchaseInvoiceStatusDraft {
			t.Errorf("expected draft, got %s", updated.Status)
		}
	})

	t.Run("cancel from draft succeeds", func(t *testing.T) {
		repo := &fakePurchaseInvoiceRepo{invoice: newInvoice(model.PurchaseInvoiceStatusDraft)}
		svc := &PurchaseInvoiceService{repo: repo}
		updated, err := svc.Cancel("c1", "actor", "bill-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Status != model.PurchaseInvoiceStatusCancelled {
			t.Errorf("expected cancelled, got %s", updated.Status)
		}
	})

	t.Run("cancel twice rejected", func(t *testing.T) {
		repo := &fakePurchaseInvoiceRepo{invoice: newInvoice(model.PurchaseInvoiceStatusCancelled)}
		svc := &PurchaseInvoiceService{repo: repo}
		_, err := svc.Cancel("c1", "actor", "bill-1")
		var it *domain.ErrInvalidTransition
		if !errors.As(err, &it) {
			t.Fatalf("expected ErrInvalidTransition, got %v", err)
		}
	})

	t.Run("update allowed once confirmed", func(t *testing.T) {
		repo := &fakePurchaseInvoiceRepo{invoice: newInvoice(model.PurchaseInvoiceStatusConfirmed), mitraExists: true}
		svc := &PurchaseInvoiceService{repo: repo}
		_, err := svc.Update("c1", "actor", "bill-1", &domain.UpdateDTO{
			MitraID: "m1", Date: "2026-01-01",
			Lines: []domain.LineDTO{piLine("A", 1, 1000)},
		})
		if err != nil {
			t.Fatalf("status must not make a document immutable, got %v", err)
		}
	})

	t.Run("delete allowed once confirmed", func(t *testing.T) {
		repo := &fakePurchaseInvoiceRepo{invoice: newInvoice(model.PurchaseInvoiceStatusConfirmed)}
		svc := &PurchaseInvoiceService{repo: repo}
		err := svc.Delete("c1", "bill-1")
		if err != nil {
			t.Fatalf("status must not make a document immutable, got %v", err)
		}
	})
}

func (f *fakePurchaseInvoiceRepo) PreviewNumber(companyID string) (string, error) {
	return "BILL/2026/0001", nil
}

func (f *fakePurchaseInvoiceRepo) CountAll(companyID string) (int64, error) { return 0, nil }
func (f *fakePurchaseInvoiceRepo) CountCreatedSince(companyID string, since time.Time) (int64, error) {
	return 0, nil
}
