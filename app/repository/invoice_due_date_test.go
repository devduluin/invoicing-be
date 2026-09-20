package repository

import (
	"errors"
	"testing"

	purchase "duluin_invoice/app/domain/purchaseinvoice"
	sales "duluin_invoice/app/domain/salesinvoice"
)

// The due-date check runs right after the dates are parsed, before any database
// access, so a repository without a DB is enough to exercise it.

func TestSalesInvoiceCreate_RejectsDueDateBeforeInvoiceDate(t *testing.T) {
	r := &SalesInvoiceRepository{}
	_, err := r.Create(&sales.CreateDTO{Date: "2026-09-20", DueDate: "2026-09-19"}, nil, "u")
	var v *sales.ErrValidation
	if !errors.As(err, &v) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestPurchaseInvoiceCreate_RejectsDueDateBeforeInvoiceDate(t *testing.T) {
	r := &PurchaseInvoiceRepository{}
	_, err := r.Create(&purchase.CreateDTO{Date: "2026-09-20", DueDate: "2026-09-19"}, nil, "u")
	var v *purchase.ErrValidation
	if !errors.As(err, &v) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}
