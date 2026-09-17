package model

// SalesInvoiceLineTax — a many-to-many join row: one sales invoice line can
// carry any number of taxes, each contributing to LineTaxAmount grouped by
// its calc method (see utils.CalcLines). Managed explicitly by the
// repository (bulk insert/select/delete, ID set by the caller like
// SalesInvoiceLine already is), not a GORM many2many association.
type SalesInvoiceLineTax struct {
	ID                 string `gorm:"type:uuid;primaryKey"     json:"id"`
	SalesInvoiceLineID string `gorm:"type:uuid;not null;index" json:"sales_invoice_line_id"`
	TaxID              string `gorm:"type:uuid;not null;index" json:"tax_id"`
}

func (SalesInvoiceLineTax) TableName() string { return "sales_invoice_line_taxes" }
