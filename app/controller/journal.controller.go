package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	domain "duluin_invoice/app/domain/journal"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type JournalController struct{ svc domain.IService }

func NewJournalController(svc domain.IService) *JournalController {
	return &JournalController{svc: svc}
}

func (ctrl *JournalController) List(c *fiber.Ctx) error {
	res, err := ctrl.svc.List(&domain.Filter{
		CompanyID:     middlewares.GetCompanyID(c),
		Search:        c.Query("search"),
		JournalBookID: c.Query("journal_book_id"),
		Status:        c.Query("status"),
		Page:          c.QueryInt("page", 1),
		PageSize:      c.QueryInt("limit", 20),
		Sort:          c.Query("sort"),
		Order:         c.Query("order"),
		Fields:        utils.ParseCSVParam(c.Query("fields")),
	})
	if err != nil {
		return utils.InternalError(c, err)
	}
	return utils.List(c, res, "OK")
}

func (ctrl *JournalController) Get(c *fiber.Ctx) error {
	row, err := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"))
	if err != nil {
		return journalErr(c, err)
	}
	return utils.Ok(c, row, "OK")
}

func (ctrl *JournalController) Create(c *fiber.Ctx) error {
	var dto domain.CreateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.Create(middlewares.GetCompanyID(c), middlewares.GetUserID(c), &dto)
	if err != nil {
		return journalErr(c, err)
	}
	return utils.Created(c, row, "Journal entry added")
}

func (ctrl *JournalController) Update(c *fiber.Ctx) error {
	var dto domain.UpdateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.Update(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"), &dto)
	if err != nil {
		return journalErr(c, err)
	}
	return utils.Ok(c, row, "Journal entry updated")
}

func (ctrl *JournalController) Delete(c *fiber.Ctx) error {
	if err := ctrl.svc.Delete(middlewares.GetCompanyID(c), c.Params("id")); err != nil {
		return journalErr(c, err)
	}
	return utils.Deleted(c, "Journal entry deleted")
}

func (ctrl *JournalController) Post(c *fiber.Ctx) error {
	row, err := ctrl.svc.Post(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"))
	if err != nil {
		return journalErr(c, err)
	}
	return utils.Ok(c, row, "Journal entry posted")
}

func (ctrl *JournalController) BackToDraft(c *fiber.Ctx) error {
	row, err := ctrl.svc.BackToDraft(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"))
	if err != nil {
		return journalErr(c, err)
	}
	return utils.Ok(c, row, "Journal entry moved back to draft")
}

func journalErr(c *fiber.Ctx, err error) error {
	var nf *domain.ErrNotFound
	if errors.As(err, &nf) {
		return utils.NotFound(c, []string{err.Error()})
	}
	var dup *domain.ErrNumberExists
	if errors.As(err, &dup) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": err.Error(), "error_code": "number_exists"})
	}
	var immutable *domain.ErrPostedImmutable
	if errors.As(err, &immutable) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": err.Error(), "error_code": "posted_immutable"})
	}
	var inactive *domain.ErrAccountInactive
	if errors.As(err, &inactive) {
		return utils.ValidationFailed(c, []string{err.Error()})
	}
	var notDraft *domain.ErrNotDraft
	if errors.As(err, &notDraft) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": err.Error(), "error_code": "not_draft"})
	}
	var notPosted *domain.ErrNotPosted
	if errors.As(err, &notPosted) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": err.Error(), "error_code": "not_posted"})
	}
	var unbalanced *domain.ErrUnbalanced
	if errors.As(err, &unbalanced) {
		return utils.ValidationFailed(c, []string{err.Error()})
	}
	var v *domain.ErrValidation
	if errors.As(err, &v) {
		return utils.ValidationFailed(c, []string{err.Error()})
	}
	return utils.InternalError(c, err)
}
