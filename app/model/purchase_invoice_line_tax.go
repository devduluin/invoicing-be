package model

// PurchaseInvoiceLineTax — a many-to-many join row: one purchase invoice
// line can carry any number of taxes, each contributing to LineTaxAmount
// grouped by its calc method (see utils.CalcLines). Managed explicitly by
// the repository (bulk insert/select/delete, ID set by the caller like
// PurchaseInvoiceLine already is), not a GORM many2many association.
type PurchaseInvoiceLineTax struct {
	ID                    string `gorm:"type:uuid;primaryKey"     json:"id"`
	PurchaseInvoiceLineID string `gorm:"type:uuid;not null;index" json:"purchase_invoice_line_id"`
	TaxID                 string `gorm:"type:uuid;not null;index" json:"tax_id"`
}

func (PurchaseInvoiceLineTax) TableName() string { return "purchase_invoice_line_taxes" }
