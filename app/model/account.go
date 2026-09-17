package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"duluin_invoice/utils"
)

// AccountGroup is the balance-sheet / P&L classification of a COA account
// (PRD §7). Simplified from acc-master's Odoo-grade taxonomy.
type AccountGroup string

const (
	AccountGroupAsset     AccountGroup = "asset"
	AccountGroupLiability AccountGroup = "liability"
	AccountGroupEquity    AccountGroup = "equity"
	AccountGroupIncome    AccountGroup = "income"
	AccountGroupExpense   AccountGroup = "expense"
)

func IsValidAccountGroup(v string) bool {
	switch AccountGroup(v) {
	case AccountGroupAsset, AccountGroupLiability, AccountGroupEquity, AccountGroupIncome, AccountGroupExpense:
		return true
	default:
		return false
	}
}

// Account is one Chart-of-Accounts entry. A company starts with the seeded
// template (IsSystem) and may add / rename / nest its own (PRD §7 note: users
// asked for a customisable COA à la Paper.id).
type Account struct {
	ID        string        `gorm:"type:uuid;primaryKey"           json:"id"`
	CompanyID string        `gorm:"type:uuid;not null;index"       json:"company_id"`
	ParentID  *string       `gorm:"type:uuid;index"                json:"parent_id,omitempty"`
	Code      string        `gorm:"type:varchar(20);not null"      json:"code"`
	Name      string        `gorm:"type:varchar(255);not null"     json:"name"`
	Group     AccountGroup  `gorm:"type:varchar(20);not null"      json:"group"`
	IsSystem  utils.BoolInt `gorm:"type:smallint;not null;default:0" json:"is_system"` // from the template — can't be deleted
	IsActive  utils.BoolInt `gorm:"type:smallint;not null;default:1" json:"is_active"`

	CreatedAt time.Time      `gorm:"autoCreateTime"   json:"created_at"`
	CreatedBy string         `gorm:"type:varchar(64)" json:"created_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"   json:"updated_at"`
	UpdatedBy string         `gorm:"type:varchar(64)" json:"updated_by,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index"            json:"-"`
}

func (Account) TableName() string { return "accounts" }

func (a *Account) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

// AccountSeed is one row of the default COA template. ParentCode links to
// another seed's Code (resolved to ParentID at insert time).
type AccountSeed struct {
	Code       string
	Name       string
	Group      AccountGroup
	ParentCode string
}

