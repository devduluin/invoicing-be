package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	domain "duluin_invoice/app/domain/documenttemplate"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type DocumentTemplateController struct{ svc domain.IService }

func NewDocumentTemplateController(svc domain.IService) *DocumentTemplateController {
	return &DocumentTemplateController{svc: svc}
}

func (ctrl *DocumentTemplateController) List(c *fiber.Ctx) error {
	items, err := ctrl.svc.List(middlewares.GetCompanyID(c))
	if err != nil {
		return utils.InternalError(c, err)
	}
	return utils.Ok(c, items, "OK")
}

func (ctrl *DocumentTemplateController) Set(c *fiber.Ctx) error {
	var dto domain.SetDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	item, err := ctrl.svc.Set(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("doc_type"), &dto)
	if err != nil {
		var v *domain.ErrValidation
		if errors.As(err, &v) {
			return utils.ValidationFailed(c, []string{err.Error()})
		}
		return utils.InternalError(c, err)
	}
	return utils.Ok(c, item, "Default template saved")
}
