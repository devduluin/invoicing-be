package service

import (
	"math"
	"strings"
	"time"

	domain "duluin_invoice/app/domain/report"
)

type ReportService struct{ repo domain.IRepository }

func NewReportService(repo domain.IRepository) domain.IService { return &ReportService{repo: repo} }

// isDebitNormal — asset & expense accounts increase with a debit; liability,
// equity and income accounts increase with a credit (standard double-entry
// convention, matching accounting-engine-service's normal_balance column —
// invoice-service has no such stored column, so it's derived from Group).
func isDebitNormal(group string) bool {
	switch group {
	case "asset", "expense":
		return true
	default: // liability, equity, income
		return false
	}
}

// signedBalance — a positive number always means "more of this account",
// regardless of which side is its natural side.
func signedBalance(group string, debit, credit float64) float64 {
	if isDebitNormal(group) {
		return debit - credit
	}
	return credit - debit
}

func (s *ReportService) TrialBalance(f *domain.Filter) (*domain.TrialBalanceResponse, error) {
	rows, err := s.repo.TrialBalanceRows(f.CompanyID, f.StartDate, f.EndDate)
	if err != nil {
		return nil, err
	}
	resp := &domain.TrialBalanceResponse{StartDate: f.StartDate, EndDate: f.EndDate}
	for i := range rows {
		// "ending = beginning + period", added like-for-like (not netted) —
		// matches accounting-engine-service's GetTrialBalance exactly.
		rows[i].EndingDebit = rows[i].BeginningDebit + rows[i].PeriodDebit
		rows[i].EndingCredit = rows[i].BeginningCredit + rows[i].PeriodCredit
		resp.TotalBeginningDebit += rows[i].BeginningDebit
		resp.TotalBeginningCredit += rows[i].BeginningCredit
		resp.TotalPeriodDebit += rows[i].PeriodDebit
		resp.TotalPeriodCredit += rows[i].PeriodCredit
		resp.TotalEndingDebit += rows[i].EndingDebit
		resp.TotalEndingCredit += rows[i].EndingCredit
	}
	resp.Rows = rows
	resp.IsBalanced = math.Abs(resp.TotalEndingDebit-resp.TotalEndingCredit) < balanceEpsilon
	return resp, nil
}

func (s *ReportService) GeneralLedger(f *domain.Filter) (*domain.GeneralLedgerResponse, error) {
	if strings.TrimSpace(f.AccountID) == "" {
		return nil, &domain.ErrValidation{Message: "an account is required for the general ledger"}
	}

	code, name, group, err := s.repo.AccountByID(f.CompanyID, f.AccountID)
	if err != nil {
		return nil, err
	}

	openDebit, openCredit, err := s.repo.LedgerOpeningBalance(f.CompanyID, f.AccountID, f.StartDate)
	if err != nil {
		return nil, err
	}
	opening := signedBalance(group, openDebit, openCredit)

	rawLines, err := s.repo.LedgerLines(f.CompanyID, f.AccountID, f.StartDate, f.EndDate)
	if err != nil {
		return nil, err
	}

	running := opening
	lines := make([]domain.LedgerLine, len(rawLines))
	for i, l := range rawLines {
		running += signedBalance(group, l.Debit, l.Credit)
		lines[i] = domain.LedgerLine{
			Date: l.Date, Number: l.Number, Description: l.Description,
			Debit: l.Debit, Credit: l.Credit, RunningBalance: running,
		}
	}

	return &domain.GeneralLedgerResponse{
		AccountID: f.AccountID, AccountCode: code, AccountName: name,
		StartDate: f.StartDate, EndDate: f.EndDate,
		OpeningBalance: opening, Lines: lines, ClosingBalance: running,
	}, nil
}

func (s *ReportService) BalanceSheet(f *domain.Filter) (*domain.BalanceSheetResponse, error) {
	asOf := f.EndDate
	rows, err := s.repo.EndingBalanceRows(f.CompanyID, asOf)
	if err != nil {
		return nil, err
	}

	resp := &domain.BalanceSheetResponse{AsOfDate: asOf}
	for _, row := range rows {
		bal := signedBalance(row.Group, row.TotalDebit, row.TotalCredit)
		if math.Abs(bal) < 1e-9 {
			continue
		}
		r := domain.BalanceSheetRow{AccountID: row.AccountID, Code: row.Code, Name: row.Name, Balance: bal}
		switch row.Group {
		case "asset":
			resp.Assets.Rows = append(resp.Assets.Rows, r)
			resp.Assets.Total += bal
		case "liability":
			resp.Liabilities.Rows = append(resp.Liabilities.Rows, r)
			resp.Liabilities.Total += bal
		case "equity":
			resp.Equity.Rows = append(resp.Equity.Rows, r)
			resp.Equity.Total += bal
		}
	}

	// invoice-service has no period-closing/retained-earnings mechanism, so
	// there's no "prior years" vs "this year" split — cumulative net profit
	// since the very first posted entry is folded in as a single synthetic
	// equity line. Once period closing exists, this is exactly what it would
	// zero out into a real Retained Earnings account.
	netProfit, err := s.cumulativeNetProfit(f.CompanyID, asOf)
	if err != nil {
		return nil, err
	}
	resp.Equity.Rows = append(resp.Equity.Rows, domain.BalanceSheetRow{Name: "Laba/Rugi Berjalan", Balance: netProfit})
	resp.Equity.Total += netProfit

	resp.TotalLiabilitiesAndEquity = resp.Liabilities.Total + resp.Equity.Total
	resp.IsBalanced = math.Abs(resp.Assets.Total-resp.TotalLiabilitiesAndEquity) < balanceEpsilon
	return resp, nil
}

func (s *ReportService) ProfitLoss(f *domain.Filter) (*domain.ProfitLossResponse, error) {
	rows, err := s.repo.IncomeExpenseRows(f.CompanyID, f.StartDate, f.EndDate)
	if err != nil {
		return nil, err
	}
	resp := &domain.ProfitLossResponse{StartDate: f.StartDate, EndDate: f.EndDate}
	for _, row := range rows {
		amt := signedBalance(row.Group, row.TotalDebit, row.TotalCredit)
		if math.Abs(amt) < 1e-9 {
			continue
		}
		r := domain.ProfitLossRow{AccountID: row.AccountID, Code: row.Code, Name: row.Name, Amount: amt}
		switch row.Group {
		case "income":
			resp.Income = append(resp.Income, r)
			resp.TotalIncome += amt
		case "expense":
			resp.Expense = append(resp.Expense, r)
			resp.TotalExpense += amt
		}
	}
	resp.NetProfit = resp.TotalIncome - resp.TotalExpense
	return resp, nil
}

func (s *ReportService) cumulativeNetProfit(companyID string, asOf time.Time) (float64, error) {
	rows, err := s.repo.IncomeExpenseRows(companyID, time.Time{}, asOf)
	if err != nil {
		return 0, err
	}
	var total float64
	for _, row := range rows {
		switch row.Group {
		case "income":
			total += signedBalance(row.Group, row.TotalDebit, row.TotalCredit)
		case "expense":
			total -= signedBalance(row.Group, row.TotalDebit, row.TotalCredit)
		}
	}
	return total, nil
}
