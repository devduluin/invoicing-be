package service

import (
	"errors"
	"testing"

	domain "duluin_invoice/app/domain/salesinvoice"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// fakeSalesInvoiceRepo — a small hand-written stand-in for domain.IRepository
// (same style as fakeSalesOrderRepo/fakeReportRepo).
type fakeSalesInvoiceRepo struct {
	invoice     *model.SalesInvoice
	taxRates    map[string]utils.TaxRate
	mitraExists bool
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

	t.Run("update rejected once confirmed", func(t *testing.T) {
		repo := &fakeSalesInvoiceRepo{invoice: newInvoice(model.SalesInvoiceStatusConfirmed), mitraExists: true}
		svc := &SalesInvoiceService{repo: repo}
		_, err := svc.Update("c1", "actor", "inv-1", &domain.UpdateDTO{
			MitraID: "m1", Date: "2026-01-01",
			Lines: []domain.LineDTO{siLine("A", 1, 1000)},
		})
		var ne *domain.ErrNotEditable
		if !errors.As(err, &ne) {
			t.Fatalf("expected ErrNotEditable, got %v", err)
		}
	})

	t.Run("delete rejected once confirmed", func(t *testing.T) {
		repo := &fakeSalesInvoiceRepo{invoice: newInvoice(model.SalesInvoiceStatusConfirmed)}
		svc := &SalesInvoiceService{repo: repo}
		err := svc.Delete("c1", "inv-1")
		var ne *domain.ErrNotEditable
		if !errors.As(err, &ne) {
			t.Fatalf("expected ErrNotEditable, got %v", err)
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
