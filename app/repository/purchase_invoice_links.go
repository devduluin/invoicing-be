package repository

import (
	"fmt"

	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/purchaseinvoice"
)

// checkPurchaseInvoiceOrder is Sales Invoice's checkSalesInvoiceLinks, purchase-side: a Purchase
// Order's total is NOT a cap on what can be billed from it — same as the sales side, a confirmed
// order can be invoiced any number of times for any amount (Paper.id, the reference product,
// behaves the same way). Only "does this order actually exist" remains, so a bill can never
// reference a deleted/foreign order. selfID is "" on create.
func checkPurchaseInvoiceOrder(tx *gorm.DB, companyID, _ string, purchaseOrderID *string, _ float64) error {
	if purchaseOrderID == nil {
		return nil
	}
	var exists int64
	if err := tx.Table("purchase_orders").Where("id = ? AND company_id = ? AND deleted_at IS NULL", *purchaseOrderID, companyID).Count(&exists).Error; err != nil {
		return fmt.Errorf("find purchase order: %w", err)
	}
	if exists == 0 {
		return &domain.ErrValidation{Message: "the purchase order was not found"}
	}
	return nil
}
