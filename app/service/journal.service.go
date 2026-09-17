package service

import (
	"fmt"
	"math"
	"strings"

	domain "duluin_invoice/app/domain/journal"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type JournalService struct{ repo domain.IRepository }

func NewJournalService(repo domain.IRepository) domain.IService { return &JournalService{repo: repo} }

func (s *JournalService) Create(companyID, actorID string, dto *domain.CreateDTO) (*model.JournalEntry, error) {
	if key := strings.TrimSpace(dto.IdempotencyKey); key != "" {
		if existing, err := s.repo.FindByIdempotencyKey(companyID, key); err != nil {
			return nil, err
		} else if existing != nil {
			return existing, nil
		}
	}
	if err := validateLines(dto.Lines); err != nil {
		return nil, err
	}
	if err := s.checkAccountsActive(companyID, dto.Lines); err != nil {
		return nil, err
	}
	dto.CompanyID = companyID
	return s.repo.Create(dto, actorID)
}

func (s *JournalService) Update(companyID, actorID, id string, dto *domain.UpdateDTO) (*model.JournalEntry, error) {
	existing, err := s.repo.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}
	if existing.Status == model.JournalEntryStatusPosted {
		return nil, &domain.ErrPostedImmutable{}
	}
	if err := validateLines(dto.Lines); err != nil {
		return nil, err
	}
	if err := s.checkAccountsActive(companyID, dto.Lines); err != nil {
		return nil, err
	}
	return s.repo.Update(companyID, id, dto, actorID)
}

func (s *JournalService) Get(companyID, id string) (*model.JournalEntry, error) {
	return s.repo.FindByID(companyID, id)
}

func (s *JournalService) List(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	return s.repo.FindAll(f)
}

func (s *JournalService) Delete(companyID, id string) error {
	existing, err := s.repo.FindByID(companyID, id)
	if err != nil {
		return err
	}
	// Adaptation from accounting-engine-service: it allows deleting a posted
	// entry and instead blocks on linked transactions/payments/bank-recon
	// rows, none of which exist in invoice-service yet — Status is the only
	// safety net available here, so it has to carry that job instead.
	if existing.Status == model.JournalEntryStatusPosted {
		return &domain.ErrPostedImmutable{}
	}
	return s.repo.Delete(companyID, id)
}

func (s *JournalService) Post(companyID, actorID, id string) (*model.JournalEntry, error) {
	entry, err := s.repo.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}
	if entry.Status != model.JournalEntryStatusDraft {
		return nil, &domain.ErrNotDraft{}
	}

	// Re-validate before posting — mirrors PostJournal in accounting-engine-service.
	lines := make([]domain.JournalLineDTO, len(entry.Lines))
	for i, l := range entry.Lines {
		lines[i] = domain.JournalLineDTO{AccountID: l.AccountID, Debit: l.Debit, Credit: l.Credit}
	}
	if err := validateLines(lines); err != nil {
		return nil, err
	}
	if err := s.checkAccountsActive(companyID, lines); err != nil {
		return nil, err
	}

	if err := s.repo.SetStatus(companyID, id, actorID, model.JournalEntryStatusPosted); err != nil {
		return nil, err
	}
	return s.repo.FindByID(companyID, id)
}

func (s *JournalService) BackToDraft(companyID, actorID, id string) (*model.JournalEntry, error) {
	entry, err := s.repo.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}
	if entry.Status != model.JournalEntryStatusPosted {
		return nil, &domain.ErrNotPosted{}
	}
	if err := s.repo.SetStatus(companyID, id, actorID, model.JournalEntryStatusDraft); err != nil {
		return nil, err
	}
	return s.repo.FindByID(companyID, id)
}

func (s *JournalService) checkAccountsActive(companyID string, lines []domain.JournalLineDTO) error {
	ids := make([]string, 0, len(lines))
	seen := make(map[string]bool, len(lines))
	for _, l := range lines {
		if l.AccountID == "" || seen[l.AccountID] {
			continue
		}
		seen[l.AccountID] = true
		ids = append(ids, l.AccountID)
	}
	if len(ids) == 0 {
		return nil
	}
	ok, err := s.repo.AccountsActive(companyID, ids)
	if err != nil {
		return err
	}
	if !ok {
		return &domain.ErrAccountInactive{}
	}
	return nil
}

// balanceEpsilon matches accounting-engine-service's ValidateJournal exactly
// (tighter than a naive 0.01 — money is 2-decimal, so float64 sums over a
// handful of lines never drift anywhere near this).
const balanceEpsilon = 0.0001

// validateLines enforces the double-entry rules the reference UI itself
// follows, matching accounting-engine-service's domain.ValidateJournal: at
// least 2 lines, every line has an account and exactly one of debit/credit
// filled, and the sheet balances with a non-zero total.
func validateLines(lines []domain.JournalLineDTO) error {
	if len(lines) < 2 {
		return &domain.ErrValidation{Message: "a journal entry must have at least 2 lines"}
	}

	var totalDebit, totalCredit float64
	for i, l := range lines {
		if strings.TrimSpace(l.AccountID) == "" {
			return &domain.ErrValidation{Message: fmt.Sprintf("line %d has no account selected", i+1)}
		}
		hasDebit := l.Debit > 0
		hasCredit := l.Credit > 0
		if hasDebit && hasCredit {
			return &domain.ErrValidation{Message: fmt.Sprintf("line %d can't have both debit and credit filled in", i+1)}
		}
		if !hasDebit && !hasCredit {
			return &domain.ErrValidation{Message: fmt.Sprintf("line %d must have either debit or credit filled in", i+1)}
		}
		totalDebit += l.Debit
		totalCredit += l.Credit
	}

	if totalDebit <= 0 {
		return &domain.ErrValidation{Message: "the journal entry total cannot be zero"}
	}
	if math.Abs(totalDebit-totalCredit) > balanceEpsilon {
		return &domain.ErrUnbalanced{Debit: totalDebit, Credit: totalCredit}
	}
	return nil
}
