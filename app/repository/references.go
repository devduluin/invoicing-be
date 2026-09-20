package repository

import (
	"fmt"

	"gorm.io/gorm"

	"duluin_invoice/utils"
)

// Reference checks for delete. Every table has soft delete, and a document's
// lines are deliberately KEPT when the document is deleted (so it stays
// auditable/restorable) — so "is this still used?" must only count references
// held by documents that are themselves not deleted. Each query therefore joins
// through to the live parent.

type refQuery struct {
	label string
	sql   string
	args  []any
}

// countRefs sums the given queries and returns a human "12 sales invoices, 3 …"
// style summary of the non-zero ones.
func countRefs(db *gorm.DB, queries []refQuery) (int64, string, error) {
	var total int64
	summary := ""
	for _, q := range queries {
		var n int64
		if err := db.Raw(q.sql, q.args...).Scan(&n).Error; err != nil {
			return 0, "", fmt.Errorf("reference check (%s): %w", q.label, err)
		}
		if n == 0 {
			continue
		}
		total += n
		if summary != "" {
			summary += ", "
		}
		summary += fmt.Sprintf("%d %s", n, q.label)
	}
	return total, summary, nil
}

func inUse(what, summary string) error {
	return &utils.ErrInUse{Message: fmt.Sprintf("This %s is still used by %s — remove or change those first.", what, summary)}
}

// simple = documents that hold the column directly on their own (non-deleted) row.
func directRefs(table, column, label, companyID, id string) refQuery {
	return refQuery{
		label: label,
		sql:   fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = ? AND company_id = ? AND deleted_at IS NULL", table, column),
		args:  []any{id, companyID},
	}
}

func journalLineRefs(column, label, companyID, id string) refQuery {
	return refQuery{
		label: label,
		sql: fmt.Sprintf(`SELECT COUNT(*) FROM journal_lines jl
			JOIN journal_entries je ON je.id = jl.journal_entry_id
			WHERE jl.%s = ? AND je.company_id = ? AND je.deleted_at IS NULL`, column),
		args: []any{id, companyID},
	}
}

func checkMitraUnused(db *gorm.DB, companyID, id string) error {
	total, summary, err := countRefs(db, []refQuery{
		directRefs("sales_orders", "mitra_id", "sales order(s)", companyID, id),
		directRefs("sales_invoices", "mitra_id", "sales invoice(s)", companyID, id),
		directRefs("sales_receipts", "mitra_id", "sales receipt(s)", companyID, id),
		directRefs("sales_payments", "mitra_id", "sales payment(s)", companyID, id),
		directRefs("purchase_orders", "mitra_id", "purchase order(s)", companyID, id),
		directRefs("purchase_invoices", "mitra_id", "purchase invoice(s)", companyID, id),
		directRefs("purchase_receipts", "mitra_id", "purchase receipt(s)", companyID, id),
		directRefs("delivery_notes", "mitra_id", "delivery note(s)", companyID, id),
		directRefs("goods_receipts", "mitra_id", "goods receipt(s)", companyID, id),
		journalLineRefs("mitra_id", "journal line(s)", companyID, id),
	})
	if err != nil {
		return err
	}
	if total > 0 {
		return inUse("partner", summary)
	}
	return nil
}

func checkBankAccountUnused(db *gorm.DB, companyID, id string) error {
	total, summary, err := countRefs(db, []refQuery{
		directRefs("sales_receipts", "bank_account_id", "sales receipt(s)", companyID, id),
		directRefs("sales_payments", "bank_account_id", "sales payment(s)", companyID, id),
		directRefs("purchase_receipts", "bank_account_id", "purchase receipt(s)", companyID, id),
	})
	if err != nil {
		return err
	}
	if total > 0 {
		return inUse("bank account", summary)
	}
	return nil
}

