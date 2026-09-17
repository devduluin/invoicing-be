// Package domain_report — financial reports computed from posted journal
// entries (Trial Balance, General Ledger, Balance Sheet, Profit & Loss).
// Read-only: no Create/Update DTOs, nothing here writes anything.
package domain_report

import "time"

// Filter drives every report. AccountID is only used by GeneralLedger.
type Filter struct {
	CompanyID string
	StartDate time.Time
	EndDate   time.Time
	AccountID string
}

// AccountBalanceRow is what the repository hands back — raw summed
// debit/credit per account, no sign convention applied yet (that's the
// service's job, since it depends on Account.Group).
type AccountBalanceRow struct {
	AccountID   string
	Code        string
	Name        string
	Group       string
	TotalDebit  float64
	TotalCredit float64
}

// TrialBalanceRow — one account's beginning/period/ending columns.
type TrialBalanceRow struct {
	AccountID      string  `json:"account_id"`
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	Group          string  `json:"group"`
	BeginningDebit float64 `json:"beginning_debit"`
	BeginningCredit float64 `json:"beginning_credit"`
	PeriodDebit    float64 `json:"period_debit"`
	PeriodCredit   float64 `json:"period_credit"`
	EndingDebit    float64 `json:"ending_debit"`
	EndingCredit   float64 `json:"ending_credit"`
}

type TrialBalanceResponse struct {
	StartDate            time.Time         `json:"start_date"`
	EndDate              time.Time         `json:"end_date"`
	Rows                 []TrialBalanceRow `json:"rows"`
	TotalBeginningDebit  float64           `json:"total_beginning_debit"`
	TotalBeginningCredit float64           `json:"total_beginning_credit"`
	TotalPeriodDebit     float64           `json:"total_period_debit"`
	TotalPeriodCredit    float64           `json:"total_period_credit"`
	TotalEndingDebit     float64           `json:"total_ending_debit"`
	TotalEndingCredit    float64           `json:"total_ending_credit"`
	IsBalanced           bool              `json:"is_balanced"`
}

// LedgerLine — one posted journal line affecting the ledger's account.
type LedgerLine struct {
	Date           time.Time `json:"date"`
	Number         string    `json:"number"`
	Description    string    `json:"description"`
	Debit          float64   `json:"debit"`
	Credit         float64   `json:"credit"`
	RunningBalance float64   `json:"running_balance"`
}

type GeneralLedgerResponse struct {
	AccountID      string       `json:"account_id"`
	AccountCode    string       `json:"account_code"`
	AccountName    string       `json:"account_name"`
	StartDate      time.Time    `json:"start_date"`
	EndDate        time.Time    `json:"end_date"`
	OpeningBalance float64      `json:"opening_balance"`
	Lines          []LedgerLine `json:"lines"`
	ClosingBalance float64      `json:"closing_balance"`
}

// BalanceSheetRow — AccountID is "" for the synthetic "Laba/Rugi Berjalan"
// row folded into Equity (see report.service.go).
type BalanceSheetRow struct {
	AccountID string  `json:"account_id"`
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Balance   float64 `json:"balance"`
}

type BalanceSheetSection struct {
	Rows  []BalanceSheetRow `json:"rows"`
	Total float64           `json:"total"`
}

type BalanceSheetResponse struct {
	AsOfDate                  time.Time           `json:"as_of_date"`
	Assets                    BalanceSheetSection `json:"assets"`
	Liabilities               BalanceSheetSection `json:"liabilities"`
	Equity                    BalanceSheetSection `json:"equity"`
	TotalLiabilitiesAndEquity float64             `json:"total_liabilities_and_equity"`
	IsBalanced                bool                `json:"is_balanced"`
}

type ProfitLossRow struct {
	AccountID string  `json:"account_id"`
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Amount    float64 `json:"amount"`
}

type ProfitLossResponse struct {
	StartDate   time.Time       `json:"start_date"`
	EndDate     time.Time       `json:"end_date"`
	Income      []ProfitLossRow `json:"income"`
	TotalIncome float64         `json:"total_income"`
	Expense     []ProfitLossRow `json:"expense"`
	TotalExpense float64        `json:"total_expense"`
	NetProfit   float64         `json:"net_profit"`
}

type IRepository interface {
	// TrialBalanceRows — beginning (date < start) and period (start..end)
	// sums per account, only for accounts with any non-zero column.
	TrialBalanceRows(companyID string, start, end time.Time) ([]TrialBalanceRow, error)
	// EndingBalanceRows — cumulative sums up to asOfDate, asset/liability/
	// equity accounts only.
	EndingBalanceRows(companyID string, asOfDate time.Time) ([]AccountBalanceRow, error)
	// IncomeExpenseRows — sums within start..end, income/expense accounts only.
	IncomeExpenseRows(companyID string, start, end time.Time) ([]AccountBalanceRow, error)
	// LedgerOpeningBalance — raw debit/credit sums for one account before start.
	LedgerOpeningBalance(companyID, accountID string, start time.Time) (debit, credit float64, err error)
	// LedgerLines — posted lines for one account within start..end, ordered by date.
	LedgerLines(companyID, accountID string, start, end time.Time) ([]LedgerLine, error)
	// AccountByID — for the ledger's header (code/name/group).
	AccountByID(companyID, accountID string) (code, name, group string, err error)
}

type IService interface {
	TrialBalance(f *Filter) (*TrialBalanceResponse, error)
	GeneralLedger(f *Filter) (*GeneralLedgerResponse, error)
	BalanceSheet(f *Filter) (*BalanceSheetResponse, error)
	ProfitLoss(f *Filter) (*ProfitLossResponse, error)
}

type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }
