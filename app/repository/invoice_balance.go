package repository

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"duluin_invoice/app/model"
)

// ErrInvoiceNotConfirmed — the invoice isn't confirmed, so it can't receive
// a payment/receipt allocation.
type ErrInvoiceNotConfirmed struct{}

func (e *ErrInvoiceNotConfirmed) Error() string {
	return "only a confirmed invoice can receive payments"
}

// ErrInvoiceExceedsBalance — the amount would push PaidAmount past
// GrandTotal.
type ErrInvoiceExceedsBalance struct{ Remaining float64 }

func (e *ErrInvoiceExceedsBalance) Error() string {
	return fmt.Sprintf("amount exceeds the remaining balance (%.2f)", e.Remaining)
}

// applySalesInvoicePayment locks the invoice row (must run inside an
// existing transaction), validates it's confirmed and the amount doesn't
// exceed the remaining balance, then adds amount to PaidAmount and derives
// PaymentStatus. Shared by SalesPaymentRepository.Verify and
// SalesReceiptRepository.Create — the only two paths that move an invoice's
// balance — so the lock/validate/update logic exists in exactly one place.
//
// Callers touching multiple invoices in one transaction (SalesReceipt.Create)
// MUST lock them in a consistent order (sort by invoice ID) to avoid
// deadlocking against a concurrent caller doing the same.
func applySalesInvoicePayment(tx *gorm.DB, companyID, invoiceID string, amount float64) error {
	var invoice model.SalesInvoice
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND company_id = ?", invoiceID, companyID).
		First(&invoice).Error; err != nil {
		return fmt.Errorf("lock invoice %s: %w", invoiceID, err)
	}
	if invoice.Status != model.SalesInvoiceStatusConfirmed {
		return &ErrInvoiceNotConfirmed{}
	}
	remaining := round2(invoice.GrandTotal - invoice.PaidAmount)
	if amount > remaining {
		return &ErrInvoiceExceedsBalance{Remaining: remaining}
	}
	paidAmount := round2(invoice.PaidAmount + amount)
	status := model.SalesInvoicePaymentUnpaid
	switch {
	case paidAmount >= invoice.GrandTotal:
		status = model.SalesInvoicePaymentPaid
	case paidAmount > 0:
		status = model.SalesInvoicePaymentPartiallyPaid
	}
	return tx.Model(&invoice).Updates(map[string]interface{}{
		"paid_amount":    paidAmount,
		"payment_status": status,
	}).Error
}
