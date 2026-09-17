package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"duluin_invoice/utils"
)

// TaxKind groups a tax for reporting/journal routing (PRD §9): output VAT on
// sales, prepaid tax on purchases.
type TaxKind string

const (
	TaxKindPPN   TaxKind = "ppn"   // PPN (Pajak Pertambahan Nilai)
	TaxKindPPh   TaxKind = "pph"   // PPh (withholding)
	TaxKindOther TaxKind = "other" // PB1, Pajak Restoran, dll
)

func IsValidTaxKind(v string) bool {
	switch TaxKind(v) {
	case TaxKindPPN, TaxKindPPh, TaxKindOther:
		return true
	default:
		return false
	}
}

// TaxCalcMethod — how the rate meets the line price (Paper.id "Metode
// Perhitungan"). Exclusive: tax is added on top of the price. Inclusive: the
// price already contains the tax, so DPP is back-calculated.
type TaxCalcMethod string

const (
	TaxCalcExclusive TaxCalcMethod = "exclusive"
	TaxCalcInclusive TaxCalcMethod = "inclusive"
)

func IsValidTaxCalcMethod(v string) bool {
	switch TaxCalcMethod(v) {
	case TaxCalcExclusive, TaxCalcInclusive:
		return true
	default:
		return false
	}
}

// Tax is a company's configurable tax entry (PRD §9). A "single" tax carries a
// rate, a calc method and the two COA accounts it posts to. A "compound" tax
// (IsCompound) is just a named pair of two single taxes applied together —
// its rate is the sum of its components and it has no accounts of its own.
// Rate is a percentage (e.g. 11.0 for 11%) stored as numeric — a rate, not a
// monetary amount.
type Tax struct {
	ID        string        `gorm:"type:uuid;primaryKey"                      json:"id"`
	CompanyID string        `gorm:"type:uuid;not null;index"                  json:"company_id"`
	Name      string        `gorm:"type:varchar(120);not null"               json:"name"`
	Kind      TaxKind       `gorm:"type:varchar(20);not null;default:'other'" json:"kind"`
	Rate      float64       `gorm:"type:numeric(7,4);not null;default:0"      json:"rate"`
	IsSystem  utils.BoolInt `gorm:"type:smallint;not null;default:0"          json:"is_system"` // seeded default — name/kind locked
	IsActive  utils.BoolInt `gorm:"type:smallint;not null;default:1"          json:"is_active"`

	// Single-tax fields (nil / default on a compound tax).
	CalcMethod        TaxCalcMethod `gorm:"type:varchar(20);not null;default:'exclusive'" json:"calc_method"`
	SalesAccountID    *string       `gorm:"type:uuid"                                     json:"sales_account_id,omitempty"`
	PurchaseAccountID *string       `gorm:"type:uuid"                                     json:"purchase_account_id,omitempty"`

	// Compound-tax fields (nil on a single tax).
	IsCompound   utils.BoolInt `gorm:"type:smallint;not null;default:0" json:"is_compound"`
	Component1ID *string       `gorm:"type:uuid"                        json:"component1_id,omitempty"`
	Component2ID *string       `gorm:"type:uuid"                        json:"component2_id,omitempty"`

	CreatedAt time.Time      `gorm:"autoCreateTime"   json:"created_at"`
	CreatedBy string         `gorm:"type:varchar(64)" json:"created_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"   json:"updated_at"`
	UpdatedBy string         `gorm:"type:varchar(64)" json:"updated_by,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index"            json:"-"`
}

func (Tax) TableName() string { return "taxes" }

func (t *Tax) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

// TaxSeed is one row of the default tax template. Account*Code links to a
// DefaultAccounts() code, resolved to an account id per company at seed time.
type TaxSeed struct {
	Name                string
	Kind                TaxKind
	CalcMethod          TaxCalcMethod
	Rate                float64
	SalesAccountCode    string
	PurchaseAccountCode string
}

