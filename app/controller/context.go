package controller

import (
	"encoding/json"
	"errors"

	"github.com/gofiber/fiber/v2"

	activation "duluin_invoice/app/domain/activation"
	audit "duluin_invoice/app/domain/audit"
	membership "duluin_invoice/app/domain/membership"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

// handleActivationError answers the two errors ActivationService can raise. Every Create handler
// that enforces a Free-tier limit (partners, transactions/month) checks this FIRST, alongside its
// own existing error mapping. Returns false when err isn't one of these, so callers fall through
// to their own switch unchanged.
func handleActivationError(c *fiber.Ctx, err error) (handled bool, resp error) {
	var limit *activation.ErrLimitReached
	if errors.As(err, &limit) {
		return true, c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"success": false, "message": err.Error(), "error_code": "limit_reached", "resource": limit.Resource, "limit": limit.Limit,
		})
	}
	var notFound *activation.ErrCompanyNotFound
	if errors.As(err, &notFound) {
		return true, utils.NotFound(c, []string{err.Error()})
	}
	return false, nil
}

// jsonFieldDiff compares two JSON objects key by key so an audit entry can show "labels changed",
// "notes changed", … instead of one unreadable blob. A key only appears when its value actually
// changed (raw-string comparison — good enough for a diagnostic diff, not a semantic one).
func jsonFieldDiff(beforeJSON, afterJSON string) map[string]audit.Change {
	var before, after map[string]json.RawMessage
	_ = json.Unmarshal([]byte(beforeJSON), &before)
	_ = json.Unmarshal([]byte(afterJSON), &after)

	changes := map[string]audit.Change{}
	for k, bv := range before {
		av, ok := after[k]
		if !ok || string(av) != string(bv) {
			changes[k] = audit.Change{Before: rawJSONValue(bv), After: rawJSONValue(av)}
		}
	}
	for k, av := range after {
		if _, seen := before[k]; !seen {
			changes[k] = audit.Change{Before: nil, After: rawJSONValue(av)}
		}
	}
	return changes
}

func rawJSONValue(r json.RawMessage) any {
	if len(r) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(r, &v); err != nil {
		return string(r)
	}
	return v
}

// membershipActor flattens the request context into a membership.Actor.
func membershipActor(c *fiber.Ctx) membership.Actor {
	return membership.Actor{
		UserID:          middlewares.GetUserID(c),
		Email:           middlewares.GetEmail(c),
		Name:            middlewares.GetName(c),
		ActiveCompanyID: middlewares.GetCompanyID(c),
		Token:           c.Get("Authorization"),
	}
}

// displayName picks the human-readable label for an audit entry's target: the person's name when
// known, else their email.
func displayName(name, email string) string {
	if name != "" {
		return name
	}
	return email
}

// auditActor flattens the request context into an audit.Actor — the one place every controller
// that logs an audit entry reads "who, from where" from, so it's never duplicated per call site.
func auditActor(c *fiber.Ctx) audit.Actor {
	return audit.Actor{
		CompanyID: middlewares.GetCompanyID(c),
		UserID:    middlewares.GetUserID(c),
		Name:      middlewares.GetName(c),
		Email:     middlewares.GetEmail(c),
		IPAddress: c.IP(),
		UserAgent: c.Get("User-Agent"),
	}
}
