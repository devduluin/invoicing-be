package repository

import (
	"errors"
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// deleted (or foreign) invoice: it can't take new payments
			return &ErrInvoiceNotConfirmed{}
		}
		return fmt.Errorf("lock invoice %s: %w", invoiceID, err)
	}
	if invoice.Status != model.SalesInvoiceStatusConfirmed {
		return &ErrInvoiceNotConfirmed{}
	}
	remaining, _ := model.SalesBalance(invoice.GrandTotal, invoice.AppliedDPAmount, invoice.PaidAmount)
	if amount > remaining+0.005 {
		return &ErrInvoiceExceedsBalance{Remaining: remaining}
	}
	paidAmount := round2(invoice.PaidAmount + amount)
	_, status := model.SalesBalance(invoice.GrandTotal, invoice.AppliedDPAmount, paidAmount)
	return tx.Model(&invoice).Updates(map[string]interface{}{
		"paid_amount":    paidAmount,
		"payment_status": status,
	}).Error
}

// reverseSalesInvoicePayment undoes a previous allocation: subtracts amount
// from PaidAmount (never below 0) and re-derives PaymentStatus. Used when a
// receipt is edited or soft-deleted. The invoice row is locked; run inside a
// transaction and lock multiple invoices in sorted-ID order.
func reverseSalesInvoicePayment(tx *gorm.DB, companyID, invoiceID string, amount float64) error {
	var invoice model.SalesInvoice
	if err := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND company_id = ?", invoiceID, companyID).
		First(&invoice).Error; err != nil {
		return fmt.Errorf("lock invoice %s: %w", invoiceID, err)
	}
	paidAmount := round2(invoice.PaidAmount - amount)
	if paidAmount < 0 {
		paidAmount = 0
	}
	_, status := model.SalesBalance(invoice.GrandTotal, invoice.AppliedDPAmount, paidAmount)
	return tx.Unscoped().Model(&model.SalesInvoice{}).Where("id = ?", invoice.ID).Updates(map[string]interface{}{
		"paid_amount":    paidAmount,
		"payment_status": status,
	}).Error
}

// syncSalesInvoiceBalance is the single place an invoice's derived balance is rebuilt: it locks the
// invoice, re-sums the CONFIRMED, non-deleted down payments linked to it (a draft or cancelled DP
// never counts, a deleted one drops out) into applied_dp_amount, and re-derives payment_status from
// Total - Applied DP - Paid. Call it inside the transaction after anything that can change the total,
// a linked DP, or that DP's status. Idempotent, so it can never double count.
func syncSalesInvoiceBalance(tx *gorm.DB, companyID, invoiceID string) error {
	var invoice model.SalesInvoice
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND company_id = ?", invoiceID, companyID).
		First(&invoice).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // deleted invoice: nothing to keep in sync
		}
		return fmt.Errorf("lock invoice %s: %w", invoiceID, err)
	}
	var applied float64
	if err := tx.Model(&model.SalesInvoice{}).
		Where("company_id = ? AND kind = ? AND status = ? AND linked_invoice_id = ?",
			companyID, model.SalesInvoiceKindDownPayment, model.SalesInvoiceStatusConfirmed, invoiceID).
		Select("COALESCE(SUM(grand_total), 0)").Scan(&applied).Error; err != nil {
		return fmt.Errorf("sum down payments of %s: %w", invoiceID, err)
	}
	applied = round2(applied)
	_, status := model.SalesBalance(invoice.GrandTotal, applied, invoice.PaidAmount)
	return tx.Model(&model.SalesInvoice{}).Where("id = ?", invoice.ID).Updates(map[string]interface{}{
		"applied_dp_amount": applied,
		"payment_status":    status,
	}).Error
}

// outstandingExcludingDP — what the invoice still owes if the given down payment were NOT applied.
// A DP may only be confirmed/created for at most this much.
func outstandingExcludingDP(tx *gorm.DB, companyID, invoiceID, dpID string) (float64, error) {
	var invoice model.SalesInvoice
	if err := tx.Where("id = ? AND company_id = ?", invoiceID, companyID).First(&invoice).Error; err != nil {
		return 0, err
	}
	var others float64
	q := tx.Model(&model.SalesInvoice{}).
		Where("company_id = ? AND kind = ? AND status = ? AND linked_invoice_id = ?",
			companyID, model.SalesInvoiceKindDownPayment, model.SalesInvoiceStatusConfirmed, invoiceID)
	if dpID != "" {
		q = q.Where("id <> ?", dpID)
	}
	if err := q.Select("COALESCE(SUM(grand_total), 0)").Scan(&others).Error; err != nil {
		return 0, err
	}
	out, _ := model.SalesBalance(invoice.GrandTotal, round2(others), invoice.PaidAmount)
	return out, nil
}
