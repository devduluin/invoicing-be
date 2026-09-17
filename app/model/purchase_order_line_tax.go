package model

// PurchaseOrderLineTax — a many-to-many join row: one purchase order line
// can carry any number of taxes, each contributing to LineTaxAmount grouped
// by its calc method (see utils.CalcLines). Managed explicitly by the
// repository (bulk insert/select/delete, ID set by the caller like
// PurchaseOrderLine already is), not a GORM many2many association.
type PurchaseOrderLineTax struct {
	ID                  string `gorm:"type:uuid;primaryKey"     json:"id"`
	PurchaseOrderLineID string `gorm:"type:uuid;not null;index" json:"purchase_order_line_id"`
	TaxID               string `gorm:"type:uuid;not null;index" json:"tax_id"`
}

func (PurchaseOrderLineTax) TableName() string { return "purchase_order_line_taxes" }