// DefaultTaxes — the default Indonesian taxes, ported from acc-master-service's
// indonesiaTaxDefs() (database/seeds/tax_indonesia.go). acc-master splits each
// tax by direction (a sales row and a purchase row); invoice-service
// carries one row with both a sales and a purchase account, so the VAT pair is
// merged and every withholding tax posts to its payable (purchase) and its
// prepaid (sale) account. Account codes are DefaultAccounts() entries:
//
//	VAT Out 210201 / VAT In 110501
//	Income Tax Payable Art. 23 210202 / Prepaid Income Tax Art. 23 110502
//	Income Tax Payable Art. 22 210203 / Prepaid Income Tax Art. 22 110503
//	Income Tax Payable Art. 26 210204 / Prepaid Income Tax Art. 26 110504
//	Final Income Tax Payable Art. 4(2) 210205 / Prepaid Final Income Tax Art. 4(2) 110505
func DefaultTaxes() []TaxSeed {
	const (
		vatOut = "210201"
		vatIn  = "110501"
		pay23  = "210202"
		pre23  = "110502"
		pay22  = "210203"
		pre22  = "110503"
		pay26  = "210204"
		pre26  = "110504"
		payFin = "210205"
		preFin = "110505"
	)
	return []TaxSeed{
		// ── VAT ──
		{"VAT 11%", TaxKindPPN, TaxCalcExclusive, 11, vatOut, vatIn},
		{"VAT 0% Export", TaxKindPPN, TaxCalcExclusive, 0, vatOut, vatIn},

		// ── Withholding Tax Art. 23 ──
		{"WHT Art. 23 Services (2%)", TaxKindPPh, TaxCalcExclusive, 2, pre23, pay23},
		{"WHT Art. 23 Rent (2%)", TaxKindPPh, TaxCalcExclusive, 2, pre23, pay23},
		{"WHT Art. 23 Royalty (15%)", TaxKindPPh, TaxCalcExclusive, 15, pre23, pay23},
		{"WHT Art. 23 Dividend (15%)", TaxKindPPh, TaxCalcExclusive, 15, pre23, pay23},
		{"WHT Art. 23 Interest (15%)", TaxKindPPh, TaxCalcExclusive, 15, pre23, pay23},

		// ── Withholding Tax Art. 22 ──
		{"WHT Art. 22 Import (API) (2.5%)", TaxKindPPh, TaxCalcExclusive, 2.5, pre22, pay22},
		{"WHT Art. 22 Import (Non-API) (7.5%)", TaxKindPPh, TaxCalcExclusive, 7.5, pre22, pay22},

		// ── Withholding Tax Art. 26 ──
		{"WHT Art. 26 Dividend (20%)", TaxKindPPh, TaxCalcExclusive, 20, pre26, pay26},
		{"WHT Art. 26 Interest (20%)", TaxKindPPh, TaxCalcExclusive, 20, pre26, pay26},
		{"WHT Art. 26 Royalty (20%)", TaxKindPPh, TaxCalcExclusive, 20, pre26, pay26},
		{"WHT Art. 26 Services (20%)", TaxKindPPh, TaxCalcExclusive, 20, pre26, pay26},

		// ── Final Withholding Tax Art. 4(2) ──
		{"Final WHT Land & Building Rent (10%)", TaxKindPPh, TaxCalcExclusive, 10, preFin, payFin},
		{"Final WHT Deposit Interest (20%)", TaxKindPPh, TaxCalcExclusive, 20, preFin, payFin},
		{"Final WHT Small Construction Services (2%)", TaxKindPPh, TaxCalcExclusive, 2, preFin, payFin},
		{"Final WHT Medium/Large Construction Services (3%)", TaxKindPPh, TaxCalcExclusive, 3, preFin, payFin},
		{"Final WHT Land & Building Rights Transfer (2.5%)", TaxKindPPh, TaxCalcExclusive, 2.5, preFin, payFin},
	}
}
