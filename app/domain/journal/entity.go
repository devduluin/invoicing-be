// Package domain_journal — manual double-entry journal entries ("Ayat Jurnal").
// The first transactional (non-master-data) record in invoice-service: a
// header plus ≥2 balanced debit/credit lines.
package domain_journal

// JournalLineDTO is one line of a journal entry. Exactly one of Debit/Credit
// must be > 0 — enforced in the service (cross-field rules don't fit struct
// tags well), not here.
type JournalLineDTO struct {
	AccountID   string  `json:"account_id"  validate:"required,uuid4"`
	MitraID     *string `json:"mitra_id"    validate:"omitempty,uuid4"`
	Description string  `json:"description" validate:"omitempty,max=255"`
	Debit       float64 `json:"debit"       validate:"gte=0"`
	Credit      float64 `json:"credit"      validate:"gte=0"`
}

// CreateDTO — Number is optional; the service auto-generates one (from the
// journal book's Code + period) when blank. IdempotencyKey is optional at the
// API layer (unlike accounting-engine-service, which requires it) — the
// invoice-frontend always sends one, but there's no automated/API-driven
// posting path here yet to justify hard-requiring it.
type CreateDTO struct {
	CompanyID      string           `json:"-"`
	JournalBookID  string           `json:"journal_book_id" validate:"required,uuid4"`
	Number         string           `json:"number"           validate:"omitempty,max=50"`
	Description    string           `json:"description"      validate:"required,max=255"`
	Date           string           `json:"date"             validate:"required"` // YYYY-MM-DD
	IdempotencyKey string           `json:"idempotency_key"  validate:"omitempty,max=100"`
	Lines          []JournalLineDTO `json:"lines"            validate:"required,min=2,dive"`
}

// UpdateDTO — a full replace: header fields + the complete new line set.
// Rejected outright by the service once the entry is Posted.
type UpdateDTO struct {
	JournalBookID string           `json:"journal_book_id" validate:"required,uuid4"`
	Number        string           `json:"number"           validate:"omitempty,max=50"`
	Description   string           `json:"description"      validate:"required,max=255"`
	Date          string           `json:"date"             validate:"required"`
	Lines         []JournalLineDTO `json:"lines"            validate:"required,min=2,dive"`
}

// Filter drives the paginated list query.
type Filter struct {
	CompanyID     string
	Search        string
	JournalBookID string
	Status        string
	Page          int
	PageSize      int
	Sort          string
	Order         string
	Fields        []string
}
