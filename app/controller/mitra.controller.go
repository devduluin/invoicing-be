package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	contactdomain "duluin_invoice/app/domain/contactperson"
	domain "duluin_invoice/app/domain/mitra"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type MitraController struct {
	svc domain.IMitraService
}

func NewMitraController(svc domain.IMitraService) *MitraController {
	return &MitraController{svc: svc}
}

func (ctrl *MitraController) Create(c *fiber.Ctx) error {
	companyID := middlewares.GetCompanyID(c)

	var dto domain.CreateMitraDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}

	// Creating a partner WITH contact persons needs the contact permission too (not just the partner one).
	dto.ContactPerms = contactPerms(c)

	mitra, err := ctrl.svc.Create(companyID, middlewares.GetUserID(c), &dto)
	if err != nil {
		return handleMitraError(c, err)
	}
	return utils.Created(c, mitra, "Partner created")
}

func (ctrl *MitraController) List(c *fiber.Ctx) error {
	filter := &domain.MitraFilter{
		CompanyID: middlewares.GetCompanyID(c),
		Search:    c.Query("search"),
		Type:      c.Query("type"),
		Page:      c.QueryInt("page", 1),
		PageSize:  c.QueryInt("limit", 20),
		IsActive:  utils.ParseBoolQuery(c.Query("is_active")),
		Sort:      c.Query("sort"),
		Order:     c.Query("order"),
		Fields:    utils.ParseCSVParam(c.Query("fields")),
	}

	res, err := ctrl.svc.List(filter)
	if err != nil {
		return utils.InternalError(c, err)
	}
	return utils.List(c, res, "OK")
}

func (ctrl *MitraController) Get(c *fiber.Ctx) error {
	mitra, err := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"))
	if err != nil {
		return handleMitraError(c, err)
	}
	return utils.Ok(c, mitra, "OK")
}

func (ctrl *MitraController) Update(c *fiber.Ctx) error {
	var dto domain.UpdateMitraDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}

	dto.ContactPerms = contactPerms(c)

	mitra, err := ctrl.svc.Update(
		middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"), &dto,
	)
	if err != nil {
		return handleMitraError(c, err)
	}
	return utils.Ok(c, mitra, "Partner updated")
}

func (ctrl *MitraController) Delete(c *fiber.Ctx) error {
	if err := ctrl.svc.Delete(middlewares.GetCompanyID(c), c.Params("id")); err != nil {
		return handleMitraError(c, err)
	}
	return utils.Deleted(c, "Partner deleted")
}

// contactPerms — what the caller may do to contact persons; a partner save that needs more is refused.
func contactPerms(c *fiber.Ctx) contactdomain.Perms {
	return contactdomain.Perms{
		Create: middlewares.HasAnyPermission(c, "invoice-mitra-contact-create"),
		Update: middlewares.HasAnyPermission(c, "invoice-mitra-contact-update"),
		Delete: middlewares.HasAnyPermission(c, "invoice-mitra-contact-delete"),
	}
}

func handleMitraError(c *fiber.Ctx, err error) error {
	if handled, resp := handleActivationError(c, err); handled {
		return resp
	}
	var forbidden *contactdomain.ErrForbidden
	if errors.As(err, &forbidden) {
		return utils.Forbidden(c, []string{err.Error()})
	}
	var inUse *utils.ErrInUse
	if errors.As(err, &inUse) {
		return utils.InUse(c, inUse.Message)
	}
	var notFound *domain.ErrNotFound
	if errors.As(err, &notFound) {
		return utils.NotFound(c, []string{err.Error()})
	}
	var invalid *domain.ErrValidation
	if errors.As(err, &invalid) {
		return utils.ValidationFailed(c, []string{err.Error()})
	}
	return utils.InternalError(c, err)
}
