package repository

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/report"
	"duluin_invoice/app/model"
)

type ReportRepository struct{ db *gorm.DB }

func NewReportRepository(db *gorm.DB) domain.IRepository { return &ReportRepository{db: db} }

// TrialBalanceRows — mirrors accounting-engine-service's GetTrialBalance,
// simplified to invoice-service's schema (no opening_balance/normal_balance
// columns — beginning is pure pre-period activity).
func (r *ReportRepository) TrialBalanceRows(companyID string, start, end time.Time) ([]domain.TrialBalanceRow, error) {
	rows := make([]domain.TrialBalanceRow, 0)
	query := `
		SELECT
			a.id AS account_id, a.code, a.name, a."group",
			COALESCE(s.beginning_debit, 0)  AS beginning_debit,
			COALESCE(s.beginning_credit, 0) AS beginning_credit,
			COALESCE(s.period_debit, 0)     AS period_debit,
			COALESCE(s.period_credit, 0)    AS period_credit
		FROM accounts a
		LEFT JOIN (
			SELECT
				jl.account_id,
				SUM(CASE WHEN je.date < ? THEN jl.debit ELSE 0 END) AS beginning_debit,
				SUM(CASE WHEN je.date < ? THEN jl.credit ELSE 0 END) AS beginning_credit,
				SUM(CASE WHEN je.date >= ? AND je.date <= ? THEN jl.debit ELSE 0 END) AS period_debit,
				SUM(CASE WHEN je.date >= ? AND je.date <= ? THEN jl.credit ELSE 0 END) AS period_credit
			FROM journal_lines jl
			JOIN journal_entries je ON je.id = jl.journal_entry_id
			WHERE je.company_id = ? AND je.status = 'posted' AND je.deleted_at IS NULL AND je.date <= ?
			GROUP BY jl.account_id
		) s ON s.account_id = a.id
		WHERE a.company_id = ? AND a.deleted_at IS NULL
		AND (COALESCE(s.beginning_debit,0) <> 0 OR COALESCE(s.beginning_credit,0) <> 0
		     OR COALESCE(s.period_debit,0) <> 0 OR COALESCE(s.period_credit,0) <> 0)
		ORDER BY a.code ASC
	`
	err := r.db.Raw(query, start, start, start, end, start, end, companyID, end, companyID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("trial balance query: %w", err)
	}
	return rows, nil
}

func (r *ReportRepository) EndingBalanceRows(companyID string, asOfDate time.Time) ([]domain.AccountBalanceRow, error) {
	return r.groupedBalanceRows(companyID, time.Time{}, asOfDate, []string{"asset", "liability", "equity"})
}

func (r *ReportRepository) IncomeExpenseRows(companyID string, start, end time.Time) ([]domain.AccountBalanceRow, error) {
	return r.groupedBalanceRows(companyID, start, end, []string{"income", "expense"})
}

// groupedBalanceRows — shared shape behind EndingBalanceRows (start is the
// zero time, meaning "all activity up to end") and IncomeExpenseRows (a real
// start..end window).
func (r *ReportRepository) groupedBalanceRows(companyID string, start, end time.Time, groups []string) ([]domain.AccountBalanceRow, error) {
	rows := make([]domain.AccountBalanceRow, 0)
	query := `
		SELECT
			a.id AS account_id, a.code, a.name, a."group",
			COALESCE(s.total_debit, 0)  AS total_debit,
			COALESCE(s.total_credit, 0) AS total_credit
		FROM accounts a
		LEFT JOIN (
			SELECT jl.account_id, SUM(jl.debit) AS total_debit, SUM(jl.credit) AS total_credit
			FROM journal_lines jl
			JOIN journal_entries je ON je.id = jl.journal_entry_id
			WHERE je.company_id = ? AND je.status = 'posted' AND je.deleted_at IS NULL
			  AND je.date >= ? AND je.date <= ?
			GROUP BY jl.account_id
		) s ON s.account_id = a.id
		WHERE a.company_id = ? AND a.deleted_at IS NULL AND a."group" IN ?
		ORDER BY a.code ASC
	`
	err := r.db.Raw(query, companyID, start, end, companyID, groups).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("grouped balance query: %w", err)
	}
	return rows, nil
}

func (r *ReportRepository) LedgerOpeningBalance(companyID, accountID string, start time.Time) (float64, float64, error) {
	var row struct{ Debit, Credit float64 }
	err := r.db.Raw(`
		SELECT COALESCE(SUM(jl.debit),0) AS debit, COALESCE(SUM(jl.credit),0) AS credit
		FROM journal_lines jl
		JOIN journal_entries je ON je.id = jl.journal_entry_id
		WHERE je.company_id = ? AND jl.account_id = ? AND je.status = 'posted'
		  AND je.deleted_at IS NULL AND je.date < ?
	`, companyID, accountID, start).Scan(&row).Error
	if err != nil {
		return 0, 0, fmt.Errorf("ledger opening balance: %w", err)
	}
	return row.Debit, row.Credit, nil
}

func (r *ReportRepository) LedgerLines(companyID, accountID string, start, end time.Time) ([]domain.LedgerLine, error) {
	lines := make([]domain.LedgerLine, 0)
	err := r.db.Raw(`
		SELECT je.date, je.number, COALESCE(NULLIF(jl.description, ''), je.description) AS description,
		       jl.debit, jl.credit
		FROM journal_lines jl
		JOIN journal_entries je ON je.id = jl.journal_entry_id
		WHERE je.company_id = ? AND jl.account_id = ? AND je.status = 'posted'
		  AND je.deleted_at IS NULL AND je.date >= ? AND je.date <= ?
		ORDER BY je.date ASC, je.created_at ASC
	`, companyID, accountID, start, end).Scan(&lines).Error
	if err != nil {
		return nil, fmt.Errorf("ledger lines: %w", err)
	}
	return lines, nil
}

func (r *ReportRepository) AccountByID(companyID, accountID string) (string, string, string, error) {
	var a model.Account
	err := r.db.Select("code", "name", "group").
		Where("id = ? AND company_id = ?", accountID, companyID).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", "", "", &domain.ErrValidation{Message: "account not found"}
	}
	if err != nil {
		return "", "", "", fmt.Errorf("find account %s: %w", accountID, err)
	}
	return a.Code, a.Name, string(a.Group), nil
}
