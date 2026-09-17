package domain_onboarding

// ErrValidation — a field-level / business rule failed (maps to HTTP 422).
type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }

// ErrAlreadyOnboarded — the caller already has a company for this active context
// and re-ran onboarding without asking for a new one (maps to HTTP 409).
type ErrAlreadyOnboarded struct{}

func (e *ErrAlreadyOnboarded) Error() string { return "company already created" }
