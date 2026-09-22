package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	domain "duluin_invoice/app/domain/contactperson"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type ContactPersonController struct{ svc domain.IService }

func NewContactPersonController(svc domain.IService) *ContactPersonController {
	return &ContactPersonController{svc: svc}
}

func handleContactError(c *fiber.Ctx, err error) error {
	var notFound *domain.ErrNotFound
	if errors.As(err, &notFound) {
		return utils.NotFound(c, []string{err.Error()})
	}
	var invalid *domain.ErrValidation
	if errors.As(err, &invalid) {
		return utils.ValidationFailed(c, []string{err.Error()})
	}
	var dup *domain.ErrDuplicate
	if errors.As(err, &dup) {
		return utils.ValidationFailed(c, []string{err.Error()})
	}
	return utils.InternalError(c, err)
}

func (ctrl *ContactPersonController) List(c *fiber.Ctx) error {
	rows, err := ctrl.svc.List(middlewares.GetCompanyID(c), c.Params("id"), c.Query("search"))
	if err != nil {
		return handleContactError(c, err)
	}
	return utils.Ok(c, rows, "OK")
}

func (ctrl *ContactPersonController) Get(c *fiber.Ctx) error {
	row, err := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"), c.Params("cid"))
	if err != nil {
		return handleContactError(c, err)
	}
	return utils.Ok(c, row, "OK")
}

func (ctrl *ContactPersonController) Create(c *fiber.Ctx) error {
	var in domain.Input
	if err := c.BodyParser(&in); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&in); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.Create(middlewares.GetCompanyID(c), c.Params("id"), middlewares.GetUserID(c), &in)
	if err != nil {
		return handleContactError(c, err)
	}
	return utils.Created(c, row, "Contact person added")
}

func (ctrl *ContactPersonController) Update(c *fiber.Ctx) error {
	var in domain.Input
	if err := c.BodyParser(&in); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&in); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.Update(middlewares.GetCompanyID(c), c.Params("id"), c.Params("cid"), middlewares.GetUserID(c), &in)
	if err != nil {
		return handleContactError(c, err)
	}
	return utils.Ok(c, row, "Contact person updated")
}

func (ctrl *ContactPersonController) Delete(c *fiber.Ctx) error {
	if err := ctrl.svc.Delete(middlewares.GetCompanyID(c), c.Params("id"), c.Params("cid")); err != nil {
		return handleContactError(c, err)
	}
	return utils.Deleted(c, "Contact person deleted")
}

func (ctrl *ContactPersonController) Summaries(c *fiber.Ctx) error {
	rows, err := ctrl.svc.Summaries(middlewares.GetCompanyID(c))
	if err != nil {
		return handleContactError(c, err)
	}
	return utils.Ok(c, rows, "OK")
}