func checkAccountUnused(db *gorm.DB, companyID, id string) error {
	total, summary, err := countRefs(db, []refQuery{
		journalLineRefs("account_id", "journal line(s)", companyID, id),
		{
			label: "tax(es)",
			sql: `SELECT COUNT(*) FROM taxes WHERE company_id = ? AND deleted_at IS NULL
			      AND (sales_account_id = ? OR purchase_account_id = ?)`,
			args: []any{companyID, id, id},
		},
		{
			label: "journal book(s)",
			sql: `SELECT COUNT(*) FROM journal_books WHERE company_id = ? AND deleted_at IS NULL
			      AND (default_account_id = ? OR default_debit_account_id = ? OR default_credit_account_id = ?)`,
			args: []any{companyID, id, id, id},
		},
	})
	if err != nil {
		return err
	}
	if total > 0 {
		return inUse("account", summary)
	}
	return nil
}

// lineTaxRefs counts line-tax rows for a tax, through live parent documents only.
func lineTaxRefs(lineTaxTable, lineTable, lineFK, parentTable, parentFK, label, companyID, id string) refQuery {
	return refQuery{
		label: label,
		sql: fmt.Sprintf(`SELECT COUNT(*) FROM %s lt
			JOIN %s l ON l.id = lt.%s
			JOIN %s p ON p.id = l.%s
			WHERE lt.tax_id = ? AND p.company_id = ? AND p.deleted_at IS NULL`, lineTaxTable, lineTable, lineFK, parentTable, parentFK),
		args: []any{id, companyID},
	}
}

func checkTaxUnused(db *gorm.DB, companyID, id string) error {
	total, summary, err := countRefs(db, []refQuery{
		lineTaxRefs("sales_order_line_taxes", "sales_order_lines", "sales_order_line_id", "sales_orders", "sales_order_id", "sales order line(s)", companyID, id),
		lineTaxRefs("sales_invoice_line_taxes", "sales_invoice_lines", "sales_invoice_line_id", "sales_invoices", "sales_invoice_id", "sales invoice line(s)", companyID, id),
		lineTaxRefs("purchase_order_line_taxes", "purchase_order_lines", "purchase_order_line_id", "purchase_orders", "purchase_order_id", "purchase order line(s)", companyID, id),
		lineTaxRefs("purchase_invoice_line_taxes", "purchase_invoice_lines", "purchase_invoice_line_id", "purchase_invoices", "purchase_invoice_id", "purchase invoice line(s)", companyID, id),
	})
	if err != nil {
		return err
	}
	if total > 0 {
		return inUse("tax", summary)
	}
	return nil
}

// Units are referenced by NAME (free-text column on the delivery-note / goods-receipt lines).
func checkUnitUnused(db *gorm.DB, companyID, name string) error {
	total, summary, err := countRefs(db, []refQuery{
		{
			label: "delivery note line(s)",
			sql: `SELECT COUNT(*) FROM delivery_note_lines l JOIN delivery_notes p ON p.id = l.delivery_note_id
			      WHERE l.unit = ? AND p.company_id = ? AND p.deleted_at IS NULL`,
			args: []any{name, companyID},
		},
		{
			label: "goods receipt line(s)",
			sql: `SELECT COUNT(*) FROM goods_receipt_lines l JOIN goods_receipts p ON p.id = l.goods_receipt_id
			      WHERE l.unit = ? AND p.company_id = ? AND p.deleted_at IS NULL`,
			args: []any{name, companyID},
		},
	})
	if err != nil {
		return err
	}
	if total > 0 {
		return inUse("unit", summary)
	}
	return nil
}

// checkInvoiceHasNoPayments guards a sales invoice against being reverted to draft
// or deleted while money is applied to it (payments / receipt allocations).
func checkInvoiceHasNoPayments(db *gorm.DB, companyID, invoiceID string) error {
	total, summary, err := countRefs(db, []refQuery{
		directRefs("sales_payments", "sales_invoice_id", "payment(s)", companyID, invoiceID),
		{
			label: "receipt allocation(s)",
			sql: `SELECT COUNT(*) FROM sales_receipt_allocations a JOIN sales_receipts r ON r.id = a.sales_receipt_id
			      WHERE a.sales_invoice_id = ? AND r.company_id = ? AND r.deleted_at IS NULL`,
			args: []any{invoiceID, companyID},
		},
	})
	if err != nil {
		return err
	}
	if total > 0 {
		return &utils.ErrInUse{Message: fmt.Sprintf("This invoice already has %s applied — it can't be reverted or deleted.", summary)}
	}
	return nil
}
