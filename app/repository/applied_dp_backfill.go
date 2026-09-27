package repository

import (
	"errors"
	"fmt"
	"log"

	"gorm.io/gorm"

	"duluin_invoice/app/model"
)

const appliedDPBackfillKey = "sales_invoice_applied_dp_v1"

// BackfillAppliedDownPaymentsOnce fills applied_dp_amount (and the matching payment_status) for
// invoices that already had confirmed down payments linked to them before the column existed.
// After this, every change goes through syncSalesInvoiceBalance.
func BackfillAppliedDownPaymentsOnce(db *gorm.DB) error {
	var rec model.MigrationRecord
	err := db.Where("key = ?", appliedDPBackfillKey).First(&rec).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("check applied dp record: %w", err)
	}
	var touched int64
	if err := db.Transaction(func(tx *gorm.DB) error {
		res := tx.Exec(`UPDATE sales_invoices i SET applied_dp_amount = s.total
			FROM (SELECT linked_invoice_id, SUM(grand_total) AS total FROM sales_invoices
			      WHERE kind = 'down_payment' AND status = 'confirmed' AND deleted_at IS NULL AND linked_invoice_id IS NOT NULL
			      GROUP BY linked_invoice_id) s
			WHERE i.id = s.linked_invoice_id AND i.kind = 'invoice'`)
		if res.Error != nil {
			return res.Error
		}
		touched = res.RowsAffected
		if err := tx.Exec(`UPDATE sales_invoices SET payment_status = CASE
				WHEN grand_total > 0 AND grand_total - applied_dp_amount - paid_amount <= 0 THEN 'paid'
				ELSE 'partially_paid' END
			WHERE kind = 'invoice' AND applied_dp_amount > 0`).Error; err != nil {
			return err
		}
		return tx.Create(&model.MigrationRecord{Key: appliedDPBackfillKey}).Error
	}); err != nil {
		return err
	}
	log.Printf("✅ Applied down-payment backfill: %d invoice(s) updated", touched)
	return nil
}
