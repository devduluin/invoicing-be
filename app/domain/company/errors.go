package domain_company

type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }

type ErrCompanyNotFound struct{}

func (e *ErrCompanyNotFound) Error() string { return "active company not found" }
