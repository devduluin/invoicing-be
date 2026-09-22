// Package activation is the Free-workspace activation milestone: Company profile + 3 Partners +
// 1 Invoice unlocks the Full Free limits. Duluin Invoice has no Product/Service master, so there is
// deliberately no product requirement, product count or product limit anywhere in this package.
package domain_activation

import (
	"fmt"
	"time"
)

const (
	StatusInitial   = "initial"
	StatusActivated = "activated"
)

// Unlimited marks a Limits field with no cap (currently only Users, on the Full tier). Never
// compare a raw count against it directly — use Reached below.
const Unlimited = -1

// Reached reports whether a used count has hit a limit, treating Unlimited as never-reached.
func Reached(used int64, limit int) bool {
	return limit >= 0 && used >= int64(limit)
}

// Limits are the caps this Free plan enforces. No product limit — see the package doc.
type Limits struct {
	Users                int `json:"users"`
	TransactionsPerMonth int `json:"transactions_per_month"`
	Partners             int `json:"partners"`
}

// InitialLimits apply before activation; FullLimits apply once activated. Companies is unlimited
// either way — a Duluin Invoice account can hold any number of companies, so it is not a separate
// enforced check.
var (
	InitialLimits = Limits{Users: 8, TransactionsPerMonth: 5, Partners: 10}
	FullLimits    = Limits{Users: Unlimited, TransactionsPerMonth: 20, Partners: 50}
)

func LimitsFor(status string) Limits {
	if status == StatusActivated {
		return FullLimits
	}
	return InitialLimits
}

// Requirement is one checklist step.
type Requirement struct {
	Key      string `json:"key"` // "company_profile" | "partners" | "invoice"
	Done     bool   `json:"done"`
	Current  int    `json:"current"`
	Required int    `json:"required"`
}

// Progress is what Settings/Overview shows: the checklist, and the limits that apply right now.
type Progress struct {
	Status       string        `json:"status"` // initial | activated
	ActivatedAt  *time.Time    `json:"activated_at,omitempty"`
	Completed    int           `json:"completed"`
	Total        int           `json:"total"`
	Requirements []Requirement `json:"requirements"`
	Limits       Limits        `json:"limits"`
}

// ErrLimitReached — a Free-tier cap was hit (HTTP 409). Wording avoids "upgrade"/"purchase"/"trial"
// per the product's own Free-plan language: it points at the activation checklist, not a paywall.
type ErrLimitReached struct {
	Resource  string // "partners" | "transactions_per_month" | "users"
	Limit     int
	Activated bool
}

func (e *ErrLimitReached) Error() string {
	label := map[string]string{
		"partners":               "partners",
		"transactions_per_month": "transactions this month",
		"users":                  "team members",
	}[e.Resource]
	if e.Activated {
		return fmt.Sprintf("You've reached your Free plan's limit of %d %s.", e.Limit, label)
	}
	return fmt.Sprintf("You've reached the Free plan's limit of %d %s. Complete your workspace setup (3 partners + 1 invoice) to unlock a higher limit.", e.Limit, label)
}

// ErrCompanyNotFound — the company row could not be found (should not happen for an authenticated,
// company-scoped request).
type ErrCompanyNotFound struct{}

func (e *ErrCompanyNotFound) Error() string { return "company not found" }
