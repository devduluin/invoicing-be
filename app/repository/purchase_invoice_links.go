package repository

import (
	"fmt"

	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/purchaseinvoice"
	"duluin_invoice/app/model"
)

// checkPurchaseInvoiceOrder is Sales Invoice's checkSalesInvoiceLinks, purchase-side: the sum of
// non-cancelled bills made from a Purchase Order may not exceed that order's total. selfID is ""
// on create. A missing/deleted order fails closed (never silently skips the cap).
func checkPurchaseInvoiceOrder(tx *gorm.DB, companyID, selfID string, purchaseOrderID *string, total float64) error {
	if purchaseOrderID == nil {
		return nil
	}
	var order struct{ GrandTotal float64 }
	res := tx.Table("purchase_orders").Select("grand_total").
		Where("id = ? AND company_id = ? AND deleted_at IS NULL", *purchaseOrderID, companyID).Scan(&order)
	if res.Error != nil {
		return fmt.Errorf("find purchase order: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return &domain.ErrValidation{Message: "the purchase order was not found"}
	}
	q := tx.Model(&model.PurchaseInvoice{}).
		Where("company_id = ? AND purchase_order_id = ? AND status <> ?", companyID, *purchaseOrderID, model.PurchaseInvoiceStatusCancelled)
	if selfID != "" {
		q = q.Where("id <> ?", selfID)
	}
	var used float64
	if err := q.Select("COALESCE(SUM(grand_total), 0)").Scan(&used).Error; err != nil {
		return fmt.Errorf("sum bills of purchase order: %w", err)
	}
	if order.GrandTotal > 0 && round2(used)+total > order.GrandTotal+amountEpsilon {
		return &domain.ErrValidation{Message: fmt.Sprintf("amount (%.2f) exceeds what is still available on the purchase order (%.2f)", total, order.GrandTotal-round2(used))}
	}
	return nil
}
