package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	domain "duluin_invoice/app/domain/salesorder"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type SalesOrderController struct{ svc domain.IService }

func NewSalesOrderController(svc domain.IService) *SalesOrderController {
	return &SalesOrderController{svc: svc}
}

func (ctrl *SalesOrderController) List(c *fiber.Ctx) error {
	res, err := ctrl.svc.List(&domain.Filter{
		CompanyID: middlewares.GetCompanyID(c),
		Search:    c.Query("search"),
		MitraID:   c.Query("mitra_id"),
		Status:    c.Query("status"),
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

func (ctrl *SalesOrderController) PreviewNumber(c *fiber.Ctx) error {
	number, err := ctrl.svc.PreviewNumber(middlewares.GetCompanyID(c))
	if err != nil {
		return salesOrderErr(c, err)
	}
	return utils.Ok(c, fiber.Map{"number": number}, "OK")
}

func (ctrl *SalesOrderController) Get(c *fiber.Ctx) error {
	row, err := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"))
	if err != nil {
		return salesOrderErr(c, err)
	}
	return utils.Ok(c, row, "OK")
}

func (ctrl *SalesOrderController) Create(c *fiber.Ctx) error {
	var dto domain.CreateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.Create(middlewares.GetCompanyID(c), middlewares.GetUserID(c), &dto)
	if err != nil {
		return salesOrderErr(c, err)
	}
	return utils.Created(c, row, "Sales order added")
}

func (ctrl *SalesOrderController) Update(c *fiber.Ctx) error {
	var dto domain.UpdateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.Update(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"), &dto)
	if err != nil {
		return salesOrderErr(c, err)
	}
	return utils.Ok(c, row, "Sales order updated")
}

func (ctrl *SalesOrderController) Delete(c *fiber.Ctx) error {
	if err := ctrl.svc.Delete(middlewares.GetCompanyID(c), c.Params("id")); err != nil {
		return salesOrderErr(c, err)
	}
	return utils.Deleted(c, "Sales order deleted")
}

func (ctrl *SalesOrderController) Confirm(c *fiber.Ctx) error {
	row, err := ctrl.svc.Confirm(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"))
	if err != nil {
		return salesOrderErr(c, err)
	}
	return utils.Ok(c, row, "Sales order confirmed")
}

func (ctrl *SalesOrderController) BackToDraft(c *fiber.Ctx) error {
	row, err := ctrl.svc.BackToDraft(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"))
	if err != nil {
		return salesOrderErr(c, err)
	}
	return utils.Ok(c, row, "Sales order moved back to draft")
}

func (ctrl *SalesOrderController) Cancel(c *fiber.Ctx) error {
	row, err := ctrl.svc.Cancel(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"))
	if err != nil {
		return salesOrderErr(c, err)
	}
	return utils.Ok(c, row, "Sales order cancelled")
}

func salesOrderErr(c *fiber.Ctx, err error) error {
	if handled, resp := handleActivationError(c, err); handled {
		return resp
	}
	var nf *domain.ErrNotFound
	if errors.As(err, &nf) {
		return utils.NotFound(c, []string{err.Error()})
	}
	var dup *domain.ErrNumberExists
	if errors.As(err, &dup) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": err.Error(), "error_code": "number_exists"})
	}
	var notEditable *domain.ErrNotEditable
	if errors.As(err, &notEditable) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": err.Error(), "error_code": "not_editable"})
	}
	var invalidTransition *domain.ErrInvalidTransition
	if errors.As(err, &invalidTransition) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": err.Error(), "error_code": "invalid_transition"})
	}
	var v *domain.ErrValidation
	if errors.As(err, &v) {
		return utils.ValidationFailed(c, []string{err.Error()})
	}
	return utils.InternalError(c, err)
}

type setSalesOrderTemplateDTO struct {
	Template string `json:"template" validate:"required,oneof=template_1 template_2 template_3 template_4 template_5 template_6 template_7"`
}

func (ctrl *SalesOrderController) SetTemplate(c *fiber.Ctx) error {
	var dto setSalesOrderTemplateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.SetTemplate(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"), dto.Template)
	if err != nil {
		return salesOrderErr(c, err)
	}
	return utils.Ok(c, row, "Template updated")
}
