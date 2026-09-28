package repository

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/salesinvoice"
	"duluin_invoice/app/model"
)

const amountEpsilon = 0.005

// checkSalesInvoiceLinks enforces the amount rules that tie a document to its source, inside the
// caller's transaction. selfID is "" on create. It never mutates anything.
//
//   - Down payment linked to an invoice: at most that invoice's outstanding, not counting this DP
//     itself (a draft DP does not reduce anything, so it can't be counted against itself).
//   - A sales order's own total is NOT a cap on what can be created from it: a confirmed order can
//     be invoiced (or drawn on for down payments) any number of times for any amount, same as the
//     reference product (Paper.id) — the order is a record of what was agreed, not a running
//     invoicing budget. Only the "does this order actually exist" check remains, so a document can
//     never reference a deleted/foreign order.
func checkSalesInvoiceLinks(tx *gorm.DB, companyID, selfID string, kind model.SalesInvoiceKind, salesOrderID, linkedInvoiceID *string, total float64) error {
	if kind == model.SalesInvoiceKindDownPayment && linkedInvoiceID != nil {
		var target model.SalesInvoice
		err := tx.Where("id = ? AND company_id = ?", *linkedInvoiceID, companyID).First(&target).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &domain.ErrValidation{Message: "the linked invoice was not found"}
		}
		if err != nil {
			return fmt.Errorf("find linked invoice: %w", err)
		}
		if target.Kind != model.SalesInvoiceKindInvoice {
			return &domain.ErrValidation{Message: "a down payment can only be linked to a sales invoice"}
		}
		avail, err := outstandingExcludingDP(tx, companyID, target.ID, selfID)
		if err != nil {
			return fmt.Errorf("outstanding of linked invoice: %w", err)
		}
		if total > avail+amountEpsilon {
			return &domain.ErrValidation{Message: fmt.Sprintf("down payment (%.2f) exceeds the outstanding of the linked invoice (%.2f)", total, avail)}
		}
		return nil
	}
	if salesOrderID == nil {
		return nil
	}
	var exists int64
	if err := tx.Table("sales_orders").Where("id = ? AND company_id = ? AND deleted_at IS NULL", *salesOrderID, companyID).Count(&exists).Error; err != nil {
		return fmt.Errorf("find sales order: %w", err)
	}
	if exists == 0 {
		return &domain.ErrValidation{Message: "the sales order was not found"}
	}
	return nil
}

// adoptSalesOrderDownPayments hands the down payments made straight from a sales order (no invoice
// yet) to the invoice now being made from that same order, so they start reducing its outstanding.
// Only unlinked DPs move; already-linked ones stay put, so a DP is never applied to two invoices.
func adoptSalesOrderDownPayments(tx *gorm.DB, companyID, invoiceID string, salesOrderID *string) error {
	if salesOrderID == nil {
		return nil
	}
	if err := tx.Model(&model.SalesInvoice{}).
		Where("company_id = ? AND kind = ? AND sales_order_id = ? AND linked_invoice_id IS NULL", companyID, model.SalesInvoiceKindDownPayment, *salesOrderID).
		Update("linked_invoice_id", invoiceID).Error; err != nil {
		return fmt.Errorf("adopt sales order down payments: %w", err)
	}
	return nil
}

// invoiceOfOrder — the invoice a down payment made from a sales order belongs to when one already
// exists (order → invoice → down payment): the first non-cancelled regular invoice of that order,
// the same one adoptSalesOrderDownPayments would have picked had the DP come first. Nil when none.
func invoiceOfOrder(tx *gorm.DB, companyID string, salesOrderID *string) (*string, error) {
	if salesOrderID == nil {
		return nil, nil
	}
	var ids []string
	err := tx.Model(&model.SalesInvoice{}).
		Where("company_id = ? AND sales_order_id = ? AND kind = ? AND status <> ?", companyID, *salesOrderID, model.SalesInvoiceKindInvoice, model.SalesInvoiceStatusCancelled).
		Order("created_at ASC").Limit(1).Pluck("id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("find invoice of sales order: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return &ids[0], nil
}

// adoptReferencedDownPayment — an invoice made FROM a down payment ("Create Invoice" on the DP's
// page) carries that DP's id in linked_invoice_id. Point the DP back at the new invoice (only if it
// is not already applied elsewhere) so it starts reducing this invoice's outstanding.
func adoptReferencedDownPayment(tx *gorm.DB, companyID, invoiceID string, referenced *string) error {
	if referenced == nil {
		return nil
	}
	if err := tx.Model(&model.SalesInvoice{}).
		Where("id = ? AND company_id = ? AND kind = ? AND linked_invoice_id IS NULL", *referenced, companyID, model.SalesInvoiceKindDownPayment).
		Update("linked_invoice_id", invoiceID).Error; err != nil {
		return fmt.Errorf("adopt referenced down payment: %w", err)
	}
	return nil
}
