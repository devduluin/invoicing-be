package domain_mitra

import "fmt"

type ErrNotFound struct{ ID string }

func (e *ErrNotFound) Error() string { return fmt.Sprintf("mitra %s not found", e.ID) }

type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }

// ErrCompanyLinked — the Duluin company is already linked to another partner of this company.
type ErrCompanyLinked struct{ Code, Partner string }

func (e *ErrCompanyLinked) Error() string {
	if e.Partner == "" {
		return fmt.Sprintf("Duluin Company Code %s is already linked to another partner", e.Code)
	}
	return fmt.Sprintf("Duluin Company Code %s is already linked to partner %s", e.Code, e.Partner)
}

// ErrCodeExists — another active partner of the company already uses this code.
type ErrCodeExists struct{ Code string }

func (e *ErrCodeExists) Error() string {
	return fmt.Sprintf("partner code %s is already in use", e.Code)
}
