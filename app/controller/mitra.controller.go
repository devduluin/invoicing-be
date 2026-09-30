package controller

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v2"

	audit "duluin_invoice/app/domain/audit"
	contactdomain "duluin_invoice/app/domain/contactperson"
	domain "duluin_invoice/app/domain/mitra"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type MitraController struct {
	svc domain.IMitraService
	doc documentAudit
}

func NewMitraController(svc domain.IMitraService, auditSvc audit.ILogger) *MitraController {
	return &MitraController{svc: svc, doc: documentAudit{log: auditSvc, module: audit.ModulePartner, entityType: "partner"}}
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
	ctrl.doc.record(c, audit.ActionCreated, mitra.ID, mitra.Name, "Created partner "+mitra.Name, nil)
	return utils.Created(c, mitra, "Partner created")
}

// Import — POST /mitra/import: the partners of an import file, saved all or nothing. Every row is
// validated first and all problems are returned together, each prefixed with its spreadsheet row.
func (ctrl *MitraController) Import(c *fiber.Ctx) error {
	var body domain.ImportMitraDTO
	if err := c.BodyParser(&body); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if len(body.Partners) == 0 {
		return utils.ValidationFailed(c, []string{"The file has no partners to import"})
	}
	if len(body.Partners) > domain.MaxImportPartners {
		return utils.ValidationFailed(c, []string{fmt.Sprintf("At most %d partners can be imported at once", domain.MaxImportPartners)})
	}

	perms := contactPerms(c)
	var msgs []string
	dtos := make([]*domain.CreateMitraDTO, 0, len(body.Partners))
	for i := range body.Partners {
		p := &body.Partners[i]
		for _, m := range validation.Struct(&p.CreateMitraDTO) {
			msgs = append(msgs, fmt.Sprintf("Row %d: %s", p.Row, m))
		}
		p.ContactPerms = perms
		dtos = append(dtos, &p.CreateMitraDTO)
	}
	if msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}

	created, err := ctrl.svc.Import(middlewares.GetCompanyID(c), middlewares.GetUserID(c), dtos)
	if err != nil {
		return handleMitraError(c, err)
	}
	for _, m := range created {
		ctrl.doc.record(c, audit.ActionCreated, m.ID, m.Name, "Imported partner "+m.Name, nil)
	}
	return utils.Created(c, fiber.Map{"created": len(created)}, fmt.Sprintf("%d partners imported", len(created)))
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

// NextCode — GET /mitra/next-code: the code a new partner would get (a preview for the form).
func (ctrl *MitraController) NextCode(c *fiber.Ctx) error {
	code, err := ctrl.svc.NextCode(middlewares.GetCompanyID(c))
	if err != nil {
		return utils.InternalError(c, err)
	}
	return utils.Ok(c, fiber.Map{"code": code}, "OK")
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

	before, _ := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"))
	mitra, err := ctrl.svc.Update(
		middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"), &dto,
	)
	if err != nil {
		return handleMitraError(c, err)
	}
	ctrl.doc.record(c, audit.ActionUpdated, mitra.ID, mitra.Name, "Updated partner "+mitra.Name, docDiff(before, mitra))
	return utils.Ok(c, mitra, "Partner updated")
}

// deleteOne — the guarded delete (a partner still on documents is refused) + its audit entry,
// shared by Delete and BulkDelete.
func (ctrl *MitraController) deleteOne(c *fiber.Ctx, id string) error {
	before, _ := ctrl.svc.Get(middlewares.GetCompanyID(c), id)
	if err := ctrl.svc.Delete(middlewares.GetCompanyID(c), id); err != nil {
		return err
	}
	if before != nil {
		ctrl.doc.record(c, audit.ActionDeleted, before.ID, before.Name, "Deleted partner "+before.Name, nil)
	}
	return nil
}

func (ctrl *MitraController) Delete(c *fiber.Ctx) error {
	if err := ctrl.deleteOne(c, c.Params("id")); err != nil {
		return handleMitraError(c, err)
	}
	return utils.Deleted(c, "Partner deleted")
}

// setActiveOne switches one partner's active status (nothing else changes) and records it.
func (ctrl *MitraController) setActiveOne(c *fiber.Ctx, id string, active bool) error {
	companyID := middlewares.GetCompanyID(c)
	before, err := ctrl.svc.Get(companyID, id)
	if err != nil {
		return err
	}
	if bool(before.IsActive) == active {
		return nil // already so
	}
	after, err := ctrl.svc.Update(companyID, middlewares.GetUserID(c), id, &domain.UpdateMitraDTO{IsActive: &active, ContactPerms: contactPerms(c)})
	if err != nil {
		return err
	}
	verb := "Deactivated"
	if active {
		verb = "Activated"
	}
	ctrl.doc.record(c, audit.ActionUpdated, after.ID, after.Name, verb+" partner "+after.Name, docDiff(before, after))
	return nil
}

// POST /mitra/bulk-activate | bulk-deactivate | bulk-delete — {"ids": [...]}, up to 100 at once, each
// through the same single-partner action; one refused partner doesn't stop the rest.
func (ctrl *MitraController) BulkActivate(c *fiber.Ctx) error {
	return ctrl.bulkSetActive(c, true)
}

func (ctrl *MitraController) BulkDeactivate(c *fiber.Ctx) error {
	return ctrl.bulkSetActive(c, false)
}

func (ctrl *MitraController) bulkSetActive(c *fiber.Ctx, active bool) error {
	ids, ok := parseBulkIDs(c)
	if !ok {
		return nil
	}
	return runBulk(c, ids, func(id string) error { return ctrl.setActiveOne(c, id, active) }, "Bulk status change processed")
}

func (ctrl *MitraController) BulkDelete(c *fiber.Ctx) error {
	ids, ok := parseBulkIDs(c)
	if !ok {
		return nil
	}
	return runBulk(c, ids, func(id string) error { return ctrl.deleteOne(c, id) }, "Bulk delete processed")
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
	var codeExists *domain.ErrCodeExists
	var linked *domain.ErrCompanyLinked
	if errors.As(err, &codeExists) || errors.As(err, &linked) {
		return utils.Conflict(c, []string{err.Error()})
	}
	var invalid *domain.ErrValidation
	if errors.As(err, &invalid) {
		return utils.ValidationFailed(c, []string{err.Error()})
	}
	return utils.InternalError(c, err)
}
