package service

import (
	"math"
	"testing"
	"time"

	domain "duluin_invoice/app/domain/report"
)

// fakeReportRepo — a small hand-written stand-in for domain.IRepository (no
// mocking framework exists in this codebase yet; the interface is tiny
// enough that this is simpler than introducing one).
type fakeReportRepo struct {
	trialBalanceRows  []domain.TrialBalanceRow
	endingBalanceRows []domain.AccountBalanceRow
	incomeExpenseRows []domain.AccountBalanceRow
	openDebit         float64
	openCredit        float64
	ledgerLines       []domain.LedgerLine
	accCode           string
	accName           string
	accGroup          string
}

func (f *fakeReportRepo) TrialBalanceRows(string, time.Time, time.Time) ([]domain.TrialBalanceRow, error) {
	return f.trialBalanceRows, nil
}
func (f *fakeReportRepo) EndingBalanceRows(string, time.Time) ([]domain.AccountBalanceRow, error) {
	return f.endingBalanceRows, nil
}
func (f *fakeReportRepo) IncomeExpenseRows(string, time.Time, time.Time) ([]domain.AccountBalanceRow, error) {
	return f.incomeExpenseRows, nil
}
func (f *fakeReportRepo) LedgerOpeningBalance(string, string, time.Time) (float64, float64, error) {
	return f.openDebit, f.openCredit, nil
}
func (f *fakeReportRepo) LedgerLines(string, string, time.Time, time.Time) ([]domain.LedgerLine, error) {
	return f.ledgerLines, nil
}
func (f *fakeReportRepo) AccountByID(string, string) (string, string, string, error) {
	return f.accCode, f.accName, f.accGroup, nil
}

func TestSignedBalance(t *testing.T) {
	cases := []struct {
		group        string
		debit        float64
		credit       float64
		wantBalance  float64
	}{
		{"asset", 100, 40, 60},     // debit-normal: debit − credit
		{"expense", 100, 40, 60},   // debit-normal
		{"liability", 40, 100, 60}, // credit-normal: credit − debit
		{"equity", 40, 100, 60},    // credit-normal
		{"income", 40, 100, 60},    // credit-normal
	}
	for _, tc := range cases {
		got := signedBalance(tc.group, tc.debit, tc.credit)
		if math.Abs(got-tc.wantBalance) > 1e-9 {
			t.Errorf("signedBalance(%s, %.0f, %.0f) = %.2f, want %.2f", tc.group, tc.debit, tc.credit, got, tc.wantBalance)
		}
	}
}

