package service

import (
	"errors"
	"testing"

	domain "duluin_invoice/app/domain/purchaseorder"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// fakePurchaseOrderRepo — a small hand-written stand-in for domain.IRepository
// (same style as fakeSalesOrderRepo).
type fakePurchaseOrderRepo struct {
	order       *model.PurchaseOrder
	taxRates    map[string]domain.TaxInfo
	mitraExists bool
}

func (f *fakePurchaseOrderRepo) Create(dto *domain.CreateDTO, calc *domain.OrderCalc, actorID string) (*model.PurchaseOrder, error) {
	return f.order, nil
}
func (f *fakePurchaseOrderRepo) Update(companyID, id string, dto *domain.UpdateDTO, calc *domain.OrderCalc, actorID string) (*model.PurchaseOrder, error) {
	return f.order, nil
}
func (f *fakePurchaseOrderRepo) FindByID(companyID, id string) (*model.PurchaseOrder, error) {
	if f.order == nil {
		return nil, &domain.ErrNotFound{ID: id}
	}
	return f.order, nil
}
func (f *fakePurchaseOrderRepo) FindAll(fl *domain.Filter) (*utils.OffsetPaginationResult, error) {
	return nil, nil
}
func (f *fakePurchaseOrderRepo) Delete(companyID, id string) error { return nil }
func (f *fakePurchaseOrderRepo) SetStatus(companyID, id, actorID string, status model.PurchaseOrderStatus) error {
	if f.order != nil {
		f.order.Status = status
	}
	return nil
}
func (f *fakePurchaseOrderRepo) MitraExists(companyID, mitraID string) (bool, error) {
	return f.mitraExists, nil
}
func (f *fakePurchaseOrderRepo) TaxRates(companyID string, taxIDs []string) (map[string]domain.TaxInfo, error) {
	return f.taxRates, nil
}

// poLine builds a line DTO for the status-transition tests below. The
// tax-math cases themselves live in utils/linecalc_test.go, since
// purchase_order.service.go delegates to the shared utils.CalcLines.
func poLine(product string, qty, price, discountPct float64, taxIDs ...string) domain.LineDTO {
	return domain.LineDTO{
		ProductName: product, Quantity: qty, UnitPrice: price,
		DiscountType: "percent", DiscountValue: discountPct, TaxIDs: taxIDs,
	}
}

func TestPurchaseOrderStatusTransitions(t *testing.T) {
	newOrder := func(status model.PurchaseOrderStatus) *model.PurchaseOrder {
		return &model.PurchaseOrder{
			ID: "po-1", Status: status,
			Lines: []model.PurchaseOrderLine{{ProductName: "A", Quantity: 1, UnitPrice: 1000}},
		}
	}

	t.Run("confirm from draft succeeds", func(t *testing.T) {
		repo := &fakePurchaseOrderRepo{order: newOrder(model.PurchaseOrderStatusDraft)}
		svc := &PurchaseOrderService{repo: repo}
		updated, err := svc.Confirm("c1", "actor", "po-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Status != model.PurchaseOrderStatusConfirmed {
			t.Errorf("expected confirmed, got %s", updated.Status)
		}
	})

	t.Run("confirm from confirmed rejected", func(t *testing.T) {
		repo := &fakePurchaseOrderRepo{order: newOrder(model.PurchaseOrderStatusConfirmed)}
		svc := &PurchaseOrderService{repo: repo}
		_, err := svc.Confirm("c1", "actor", "po-1")
		var it *domain.ErrInvalidTransition
		if !errors.As(err, &it) {
			t.Fatalf("expected ErrInvalidTransition, got %v", err)
		}
	})

	t.Run("back to draft from confirmed succeeds", func(t *testing.T) {
		repo := &fakePurchaseOrderRepo{order: newOrder(model.PurchaseOrderStatusConfirmed)}
		svc := &PurchaseOrderService{repo: repo}
		updated, err := svc.BackToDraft("c1", "actor", "po-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Status != model.PurchaseOrderStatusDraft {
			t.Errorf("expected draft, got %s", updated.Status)
		}
	})

	t.Run("cancel from draft succeeds", func(t *testing.T) {
		repo := &fakePurchaseOrderRepo{order: newOrder(model.PurchaseOrderStatusDraft)}
		svc := &PurchaseOrderService{repo: repo}
		updated, err := svc.Cancel("c1", "actor", "po-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Status != model.PurchaseOrderStatusCancelled {
			t.Errorf("expected cancelled, got %s", updated.Status)
		}
	})

	t.Run("cancel twice rejected", func(t *testing.T) {
		repo := &fakePurchaseOrderRepo{order: newOrder(model.PurchaseOrderStatusCancelled)}
		svc := &PurchaseOrderService{repo: repo}
		_, err := svc.Cancel("c1", "actor", "po-1")
		var it *domain.ErrInvalidTransition
		if !errors.As(err, &it) {
			t.Fatalf("expected ErrInvalidTransition, got %v", err)
		}
	})

	t.Run("update rejected once confirmed", func(t *testing.T) {
		repo := &fakePurchaseOrderRepo{order: newOrder(model.PurchaseOrderStatusConfirmed), mitraExists: true}
		svc := &PurchaseOrderService{repo: repo}
		_, err := svc.Update("c1", "actor", "po-1", &domain.UpdateDTO{
			MitraID: "m1", Date: "2026-01-01",
			Lines: []domain.LineDTO{poLine("A", 1, 1000, 0)},
		})
		var ne *domain.ErrNotEditable
		if !errors.As(err, &ne) {
			t.Fatalf("expected ErrNotEditable, got %v", err)
		}
	})

	t.Run("delete rejected once confirmed", func(t *testing.T) {
		repo := &fakePurchaseOrderRepo{order: newOrder(model.PurchaseOrderStatusConfirmed)}
		svc := &PurchaseOrderService{repo: repo}
		err := svc.Delete("c1", "po-1")
		var ne *domain.ErrNotEditable
		if !errors.As(err, &ne) {
			t.Fatalf("expected ErrNotEditable, got %v", err)
		}
	})
}

func (f *fakePurchaseOrderRepo) PreviewNumber(companyID string) (string, error) {
	return "PO/2026/0001", nil
}
