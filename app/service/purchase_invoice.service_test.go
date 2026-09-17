package service

import (
	"errors"
	"testing"

	domain "duluin_invoice/app/domain/purchaseinvoice"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// fakePurchaseInvoiceRepo — a small hand-written stand-in for
// domain.IRepository (same style as fakeSalesInvoiceRepo).
type fakePurchaseInvoiceRepo struct {
	invoice     *model.PurchaseInvoice
	taxRates    map[string]utils.TaxRate
	mitraExists bool
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

	t.Run("update rejected once confirmed", func(t *testing.T) {
		repo := &fakePurchaseInvoiceRepo{invoice: newInvoice(model.PurchaseInvoiceStatusConfirmed), mitraExists: true}
		svc := &PurchaseInvoiceService{repo: repo}
		_, err := svc.Update("c1", "actor", "bill-1", &domain.UpdateDTO{
			MitraID: "m1", Date: "2026-01-01",
			Lines: []domain.LineDTO{piLine("A", 1, 1000)},
		})
		var ne *domain.ErrNotEditable
		if !errors.As(err, &ne) {
			t.Fatalf("expected ErrNotEditable, got %v", err)
		}
	})

	t.Run("delete rejected once confirmed", func(t *testing.T) {
		repo := &fakePurchaseInvoiceRepo{invoice: newInvoice(model.PurchaseInvoiceStatusConfirmed)}
		svc := &PurchaseInvoiceService{repo: repo}
		err := svc.Delete("c1", "bill-1")
		var ne *domain.ErrNotEditable
		if !errors.As(err, &ne) {
			t.Fatalf("expected ErrNotEditable, got %v", err)
		}
	})
}

func (f *fakePurchaseInvoiceRepo) PreviewNumber(companyID string) (string, error) {
	return "BILL/2026/0001", nil
}
