package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	domain "duluin_invoice/app/domain/salespayment"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type SalesPaymentController struct{ svc domain.IService }

func NewSalesPaymentController(svc domain.IService) *SalesPaymentController {
	return &SalesPaymentController{svc: svc}
}

func (ctrl *SalesPaymentController) List(c *fiber.Ctx) error {
	res, err := ctrl.svc.List(&domain.Filter{
		CompanyID:      middlewares.GetCompanyID(c),
		SalesInvoiceID: c.Query("sales_invoice_id"),
		MitraID:        c.Query("mitra_id"),
		Status:         c.Query("status"),
		Search:         c.Query("search"),
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

func (ctrl *SalesPaymentController) Get(c *fiber.Ctx) error {
	row, err := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"))
	if err != nil {
		return salesPaymentErr(c, err)
	}
	return utils.Ok(c, row, "OK")
}

func (ctrl *SalesPaymentController) Create(c *fiber.Ctx) error {
	var dto domain.CreateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.Create(middlewares.GetCompanyID(c), middlewares.GetUserID(c), &dto)
	if err != nil {
		return salesPaymentErr(c, err)
	}
	return utils.Created(c, row, "Payment recorded")
}

func (ctrl *SalesPaymentController) Verify(c *fiber.Ctx) error {
	row, err := ctrl.svc.Verify(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"))
	if err != nil {
		return salesPaymentErr(c, err)
	}
	return utils.Ok(c, row, "Payment verified")
}

func salesPaymentErr(c *fiber.Ctx, err error) error {
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
	var tr *domain.ErrInvalidTransition
	if errors.As(err, &tr) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": err.Error(), "error_code": "invalid_transition"})
	}
	var v *domain.ErrValidation
	if errors.As(err, &v) {
		return utils.ValidationFailed(c, []string{err.Error()})
	}
	return utils.InternalError(c, err)
}
