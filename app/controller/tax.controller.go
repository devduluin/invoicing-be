package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	domain "duluin_invoice/app/domain/tax"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type TaxController struct{ svc domain.IService }

func NewTaxController(svc domain.IService) *TaxController { return &TaxController{svc: svc} }

func (ctrl *TaxController) List(c *fiber.Ctx) error {
	res, err := ctrl.svc.List(&domain.Filter{
		CompanyID: middlewares.GetCompanyID(c),
		Search:    c.Query("search"),
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

func (ctrl *TaxController) Get(c *fiber.Ctx) error {
	row, err := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"))
	if err != nil {
		return taxErr(c, err)
	}
	return utils.Ok(c, row, "OK")
}

func (ctrl *TaxController) Create(c *fiber.Ctx) error {
	var dto domain.CreateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.Create(middlewares.GetCompanyID(c), middlewares.GetUserID(c), &dto)
	if err != nil {
		return taxErr(c, err)
	}
	return utils.Created(c, row, "Tax added")
}

func (ctrl *TaxController) Update(c *fiber.Ctx) error {
	var dto domain.UpdateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.Update(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"), &dto)
	if err != nil {
		return taxErr(c, err)
	}
	return utils.Ok(c, row, "Tax updated")
}

func (ctrl *TaxController) Delete(c *fiber.Ctx) error {
	if err := ctrl.svc.Delete(middlewares.GetCompanyID(c), c.Params("id")); err != nil {
		return taxErr(c, err)
	}
	return utils.Deleted(c, "Tax deleted")
}

func taxErr(c *fiber.Ctx, err error) error {
	var nf *domain.ErrNotFound
	if errors.As(err, &nf) {
		return utils.NotFound(c, []string{err.Error()})
	}
	var locked *domain.ErrSystemLocked
	if errors.As(err, &locked) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": err.Error(), "error_code": "system_locked"})
	}
	var inUse *domain.ErrComponentInUse
	if errors.As(err, &inUse) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": err.Error(), "error_code": "component_in_use"})
	}
	var v *domain.ErrValidation
	if errors.As(err, &v) {
		return utils.ValidationFailed(c, []string{err.Error()})
	}
	return utils.InternalError(c, err)
}
