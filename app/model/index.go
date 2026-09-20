package model

import (
	"log"

	"gorm.io/gorm"
)

// AllModels lists every table managed by AutoMigrate.
func AllModels() []interface{} {
	return []interface{}{
		&MigrationRecord{},
		&Company{},
		&UserAccountSSO{},
		&Mitra{},
		&BankAccount{},
		&Account{},
		&Tax{},
		&Unit{},
		&JournalBook{},
		&JournalEntry{},
		&JournalLine{},
		&SalesOrder{},
		&SalesOrderLine{},
		&SalesOrderLineTax{},
		&SalesInvoice{},
		&SalesInvoiceLine{},
		&SalesInvoiceLineTax{},
		&SalesReceipt{},
		&SalesReceiptAllocation{},
		&SalesPayment{},
		&PurchaseOrder{},
		&PurchaseOrderLine{},
		&PurchaseOrderLineTax{},
		&PurchaseInvoice{},
		&PurchaseInvoiceLine{},
		&PurchaseInvoiceLineTax{},
		&PurchaseReceipt{},
		&DeliveryNote{},
		&DeliveryNoteLine{},
		&GoodsReceipt{},
		&GoodsReceiptLine{},
	}
}

// AutoMigrateAll runs GORM AutoMigrate under a Postgres advisory lock so parallel
// instances don't race on schema changes.
func AutoMigrateAll(db *gorm.DB) error {
	log.Println("🔼 Running AutoMigrate...")

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	if err := tx.Exec("SELECT pg_advisory_xact_lock(90821147);").Error; err != nil {
		log.Printf("⚠️  advisory lock failed, migrating without lock: %v", err)
		tx.Rollback()
		if err := db.AutoMigrate(AllModels()...); err != nil {
			return err
		}
		return ensurePartialIndexes(db)
	}

	if err := tx.AutoMigrate(AllModels()...); err != nil {
		tx.Rollback()
		return err
	}
	if err := ensurePartialIndexes(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

// ensurePartialIndexes creates the WHERE-scoped unique indexes GORM tags can't
// express. Idempotent (IF NOT EXISTS).
func ensurePartialIndexes(db *gorm.DB) error {
	stmts := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_companies_code_active
		 ON companies (lower(code)) WHERE deleted_at IS NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_user_account_sso_user_company_active
		 ON user_account_sso (user_id, company_id)
		 WHERE deleted_at IS NULL AND user_id <> ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_user_account_sso_company_email_active
		 ON user_account_sso (company_id, lower(email))
		 WHERE deleted_at IS NULL AND email <> ''`,
		`CREATE INDEX IF NOT EXISTS idx_user_account_sso_role_active
		 ON user_account_sso (role_id) WHERE deleted_at IS NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_accounts_company_code_active
		 ON accounts (company_id, lower(code)) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_bank_accounts_company_active
		 ON bank_accounts (company_id) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_taxes_company_active
		 ON taxes (company_id) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_taxes_component1
		 ON taxes (component1_id) WHERE deleted_at IS NULL AND component1_id IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_taxes_component2
		 ON taxes (component2_id) WHERE deleted_at IS NULL AND component2_id IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_units_company_active
		 ON units (company_id) WHERE deleted_at IS NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_journal_books_company_code_active
		 ON journal_books (company_id, lower(code)) WHERE deleted_at IS NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_journal_entries_company_number_active
		 ON journal_entries (company_id, lower(number)) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_journal_lines_entry
		 ON journal_lines (journal_entry_id)`,
		`CREATE INDEX IF NOT EXISTS idx_journal_lines_account
		 ON journal_lines (account_id)`,
		`CREATE INDEX IF NOT EXISTS idx_journal_lines_mitra
		 ON journal_lines (mitra_id) WHERE mitra_id IS NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_journal_entries_company_idempotency
		 ON journal_entries (company_id, idempotency_key) WHERE idempotency_key <> ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_sales_orders_company_number_active
		 ON sales_orders (company_id, lower(number)) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_sales_order_lines_order
		 ON sales_order_lines (sales_order_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sales_order_lines_tax
		 ON sales_order_lines (tax_id) WHERE tax_id IS NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_sales_invoices_company_number_active
		 ON sales_invoices (company_id, lower(number)) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_sales_invoice_lines_invoice
		 ON sales_invoice_lines (sales_invoice_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sales_invoice_lines_tax
		 ON sales_invoice_lines (tax_id) WHERE tax_id IS NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_sales_receipts_company_number_active
		 ON sales_receipts (company_id, lower(number)) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_sales_receipt_allocations_receipt
		 ON sales_receipt_allocations (sales_receipt_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sales_receipt_allocations_invoice
		 ON sales_receipt_allocations (sales_invoice_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_sales_payments_company_number_active
		 ON sales_payments (company_id, lower(number)) WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_sales_payments_invoice
		 ON sales_payments (sales_invoice_id) WHERE deleted_at IS NULL`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return err
		}
	}
	return nil
}
