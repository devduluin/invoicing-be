package model

// SalesOrderLineTax — a many-to-many join row: one sales order line can
// carry any number of taxes, each contributing to LineTaxAmount grouped by
// its calc method (see utils.CalcLines). Managed explicitly by the
// repository (bulk insert/select/delete, ID set by the caller like
// SalesOrderLine already is), not a GORM many2many association.
type SalesOrderLineTax struct {
	ID               string `gorm:"type:uuid;primaryKey"     json:"id"`
	SalesOrderLineID string `gorm:"type:uuid;not null;index" json:"sales_order_line_id"`
	TaxID            string `gorm:"type:uuid;not null;index" json:"tax_id"`
}

func (SalesOrderLineTax) TableName() string { return "sales_order_line_taxes" }
