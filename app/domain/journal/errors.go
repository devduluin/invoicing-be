package domain_journal

import "fmt"

type ErrNotFound struct{ ID string }

func (e *ErrNotFound) Error() string { return fmt.Sprintf("journal entry %s not found", e.ID) }

type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }

// ErrUnbalanced — total debit and total credit don't match.
type ErrUnbalanced struct{ Debit, Credit float64 }

func (e *ErrUnbalanced) Error() string {
	return fmt.Sprintf("total debit (%.2f) and total credit (%.2f) must match", e.Debit, e.Credit)
}

type ErrNumberExists struct{ Number string }

func (e *ErrNumberExists) Error() string {
	return fmt.Sprintf("journal no. %s is already in use", e.Number)
}

// ErrPostedImmutable — accounting-engine-service parity: a posted entry can't
// be edited or deleted directly; it must go back to draft first.
type ErrPostedImmutable struct{}

func (e *ErrPostedImmutable) Error() string {
	return "a posted journal entry can't be edited or deleted — move it back to draft first"
}

// ErrAccountInactive — a line references a deactivated COA account.
type ErrAccountInactive struct{}

func (e *ErrAccountInactive) Error() string {
	return "one of the accounts on this journal entry's lines is inactive"
}

// ErrNotDraft — Post() called on something that isn't a draft.
type ErrNotDraft struct{}

func (e *ErrNotDraft) Error() string { return "only a draft journal entry can be posted" }

// ErrNotPosted — BackToDraft() called on something that isn't posted.
type ErrNotPosted struct{}

func (e *ErrNotPosted) Error() string {
	return "only a posted journal entry can be moved back to draft"
}
