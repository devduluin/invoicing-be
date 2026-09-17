package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	domain "duluin_invoice/app/domain/salesreceipt"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type SalesReceiptController struct{ svc domain.IService }

func NewSalesReceiptController(svc domain.IService) *SalesReceiptController {
	return &SalesReceiptController{svc: svc}
}

func (ctrl *SalesReceiptController) List(c *fiber.Ctx) error {
	res, err := ctrl.svc.List(&domain.Filter{
		CompanyID:      middlewares.GetCompanyID(c),
		Search:         c.Query("search"),
		MitraID:        c.Query("mitra_id"),
		SalesInvoiceID: c.Query("sales_invoice_id"),
		Page:           c.QueryInt("page", 1),
		PageSize:       c.QueryInt("limit", 20),
		Sort:           c.Query("sort"),
		Order:          c.Query("order"),
		Fields:         utils.ParseCSVParam(c.Query("fields")),
	})
	if err != nil {
		return utils.InternalError(c, err)
	}
	return utils.List(c, res, "OK")
}

func (ctrl *SalesReceiptController) PreviewNumber(c *fiber.Ctx) error {
	number, err := ctrl.svc.PreviewNumber(middlewares.GetCompanyID(c))
	if err != nil {
		return salesReceiptErr(c, err)
	}
	return utils.Ok(c, fiber.Map{"number": number}, "OK")
}

func (ctrl *SalesReceiptController) Get(c *fiber.Ctx) error {
	row, err := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"))
	if err != nil {
		return salesReceiptErr(c, err)
	}
	return utils.Ok(c, row, "OK")
}

func (ctrl *SalesReceiptController) Create(c *fiber.Ctx) error {
	var dto domain.CreateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.Create(middlewares.GetCompanyID(c), middlewares.GetUserID(c), &dto)
	if err != nil {
		return salesReceiptErr(c, err)
	}
	return utils.Created(c, row, "Receipt added")
}

func salesReceiptErr(c *fiber.Ctx, err error) error {
	var nf *domain.ErrNotFound
	if errors.As(err, &nf) {
		return utils.NotFound(c, []string{err.Error()})
	}
	var dup *domain.ErrNumberExists
	if errors.As(err, &dup) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": err.Error(), "error_code": "number_exists"})
	}
	var bal *domain.ErrExceedsBalance
	if errors.As(err, &bal) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": err.Error(), "error_code": "exceeds_balance"})
	}
	var v *domain.ErrValidation
	if errors.As(err, &v) {
		return utils.ValidationFailed(c, []string{err.Error()})
	}
	return utils.InternalError(c, err)
}
