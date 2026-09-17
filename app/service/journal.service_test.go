package service

import (
	"errors"
	"testing"

	domain "duluin_invoice/app/domain/journal"
)

func line(accountID string, debit, credit float64) domain.JournalLineDTO {
	return domain.JournalLineDTO{AccountID: accountID, Debit: debit, Credit: credit}
}

func TestValidateLines(t *testing.T) {
	cases := []struct {
		name    string
		lines   []domain.JournalLineDTO
		wantErr any // nil, or a target type via errors.As
	}{
		{
			name:  "balanced valid entry",
			lines: []domain.JournalLineDTO{line("acc-1", 100, 0), line("acc-2", 0, 100)},
		},
		{
			name:    "fewer than 2 lines",
			lines:   []domain.JournalLineDTO{line("acc-1", 100, 0)},
			wantErr: &domain.ErrValidation{},
		},
		{
			name:    "missing account on a line",
			lines:   []domain.JournalLineDTO{line("", 100, 0), line("acc-2", 0, 100)},
			wantErr: &domain.ErrValidation{},
		},
		{
			name:    "line with both debit and credit",
			lines:   []domain.JournalLineDTO{line("acc-1", 100, 50), line("acc-2", 0, 100)},
			wantErr: &domain.ErrValidation{},
		},
		{
			name:    "line with neither debit nor credit",
			lines:   []domain.JournalLineDTO{line("acc-1", 100, 0), line("acc-2", 0, 0)},
			wantErr: &domain.ErrValidation{},
		},
		{
			name:    "zero total",
			lines:   []domain.JournalLineDTO{line("acc-1", 0, 0), line("acc-2", 0, 0)},
			wantErr: &domain.ErrValidation{},
		},
		{
			name:    "unbalanced totals",
			lines:   []domain.JournalLineDTO{line("acc-1", 100, 0), line("acc-2", 0, 90)},
			wantErr: &domain.ErrUnbalanced{},
		},
		{
			name: "balanced across three lines",
			lines: []domain.JournalLineDTO{
				line("acc-1", 150, 0),
				line("acc-2", 0, 100),
				line("acc-3", 0, 50),
			},
		},
		{
			// balanceEpsilon is 0.0001 (accounting-engine-service parity) —
			// tight enough to still catch a genuine 1-cent mismatch, loose
			// enough to absorb float64 summation noise.
			name:  "within rounding epsilon",
			lines: []domain.JournalLineDTO{line("acc-1", 100.00001, 0), line("acc-2", 0, 100)},
		},
		{
			name:    "a 1-cent mismatch is still caught by the tighter epsilon",
			lines:   []domain.JournalLineDTO{line("acc-1", 100.01, 0), line("acc-2", 0, 100)},
			wantErr: &domain.ErrUnbalanced{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateLines(tc.lines)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			switch tc.wantErr.(type) {
			case *domain.ErrValidation:
				var target *domain.ErrValidation
				if !errors.As(err, &target) {
					t.Fatalf("expected *domain.ErrValidation, got %T (%v)", err, err)
				}
			case *domain.ErrUnbalanced:
				var target *domain.ErrUnbalanced
				if !errors.As(err, &target) {
					t.Fatalf("expected *domain.ErrUnbalanced, got %T (%v)", err, err)
				}
			}
		})
	}
}
