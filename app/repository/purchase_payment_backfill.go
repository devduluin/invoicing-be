package repository

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// BackfillPurchasePayments recomputes PaidAmount / PaymentStatus for purchase invoices from the live
// purchase receipts linked to them. Receipts created before payment tracking existed never moved the
// invoice balance; this brings those invoices in line. It is derived data and idempotent: it only
// touches rows whose stored values differ from the receipts, so running it on every start is safe.
func BackfillPurchasePayments(db *gorm.DB) error {
	res := db.Exec(`
		UPDATE purchase_invoices pi
		SET paid_amount = s.total,
		    payment_status = CASE
		        WHEN pi.grand_total > 0 AND s.total >= pi.grand_total THEN 'paid'
		        WHEN s.total > 0 THEN 'partially_paid'
		        ELSE 'unpaid'
		    END
		FROM (
		    SELECT purchase_invoice_id, company_id, SUM(amount) AS total
		    FROM purchase_receipts
		    WHERE purchase_invoice_id IS NOT NULL AND deleted_at IS NULL
		    GROUP BY purchase_invoice_id, company_id
		) s
		WHERE pi.id = s.purchase_invoice_id AND pi.company_id = s.company_id AND pi.paid_amount <> s.total`)
	if res.Error != nil {
		return fmt.Errorf("backfill purchase payments: %w", res.Error)
	}
	if res.RowsAffected > 0 {
		log.Printf("purchase payments backfill: updated %d invoice(s)", res.RowsAffected)
	}
	return nil
}
