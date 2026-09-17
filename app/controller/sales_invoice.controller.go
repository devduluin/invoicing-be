package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	domain "duluin_invoice/app/domain/salesinvoice"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type SalesInvoiceController struct{ svc domain.IService }

func NewSalesInvoiceController(svc domain.IService) *SalesInvoiceController {
	return &SalesInvoiceController{svc: svc}
}

func (ctrl *SalesInvoiceController) List(c *fiber.Ctx) error {
	res, err := ctrl.svc.List(&domain.Filter{
		CompanyID: middlewares.GetCompanyID(c),
		Kind:      c.Query("kind"),
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

func (ctrl *SalesInvoiceController) PreviewNumber(c *fiber.Ctx) error {
	kind := c.Query("kind", "invoice")
	number, err := ctrl.svc.PreviewNumber(middlewares.GetCompanyID(c), kind)
	if err != nil {
		return salesInvoiceErr(c, err)
	}
	return utils.Ok(c, fiber.Map{"number": number}, "OK")
}

func (ctrl *SalesInvoiceController) Get(c *fiber.Ctx) error {
	row, err := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"))
	if err != nil {
		return salesInvoiceErr(c, err)
	}
	return utils.Ok(c, row, "OK")
}

func (ctrl *SalesInvoiceController) Create(c *fiber.Ctx) error {
	var dto domain.CreateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.Create(middlewares.GetCompanyID(c), middlewares.GetUserID(c), &dto)
	if err != nil {
		return salesInvoiceErr(c, err)
	}
	return utils.Created(c, row, "Invoice added")
}

func (ctrl *SalesInvoiceController) Update(c *fiber.Ctx) error {
	var dto domain.UpdateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.Update(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"), &dto)
	if err != nil {
		return salesInvoiceErr(c, err)
	}
	return utils.Ok(c, row, "Invoice updated")
}

func (ctrl *SalesInvoiceController) Delete(c *fiber.Ctx) error {
	if err := ctrl.svc.Delete(middlewares.GetCompanyID(c), c.Params("id")); err != nil {
		return salesInvoiceErr(c, err)
	}
	return utils.Deleted(c, "Invoice deleted")
}

func (ctrl *SalesInvoiceController) Confirm(c *fiber.Ctx) error {
	row, err := ctrl.svc.Confirm(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"))
	if err != nil {
		return salesInvoiceErr(c, err)
	}
	return utils.Ok(c, row, "Invoice confirmed")
}

func (ctrl *SalesInvoiceController) BackToDraft(c *fiber.Ctx) error {
	row, err := ctrl.svc.BackToDraft(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"))
	if err != nil {
		return salesInvoiceErr(c, err)
	}
	return utils.Ok(c, row, "Invoice moved back to draft")
}

func (ctrl *SalesInvoiceController) Cancel(c *fiber.Ctx) error {
	row, err := ctrl.svc.Cancel(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"))
	if err != nil {
		return salesInvoiceErr(c, err)
	}
	return utils.Ok(c, row, "Invoice cancelled")
}

func salesInvoiceErr(c *fiber.Ctx, err error) error {
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