func TestTrialBalance_EndingIsAdditiveAndBalanceFlagIsCorrect(t *testing.T) {
	repo := &fakeReportRepo{
		trialBalanceRows: []domain.TrialBalanceRow{
			{AccountID: "a1", Code: "1101", Group: "asset", BeginningDebit: 500, PeriodDebit: 200},
			{AccountID: "a2", Code: "2101", Group: "liability", BeginningCredit: 500, PeriodCredit: 200},
		},
	}
	svc := NewReportService(repo)
	resp, err := svc.TrialBalance(&domain.Filter{CompanyID: "c1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Rows[0].EndingDebit != 700 {
		t.Errorf("expected ending debit 700, got %.2f", resp.Rows[0].EndingDebit)
	}
	if resp.Rows[1].EndingCredit != 700 {
		t.Errorf("expected ending credit 700, got %.2f", resp.Rows[1].EndingCredit)
	}
	if !resp.IsBalanced {
		t.Error("expected trial balance to be balanced")
	}

	// Now make it unbalanced.
	repo.trialBalanceRows[1].PeriodCredit = 100
	resp, err = svc.TrialBalance(&domain.Filter{CompanyID: "c1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.IsBalanced {
		t.Error("expected trial balance to be flagged unbalanced")
	}
}

func TestBalanceSheet_FoldsNetProfitToBalance(t *testing.T) {
	// Assets (1000) intentionally exceed Liabilities+Equity (600) by exactly
	// the 400 of unclosed net profit sitting in income/expense — a company
	// that has never run a period-close, which is every company here.
	repo := &fakeReportRepo{
		endingBalanceRows: []domain.AccountBalanceRow{
			{AccountID: "a1", Group: "asset", TotalDebit: 1000},
			{AccountID: "l1", Group: "liability", TotalCredit: 400},
			{AccountID: "e1", Group: "equity", TotalCredit: 200},
		},
		incomeExpenseRows: []domain.AccountBalanceRow{
			{AccountID: "i1", Group: "income", TotalCredit: 500},
			{AccountID: "x1", Group: "expense", TotalDebit: 100},
		},
	}
	svc := NewReportService(repo)
	resp, err := svc.BalanceSheet(&domain.Filter{CompanyID: "c1", EndDate: time.Now()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Assets.Total != 1000 {
		t.Errorf("expected assets 1000, got %.2f", resp.Assets.Total)
	}
	wantEquity := 200 + 400.0 // opening equity + net profit (500 income - 100 expense)
	if math.Abs(resp.Equity.Total-wantEquity) > 1e-9 {
		t.Errorf("expected equity %.2f (incl. folded net profit), got %.2f", wantEquity, resp.Equity.Total)
	}
	if !resp.IsBalanced {
		t.Errorf("expected balance sheet to balance once net profit is folded in: assets=%.2f liab+eq=%.2f",
			resp.Assets.Total, resp.TotalLiabilitiesAndEquity)
	}
}

func TestProfitLoss_NetsIncomeMinusExpense(t *testing.T) {
	repo := &fakeReportRepo{
		incomeExpenseRows: []domain.AccountBalanceRow{
			{AccountID: "i1", Group: "income", TotalCredit: 800},
			{AccountID: "x1", Group: "expense", TotalDebit: 300},
		},
	}
	svc := NewReportService(repo)
	resp, err := svc.ProfitLoss(&domain.Filter{CompanyID: "c1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TotalIncome != 800 || resp.TotalExpense != 300 {
		t.Fatalf("unexpected totals: income=%.2f expense=%.2f", resp.TotalIncome, resp.TotalExpense)
	}
	if resp.NetProfit != 500 {
		t.Errorf("expected net profit 500, got %.2f", resp.NetProfit)
	}
}

func TestGeneralLedger_RunningBalanceWalksForwardFromOpening(t *testing.T) {
	repo := &fakeReportRepo{
		accCode: "1101", accName: "Cash & Bank", accGroup: "asset",
		openDebit: 1000, openCredit: 0, // opening balance 1000 (debit-normal)
		ledgerLines: []domain.LedgerLine{
			{Debit: 500},           // running: 1500
			{Credit: 200},          // running: 1300
			{Debit: 0, Credit: 0},  // no-op line — running stays 1300
		},
	}
	svc := NewReportService(repo)
	resp, err := svc.GeneralLedger(&domain.Filter{CompanyID: "c1", AccountID: "a1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.OpeningBalance != 1000 {
		t.Errorf("expected opening balance 1000, got %.2f", resp.OpeningBalance)
	}
	want := []float64{1500, 1300, 1300}
	for i, w := range want {
		if resp.Lines[i].RunningBalance != w {
			t.Errorf("line %d: expected running balance %.2f, got %.2f", i, w, resp.Lines[i].RunningBalance)
		}
	}
	if resp.ClosingBalance != 1300 {
		t.Errorf("expected closing balance 1300, got %.2f", resp.ClosingBalance)
	}
}

func TestGeneralLedger_RequiresAccountID(t *testing.T) {
	svc := NewReportService(&fakeReportRepo{})
	if _, err := svc.GeneralLedger(&domain.Filter{CompanyID: "c1"}); err == nil {
		t.Error("expected an error when AccountID is blank")
	}
}
