package service

import (
	"errors"
	"testing"

	domain "duluin_invoice/app/domain/salesorder"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// fakeSalesOrderRepo — a small hand-written stand-in for domain.IRepository
// (same style as fakeReportRepo in report.service_test.go).
type fakeSalesOrderRepo struct {
	order       *model.SalesOrder
	taxRates    map[string]domain.TaxInfo
	mitraExists bool
}

func (f *fakeSalesOrderRepo) Create(dto *domain.CreateDTO, calc *domain.OrderCalc, actorID string) (*model.SalesOrder, error) {
	return f.order, nil
}
func (f *fakeSalesOrderRepo) Update(companyID, id string, dto *domain.UpdateDTO, calc *domain.OrderCalc, actorID string) (*model.SalesOrder, error) {
	return f.order, nil
}
func (f *fakeSalesOrderRepo) FindByID(companyID, id string) (*model.SalesOrder, error) {
	if f.order == nil {
		return nil, &domain.ErrNotFound{ID: id}
	}
	return f.order, nil
}
func (f *fakeSalesOrderRepo) FindAll(fl *domain.Filter) (*utils.OffsetPaginationResult, error) {
	return nil, nil
}
func (f *fakeSalesOrderRepo) Delete(companyID, id string) error { return nil }
func (f *fakeSalesOrderRepo) SetStatus(companyID, id, actorID string, status model.SalesOrderStatus) error {
	if f.order != nil {
		f.order.Status = status
	}
	return nil
}
func (f *fakeSalesOrderRepo) MitraExists(companyID, mitraID string) (bool, error) {
	return f.mitraExists, nil
}
func (f *fakeSalesOrderRepo) TaxRates(companyID string, taxIDs []string) (map[string]domain.TaxInfo, error) {
	return f.taxRates, nil
}

// soLine builds a line DTO for the status-transition tests below. The
// tax-math cases themselves moved to utils/linecalc_test.go, since
// sales_order.service.go now delegates to the shared utils.CalcLines.
func soLine(product string, qty, price, discountPct float64, taxIDs ...string) domain.LineDTO {
	return domain.LineDTO{
		ProductName: product, Quantity: qty, UnitPrice: price,
		DiscountType: "percent", DiscountValue: discountPct, TaxIDs: taxIDs,
	}
}

func TestSalesOrderStatusTransitions(t *testing.T) {
	newOrder := func(status model.SalesOrderStatus) *model.SalesOrder {
		return &model.SalesOrder{
			ID: "so-1", Status: status,
			Lines: []model.SalesOrderLine{{ProductName: "A", Quantity: 1, UnitPrice: 1000}},
		}
	}

	t.Run("confirm from draft succeeds", func(t *testing.T) {
		repo := &fakeSalesOrderRepo{order: newOrder(model.SalesOrderStatusDraft)}
		svc := &SalesOrderService{repo: repo}
		updated, err := svc.Confirm("c1", "actor", "so-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Status != model.SalesOrderStatusConfirmed {
			t.Errorf("expected confirmed, got %s", updated.Status)
		}
	})

	t.Run("confirm from confirmed rejected", func(t *testing.T) {
		repo := &fakeSalesOrderRepo{order: newOrder(model.SalesOrderStatusConfirmed)}
		svc := &SalesOrderService{repo: repo}
		_, err := svc.Confirm("c1", "actor", "so-1")
		var it *domain.ErrInvalidTransition
		if !errors.As(err, &it) {
			t.Fatalf("expected ErrInvalidTransition, got %v", err)
		}
	})

	t.Run("back to draft from confirmed succeeds", func(t *testing.T) {
		repo := &fakeSalesOrderRepo{order: newOrder(model.SalesOrderStatusConfirmed)}
		svc := &SalesOrderService{repo: repo}
		updated, err := svc.BackToDraft("c1", "actor", "so-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Status != model.SalesOrderStatusDraft {
			t.Errorf("expected draft, got %s", updated.Status)
		}
	})

	t.Run("cancel from draft succeeds", func(t *testing.T) {
		repo := &fakeSalesOrderRepo{order: newOrder(model.SalesOrderStatusDraft)}
		svc := &SalesOrderService{repo: repo}
		updated, err := svc.Cancel("c1", "actor", "so-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Status != model.SalesOrderStatusCancelled {
			t.Errorf("expected cancelled, got %s", updated.Status)
		}
	})

	t.Run("cancel twice rejected", func(t *testing.T) {
		repo := &fakeSalesOrderRepo{order: newOrder(model.SalesOrderStatusCancelled)}
		svc := &SalesOrderService{repo: repo}
		_, err := svc.Cancel("c1", "actor", "so-1")
		var it *domain.ErrInvalidTransition
		if !errors.As(err, &it) {
			t.Fatalf("expected ErrInvalidTransition, got %v", err)
		}
	})

	t.Run("update rejected once confirmed", func(t *testing.T) {
		repo := &fakeSalesOrderRepo{order: newOrder(model.SalesOrderStatusConfirmed), mitraExists: true}
		svc := &SalesOrderService{repo: repo}
		_, err := svc.Update("c1", "actor", "so-1", &domain.UpdateDTO{
			MitraID: "m1", Date: "2026-01-01",
			Lines: []domain.LineDTO{soLine("A", 1, 1000, 0)},
		})
		var ne *domain.ErrNotEditable
		if !errors.As(err, &ne) {
			t.Fatalf("expected ErrNotEditable, got %v", err)
		}
	})

	t.Run("delete rejected once confirmed", func(t *testing.T) {
		repo := &fakeSalesOrderRepo{order: newOrder(model.SalesOrderStatusConfirmed)}
		svc := &SalesOrderService{repo: repo}
		err := svc.Delete("c1", "so-1")
		var ne *domain.ErrNotEditable
		if !errors.As(err, &ne) {
			t.Fatalf("expected ErrNotEditable, got %v", err)
		}
	})
}

func (f *fakeSalesOrderRepo) PreviewNumber(companyID string) (string, error) {
	return "SO/2026/0001", nil
}
