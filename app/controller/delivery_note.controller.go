package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	domain "duluin_invoice/app/domain/deliverynote"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type DeliveryNoteController struct{ svc domain.IService }

func NewDeliveryNoteController(svc domain.IService) *DeliveryNoteController {
	return &DeliveryNoteController{svc: svc}
}

func (ctrl *DeliveryNoteController) List(c *fiber.Ctx) error {
	res, err := ctrl.svc.List(&domain.Filter{
		CompanyID:    middlewares.GetCompanyID(c),
		Search:       c.Query("search"),
		MitraID:      c.Query("mitra_id"),
		SalesOrderID: c.Query("sales_order_id"),
		Page:         c.QueryInt("page", 1),
		PageSize:     c.QueryInt("limit", 20),
		Sort:         c.Query("sort"),
		Order:        c.Query("order"),
		Fields:       utils.ParseCSVParam(c.Query("fields")),
	})
	if err != nil {
		return utils.InternalError(c, err)
	}
	return utils.List(c, res, "OK")
}

func (ctrl *DeliveryNoteController) Get(c *fiber.Ctx) error {
	row, err := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"))
	if err != nil {
		return deliveryNoteErr(c, err)
	}
	return utils.Ok(c, row, "OK")
}

func (ctrl *DeliveryNoteController) Create(c *fiber.Ctx) error {
	var dto domain.CreateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.Create(middlewares.GetCompanyID(c), middlewares.GetUserID(c), &dto)
	if err != nil {
		return deliveryNoteErr(c, err)
	}
	return utils.Created(c, row, "Delivery note added")
}

func (ctrl *DeliveryNoteController) Update(c *fiber.Ctx) error {
	var dto domain.UpdateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.Update(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"), &dto)
	if err != nil {
		return deliveryNoteErr(c, err)
	}
	return utils.Ok(c, row, "Delivery note updated")
}

func (ctrl *DeliveryNoteController) Delete(c *fiber.Ctx) error {
	if err := ctrl.svc.Delete(middlewares.GetCompanyID(c), c.Params("id")); err != nil {
		return deliveryNoteErr(c, err)
	}
	return utils.Deleted(c, "Delivery note deleted")
}

func deliveryNoteErr(c *fiber.Ctx, err error) error {
	var nf *domain.ErrNotFound
	if errors.As(err, &nf) {
		return utils.NotFound(c, []string{err.Error()})
	}
	var dup *domain.ErrNumberExists
	if errors.As(err, &dup) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": err.Error(), "error_code": "number_exists"})
	}
	var v *domain.ErrValidation
	if errors.As(err, &v) {
		return utils.ValidationFailed(c, []string{err.Error()})
	}
	return utils.InternalError(c, err)
}
