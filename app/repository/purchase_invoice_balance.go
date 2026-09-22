package repository

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"duluin_invoice/app/model"
)

// ErrPurchaseInvoiceNotConfirmed — only a confirmed bill can be paid.
type ErrPurchaseInvoiceNotConfirmed struct{}

func (e *ErrPurchaseInvoiceNotConfirmed) Error() string {
	return "only a confirmed purchase invoice can receive payments"
}

// ErrPurchaseInvoiceExceedsBalance — the payment would push PaidAmount past GrandTotal.
type ErrPurchaseInvoiceExceedsBalance struct{ Remaining float64 }

func (e *ErrPurchaseInvoiceExceedsBalance) Error() string {
	return fmt.Sprintf("amount exceeds the outstanding balance (%.2f)", e.Remaining)
}

// ErrPurchaseInvoiceMismatch — the invoice is missing or belongs to another partner.
type ErrPurchaseInvoiceMismatch struct{ Message string }

func (e *ErrPurchaseInvoiceMismatch) Error() string { return e.Message }

// purchasePaymentStatus derives the status from the two numbers (the same rule as sales):
// Paid when PaidAmount >= GrandTotal, Partially Paid when 0 < PaidAmount < GrandTotal, else Unpaid.
func purchasePaymentStatus(paid, total float64) model.PurchaseInvoicePaymentStatus {
	switch {
	case total > 0 && paid >= total:
		return model.PurchaseInvoicePaymentPaid
	case paid > 0:
		return model.PurchaseInvoicePaymentPartiallyPaid
	default:
		return model.PurchaseInvoicePaymentUnpaid
	}
}

func lockPurchaseInvoice(tx *gorm.DB, companyID, invoiceID string) (*model.PurchaseInvoice, error) {
	var invoice model.PurchaseInvoice
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND company_id = ?", invoiceID, companyID).
		First(&invoice).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &ErrPurchaseInvoiceMismatch{Message: "purchase invoice not found"}
		}
		return nil, fmt.Errorf("lock purchase invoice %s: %w", invoiceID, err)
	}
	return &invoice, nil
}

// applyPurchaseInvoicePayment locks the invoice (inside an existing transaction), checks it belongs
// to the partner and is confirmed, that the amount fits the outstanding balance, then adds it to
// PaidAmount and re-derives PaymentStatus. The one place a purchase invoice's balance goes up.
func applyPurchaseInvoicePayment(tx *gorm.DB, companyID, invoiceID, mitraID string, amount float64) error {
	invoice, err := lockPurchaseInvoice(tx, companyID, invoiceID)
	if err != nil {
		return err
	}
	if invoice.MitraID != mitraID {
		return &ErrPurchaseInvoiceMismatch{Message: "purchase invoice does not belong to the selected partner"}
	}
	if invoice.Status != model.PurchaseInvoiceStatusConfirmed {
		return &ErrPurchaseInvoiceNotConfirmed{}
	}
	remaining := round2(invoice.GrandTotal - invoice.PaidAmount)
	if amount > remaining {
		return &ErrPurchaseInvoiceExceedsBalance{Remaining: remaining}
	}
	paid := round2(invoice.PaidAmount + amount)
	return tx.Model(&model.PurchaseInvoice{}).Where("id = ?", invoice.ID).Updates(map[string]interface{}{
		"paid_amount":    paid,
		"payment_status": purchasePaymentStatus(paid, invoice.GrandTotal),
	}).Error
}

// reversePurchaseInvoicePayment undoes an earlier payment (receipt edited or deleted): subtracts the
// amount (never below 0) and re-derives the status. Works on soft-deleted invoices too.
func reversePurchaseInvoicePayment(tx *gorm.DB, companyID, invoiceID string, amount float64) error {
	var invoice model.PurchaseInvoice
	if err := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND company_id = ?", invoiceID, companyID).
		First(&invoice).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return fmt.Errorf("lock purchase invoice %s: %w", invoiceID, err)
	}
	paid := round2(invoice.PaidAmount - amount)
	if paid < 0 {
		paid = 0
	}
	return tx.Unscoped().Model(&model.PurchaseInvoice{}).Where("id = ?", invoice.ID).Updates(map[string]interface{}{
		"paid_amount":    paid,
		"payment_status": purchasePaymentStatus(paid, invoice.GrandTotal),
	}).Error
}

// refreshPurchaseInvoicePaymentStatus — same as the sales version: after an edit the total may
// differ from what was paid, so the status is derived again from paid_amount vs the new total.
func refreshPurchaseInvoicePaymentStatus(tx *gorm.DB, companyID, invoiceID string) error {
	invoice, err := lockPurchaseInvoice(tx, companyID, invoiceID)
	if err != nil {
		return err
	}
	return tx.Model(&model.PurchaseInvoice{}).Where("id = ?", invoice.ID).
		Update("payment_status", purchasePaymentStatus(invoice.PaidAmount, invoice.GrandTotal)).Error
}
