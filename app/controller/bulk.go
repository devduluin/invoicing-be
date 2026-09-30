package controller

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/validation"
	"duluin_invoice/utils"
)

// bulkIDsRequest is the shared body for every "act on N documents in one request" endpoint —
// capped at 100 so one request can't be used to smuggle an unbounded, slow operation.
type bulkIDsRequest struct {
	IDs []string `json:"ids" validate:"required,min=1,max=100,dive,uuid4"`
}

// BulkResult is one row of a bulk endpoint's response — success or the reason it failed, so a
// partial failure (e.g. one order out of ten still has a delivery note) doesn't hide behind a
// single aggregate error and doesn't stop the rest of the batch.
type BulkResult struct {
	ID      string `json:"id"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// parseBulkIDs reads and validates the shared {"ids": [...]} body, answering the request itself on
// failure. ok is false when the caller should stop (the response was already written).
func parseBulkIDs(c *fiber.Ctx) (ids []string, ok bool) {
	var body bulkIDsRequest
	if err := c.BodyParser(&body); err != nil {
		_ = utils.BadRequest(c, []string{"Invalid request body"})
		return nil, false
	}
	if msgs := validation.Struct(&body); msgs != nil {
		_ = utils.ValidationFailed(c, msgs)
		return nil, false
	}
	return body.IDs, true
}

// runBulk calls `one` for every id — the exact same single-document action (and its guards and audit
// entry) the one-row endpoint runs — collecting a per-id result, so one refused document (e.g. a
// confirmed invoice that already has payments) never stops the rest of the batch.
func runBulk(c *fiber.Ctx, ids []string, one func(id string) error, message string) error {
	results := make([]BulkResult, 0, len(ids))
	for _, id := range ids {
		if err := one(id); err != nil {
			results = append(results, BulkResult{ID: id, Success: false, Message: err.Error()})
			continue
		}
		results = append(results, BulkResult{ID: id, Success: true})
	}
	return utils.Ok(c, results, message)
}

// runBulkDelete calls `del` once per id — the exact same single-document Delete each document type
// already uses (so every existing guard, side effect and audit entry stays exactly as it is) — but
// as ONE HTTP round trip instead of N from the client. That round-trip cost is the real bottleneck
// a bulk action feels (SSO/RBAC validation runs once per request, same as any other endpoint), so
// collapsing N requests into 1 is most of the available speed-up without weakening any per-document
// safety check into a single unguarded batch SQL statement.
func runBulkDelete(c *fiber.Ctx, ids []string, del func(id string) error) error {
	return runBulk(c, ids, del, "Bulk delete processed")
}