// DefaultAccounts — the default Indonesian chart of accounts, ported verbatim
// from acc-master-service's `indonesiaAccounts()` (database/seeds/coa_indonesia.go,
// the COATemplateIndonesiaDefault template). Same codes, names and hierarchy so
// a company's books line up across the two products. acc-master's fine-grained
// AccountType is collapsed to Group here (invoice-service has no GL yet); the
// dotted seed codes (1101.001) are stored pre-normalised the way acc-master
// persists them (110101).
func DefaultAccounts() []AccountSeed {
	g := AccountGroupAsset
	l := AccountGroupLiability
	e := AccountGroupEquity
	i := AccountGroupIncome
	x := AccountGroupExpense
	return []AccountSeed{
		// ── Header / group accounts ──────────────────────────────────────
		{"1101", "Cash & Bank", g, ""},
		{"1102", "Receivables", g, ""},
		{"1103", "Inventories", g, ""},
		{"1104", "Prepaid Assets", g, ""},
		{"1105", "Other Current Assets", g, ""},
		{"1200", "Fixed Assets", g, ""},
		{"2101", "Current Liabilities", l, ""},
		{"2102", "Other Current Liabilities", l, ""},
		{"3001", "Equity", e, ""},
		{"3100", "Accumulated OCI", e, ""},
		{"4001", "Operating Revenue", i, ""},
		{"5001", "Cost of Goods Sold", x, ""},
		{"6000", "Operating Expenses", x, ""},
		{"6001", "Personnel Expenses", x, "6000"},
		{"6002", "General & Administrative Expenses", x, "6000"},
		{"6003", "Marketing & Selling Expenses", x, "6000"},
		{"6004", "Depreciation & Amortization Expenses", x, "6000"},
		{"7001", "Other Income", i, ""},
		{"8001", "Other Expense", x, ""},

		// ── Cash & Bank ──────────────────────────────────────────────────
		{"110101", "Bank", g, "1101"},
		{"110102", "Petty Cash", g, "1101"},
		{"110103", "Bank Mandiri", g, "1101"},
		{"110104", "Bank Permata", g, "1101"},
		{"110105", "Bank OCBC", g, "1101"},
		{"110106", "Bank BRI", g, "1101"},
		{"110107", "Bank BJB", g, "1101"},
		{"110108", "Bank BNI", g, "1101"},
		{"110109", "Bank Danamon", g, "1101"},
		{"110110", "Outstanding Receipts", g, "1101"},
		{"110111", "Outstanding Payments", g, "1101"},

		// ── Receivable ───────────────────────────────────────────────────
		{"110201", "Account Receivable", g, "1102"},
		{"110202", "Reconciled Account Receivable", g, "1102"},

		// ── Inventory ────────────────────────────────────────────────────
		{"110301", "Inventory", g, "1103"},
		{"110302", "Delivered Inventory", g, "1103"},
		{"110303", "Inventory Adjustment", x, "1103"},

		// ── Prepaid / other current assets ───────────────────────────────
		{"110401", "Office Supplies", g, "1104"},
		{"110402", "Prepaid Rent", g, "1104"},
		{"110403", "Prepaid Insurance", g, "1104"},
		{"110501", "VAT In", g, "1105"},
		{"110502", "Prepaid Income Tax Art. 23", g, "1105"},
		{"110503", "Prepaid Income Tax Art. 22", g, "1105"},
		{"110504", "Prepaid Income Tax Art. 26", g, "1105"},
		{"110505", "Prepaid Final Income Tax Art. 4(2)", g, "1105"},

		// ── Fixed assets ─────────────────────────────────────────────────
		{"120101", "Land", g, "1200"},
		{"120201", "Building", g, "1200"},
		{"120301", "Vehicle", g, "1200"},
		{"120401", "Equipment", g, "1200"},
		{"120501", "Office Furniture", g, "1200"},
		{"120601", "Accumulated Depreciation", g, "1200"},

		// ── Liabilities ──────────────────────────────────────────────────
		{"210101", "Account Payable", l, "2101"},
		{"210102", "Unearned Revenue", l, "2101"},
		{"210201", "VAT Out", l, "2102"},
		{"210202", "Income Tax Payable Art. 23", l, "2102"},
		{"210203", "Income Tax Payable Art. 22", l, "2102"},
		{"210204", "Income Tax Payable Art. 26", l, "2102"},
		{"210205", "Final Income Tax Payable Art. 4(2)", l, "2102"},

		// ── Equity ───────────────────────────────────────────────────────
		{"300101", "Equity", e, "3001"},
		{"300102", "Beginning Balance Equity", e, "3001"},
		{"300103", "Retained Earnings", e, "3001"},
		{"300104", "Current Year Earnings", e, "3001"},
		{"310001", "Revaluation Surplus", e, "3100"},

		// ── Revenue ──────────────────────────────────────────────────────
		{"400101", "Operating Revenue", i, "4001"},
		{"400102", "Platform Fee", i, "4001"},
		{"400103", "Payment Processing Fee", i, "4001"},
		{"400104", "Sales Discount", i, "4001"},

		// ── Cost of goods sold ───────────────────────────────────────────
		{"500101", "Financing Cost", x, "5001"},
		{"500102", "Software, Cloud & IT Infrastructure Cost", x, "5001"},

		// ── Operating expense ────────────────────────────────────────────
		{"600101", "Employee Salary", x, "6001"},
		{"600102", "Employee Bonus", x, "6001"},
		{"600201", "Office Rent & Lease Expenses", x, "6002"},
		{"600202", "Utilities (Electricity, Water, Internet)", x, "6002"},
		{"600301", "Marketing and Promotions Campaign", x, "6003"},
		{"600302", "Association and Partner Support", x, "6003"},
		{"600401", "Depreciation Expense", x, "6004"},

		// ── Other income ─────────────────────────────────────────────────
		{"700101", "Bank Interest Income", i, "7001"},
		{"700102", "Profit from Assets Disposal", i, "7001"},
		{"700103", "Exchange Rate Gain", i, "7001"},

		// ── Other expense ────────────────────────────────────────────────
		{"800101", "Interest Fee on Short Term Loans", x, "8001"},
		{"800102", "Bank Admin Fee", x, "8001"},
		{"800103", "Tax on Bank Interest Income", x, "8001"},
		{"800104", "Exchange Rate Loss", x, "8001"},
		{"800105", "Rounding", x, "8001"},
		{"800106", "Suspense Account", x, "8001"},
	}
}
