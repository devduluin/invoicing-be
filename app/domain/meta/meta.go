// Package domain_meta — read-only reference data shared across the product
// (bank directory, etc). Sourced from the Duluin dashboard meta API.
package domain_meta

import "context"

// Bank is one entry of the Indonesian bank directory (SSO /api/meta/bank).
type Bank struct {
	Name      string `json:"name"`
	Code      string `json:"code"`
	SwiftCode string `json:"swift_code,omitempty"`
}

type IBankDirectory interface {
	// ListBanks returns the full directory, sorted by name. Cached.
	ListBanks(ctx context.Context) ([]Bank, error)
}
