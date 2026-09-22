package utils

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

type ResponseData struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
}

type ErrorResponse struct {
	Success bool     `json:"success"`
	Message string   `json:"message"`
	Errors  []string `json:"errors,omitempty"`
}

func Ok(c *fiber.Ctx, data interface{}, message string) error {
	return c.Status(fiber.StatusOK).JSON(ResponseData{Success: true, Message: message, Data: data})
}

func OkWithMeta(c *fiber.Ctx, data interface{}, meta interface{}, message string) error {
	return c.Status(fiber.StatusOK).JSON(ResponseData{Success: true, Message: message, Data: data, Meta: meta})
}

// List returns the uniform list envelope the frontend MasterTable expects
// (matches acc-master-service): data + the column catalog + pagination meta.
func List(c *fiber.Ctx, r *OffsetPaginationResult, message string) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":    true,
		"message":    message,
		"data":       r.Data,
		"columns":    r.Columns,
		"attributes": r.Attributes,
		"meta":       r.Meta,
	})
}

func Created(c *fiber.Ctx, data interface{}, message string) error {
	return c.Status(fiber.StatusCreated).JSON(ResponseData{Success: true, Message: message, Data: data})
}

func Deleted(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusOK).JSON(ResponseData{Success: true, Message: message})
}

func errResponse(c *fiber.Ctx, status int, message string, errs []string) error {
	return c.Status(status).JSON(ErrorResponse{Success: false, Message: message, Errors: errs})
}

func first(messages []string, fallback string) string {
	if len(messages) > 0 && messages[0] != "" {
		return messages[0]
	}
	return fallback
}

func BadRequest(c *fiber.Ctx, messages []string) error {
	return errResponse(c, fiber.StatusBadRequest, first(messages, "Bad Request"), messages)
}

func Unauthorized(c *fiber.Ctx, messages []string) error {
	return errResponse(c, fiber.StatusUnauthorized, first(messages, "Unauthorized"), messages)
}

func Forbidden(c *fiber.Ctx, messages []string) error {
	return errResponse(c, fiber.StatusForbidden, first(messages, "Forbidden"), messages)
}

func NotFound(c *fiber.Ctx, messages []string) error {
	return errResponse(c, fiber.StatusNotFound, first(messages, "Not Found"), messages)
}

func ValidationFailed(c *fiber.Ctx, messages []string) error {
	return errResponse(c, fiber.StatusUnprocessableEntity, first(messages, "Validation Error"), messages)
}

func Conflict(c *fiber.Ctx, messages []string) error {
	return errResponse(c, fiber.StatusConflict, first(messages, "Conflict"), messages)
}

// InternalError logs the real error server-side (with request id) and returns a
// generic message — internal/DB details must not leak to the client.
func InternalError(c *fiber.Ctx, err error) error {
	reqID, _ := c.Locals("requestid").(string)
	log.Printf("[internal-error] request_id=%s path=%s err=%v", reqID, c.Path(), err)
	return errResponse(c, fiber.StatusInternalServerError, "Terjadi kesalahan pada server", nil)
}

func ServiceUnavailable(c *fiber.Ctx, messages []string) error {
	return errResponse(c, fiber.StatusServiceUnavailable, first(messages, "Service Unavailable"), messages)
}
