package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"duluin_invoice/utils"
)

// JournalBookType mirrors acc-master's account.journal type (Odoo).
type JournalBookType string

const (
	JournalBookTypeGeneral  JournalBookType = "general"
	JournalBookTypeSale     JournalBookType = "sale"
	JournalBookTypePurchase JournalBookType = "purchase"
	JournalBookTypeCash     JournalBookType = "cash"
	JournalBookTypeBank     JournalBookType = "bank"
)

func IsValidJournalBookType(v string) bool {
	switch JournalBookType(v) {
	case JournalBookTypeGeneral, JournalBookTypeSale, JournalBookTypePurchase, JournalBookTypeCash, JournalBookTypeBank:
		return true
	default:
		return false
	}
}

// JournalBook is master data for a "buku jurnal" (Odoo: account.journal) — the
// reference UI's "Jurnal" dropdown. Simplified from acc-master's Journal: no
// multi-currency/suspense/profit-loss/sequence-prefix fields, since
// GenerateJournalEntryNumber-equivalent numbering here only ever reads Code
// (see journal.repository.go), and invoice-service has no multi-currency
// module to hang the rest on.
type JournalBook struct {
	ID                     string          `gorm:"type:uuid;primaryKey"       json:"id"`
	CompanyID              string          `gorm:"type:uuid;not null;index"   json:"company_id"`
	Code                   string          `gorm:"type:varchar(20);not null"  json:"code"`
	Name                   string          `gorm:"type:varchar(255);not null" json:"name"`
	Type                   JournalBookType `gorm:"type:varchar(20);not null"  json:"type"`
	DefaultAccountID       *string         `gorm:"type:uuid;index"            json:"default_account_id,omitempty"`
	DefaultDebitAccountID  *string         `gorm:"type:uuid;index"            json:"default_debit_account_id,omitempty"`
	DefaultCreditAccountID *string         `gorm:"type:uuid;index"            json:"default_credit_account_id,omitempty"`
	IsSystem               utils.BoolInt   `gorm:"type:smallint;not null;default:0" json:"is_system"`
	IsActive               utils.BoolInt   `gorm:"type:smallint;not null;default:1" json:"is_active"`

	CreatedAt time.Time      `gorm:"autoCreateTime"   json:"created_at"`
	CreatedBy string         `gorm:"type:varchar(64)" json:"created_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"   json:"updated_at"`
	UpdatedBy string         `gorm:"type:varchar(64)" json:"updated_by,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index"            json:"-"`
}

func (JournalBook) TableName() string { return "journal_books" }

func (j *JournalBook) BeforeCreate(tx *gorm.DB) error {
	if j.ID == "" {
		j.ID = uuid.New().String()
	}
	return nil
}

// journalBookDef is the seed shape — mirrors acc-master's journalDef, dropping
// the currency/suspense/profit/loss keys that repo resolves (no equivalent
// module here yet).
type journalBookDef struct {
	Name string
	Code string
	Type JournalBookType
}

// DefaultJournalBooks — verbatim from acc-master-service's indonesiaJournals()
// (database/seeds/journal_indonesia.go): same 8 books, names, codes, types.
// Account links aren't seeded (optional fields, set later per company).
func DefaultJournalBooks() []journalBookDef {
	return []journalBookDef{
		{Name: "Sales", Code: "SALES", Type: JournalBookTypeSale},
		{Name: "Purchases", Code: "PURCH", Type: JournalBookTypePurchase},
		{Name: "Bank", Code: "BANK", Type: JournalBookTypeBank},
		{Name: "Cash", Code: "CASH", Type: JournalBookTypeCash},
		{Name: "Miscellaneous Operations", Code: "MISC", Type: JournalBookTypeGeneral},
		{Name: "Exchange Difference", Code: "EXCH", Type: JournalBookTypeGeneral},
		{Name: "Cash Basis Taxes", Code: "CABA", Type: JournalBookTypeGeneral},
		{Name: "Tax Returns", Code: "TAX", Type: JournalBookTypeGeneral},
	}
}
