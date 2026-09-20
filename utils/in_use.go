package utils

import "github.com/gofiber/fiber/v2"

// ErrInUse — a record can't be deleted because live documents still reference it.
// (Soft delete keeps the row, so dangling references would silently point at a
// hidden record; refusing is the only consistent answer.)
type ErrInUse struct{ Message string }

func (e *ErrInUse) Error() string { return e.Message }

// InUse renders ErrInUse as 409 with a machine-readable code. The frontend never
// treats a 409 as a dead session.
func InUse(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusConflict).JSON(fiber.Map{
		"success":    false,
		"message":    message,
		"error_code": "in_use",
	})
}
