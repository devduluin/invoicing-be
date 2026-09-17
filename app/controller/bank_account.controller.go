package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	domain "duluin_invoice/app/domain/bankaccount"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type BankAccountController struct{ svc domain.IService }

func NewBankAccountController(svc domain.IService) *BankAccountController {
	return &BankAccountController{svc: svc}
}

func (ctrl *BankAccountController) List(c *fiber.Ctx) error {
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

func (ctrl *BankAccountController) Get(c *fiber.Ctx) error {
	row, err := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"))
	if err != nil {
		return bankAccountErr(c, err)
	}
	return utils.Ok(c, row, "OK")
}

func (ctrl *BankAccountController) Create(c *fiber.Ctx) error {
	var dto domain.CreateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.Create(middlewares.GetCompanyID(c), middlewares.GetUserID(c), &dto)
	if err != nil {
		return bankAccountErr(c, err)
	}
	return utils.Created(c, row, "Bank account added")
}

func (ctrl *BankAccountController) Update(c *fiber.Ctx) error {
	var dto domain.UpdateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.Update(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"), &dto)
	if err != nil {
		return bankAccountErr(c, err)
	}
	return utils.Ok(c, row, "Bank account updated")
}

func (ctrl *BankAccountController) Delete(c *fiber.Ctx) error {
	if err := ctrl.svc.Delete(middlewares.GetCompanyID(c), c.Params("id")); err != nil {
		return bankAccountErr(c, err)
	}
	return utils.Deleted(c, "Bank account deleted")
}

func bankAccountErr(c *fiber.Ctx, err error) error {
	var nf *domain.ErrNotFound
	if errors.As(err, &nf) {
		return utils.NotFound(c, []string{err.Error()})
	}
	var v *domain.ErrValidation
	if errors.As(err, &v) {
		return utils.ValidationFailed(c, []string{err.Error()})
	}
	return utils.InternalError(c, err)
}
