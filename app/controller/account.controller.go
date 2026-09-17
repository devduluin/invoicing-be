package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	domain "duluin_invoice/app/domain/account"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type AccountController struct{ svc domain.IService }

func NewAccountController(svc domain.IService) *AccountController {
	return &AccountController{svc: svc}
}

func (ctrl *AccountController) List(c *fiber.Ctx) error {
	res, err := ctrl.svc.List(&domain.Filter{
		CompanyID: middlewares.GetCompanyID(c),
		Search:    c.Query("search"),
		Group:     c.Query("group"),
		Page:      c.QueryInt("page", 1),
		PageSize:  c.QueryInt("limit", 20),
		Sort:      c.Query("sort"),
		Order:     c.Query("order"),
		Fields:    utils.ParseCSVParam(c.Query("fields")),
	})
	if err != nil {
		return utils.InternalError(c, err)
	}
	return utils.List(c, res, "OK")
}

func (ctrl *AccountController) Get(c *fiber.Ctx) error {
	row, err := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"))
	if err != nil {
		return accountErr(c, err)
	}
	return utils.Ok(c, row, "OK")
}

func (ctrl *AccountController) Create(c *fiber.Ctx) error {
	var dto domain.CreateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.Create(middlewares.GetCompanyID(c), middlewares.GetUserID(c), &dto)
	if err != nil {
		return accountErr(c, err)
	}
	return utils.Created(c, row, "Account added")
}

func (ctrl *AccountController) Update(c *fiber.Ctx) error {
	var dto domain.UpdateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.Update(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"), &dto)
	if err != nil {
		return accountErr(c, err)
	}
	return utils.Ok(c, row, "Account updated")
}

func (ctrl *AccountController) Delete(c *fiber.Ctx) error {
	if err := ctrl.svc.Delete(middlewares.GetCompanyID(c), c.Params("id")); err != nil {
		return accountErr(c, err)
	}
	return utils.Deleted(c, "Account deleted")
}

func accountErr(c *fiber.Ctx, err error) error {
	var nf *domain.ErrNotFound
	if errors.As(err, &nf) {
		return utils.NotFound(c, []string{err.Error()})
	}
	var dup *domain.ErrCodeExists
	if errors.As(err, &dup) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": err.Error(), "error_code": "code_exists"})
	}
	var kids *domain.ErrHasChildren
	if errors.As(err, &kids) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": err.Error(), "error_code": "has_children"})
	}
	var locked *domain.ErrSystemLocked
	if errors.As(err, &locked) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": err.Error(), "error_code": "system_locked"})
	}
	var v *domain.ErrValidation
	if errors.As(err, &v) {
		return utils.ValidationFailed(c, []string{err.Error()})
	}
	return utils.InternalError(c, err)
}
