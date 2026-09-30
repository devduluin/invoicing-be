package controller

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v2"

	audit "duluin_invoice/app/domain/audit"
	domain "duluin_invoice/app/domain/salesperson"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type SalespersonController struct {
	svc domain.IService
	// the bulk and from-members endpoints act on many salespersons in one request, so they record
	// one entry per salesperson themselves (the /salespersons audit middleware skips them)
	doc documentAudit
}

func NewSalespersonController(svc domain.IService, auditSvc audit.ILogger) *SalespersonController {
	return &SalespersonController{svc: svc, doc: documentAudit{log: auditSvc, module: audit.ModuleMasterData, entityType: "salesperson"}}
}

func (ctrl *SalespersonController) List(c *fiber.Ctx) error {
	res, err := ctrl.svc.List(&domain.Filter{
		CompanyID: middlewares.GetCompanyID(c),
		Search:    c.Query("search"),
		IsActive:  utils.ParseBoolQuery(c.Query("is_active")),
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

func (ctrl *SalespersonController) Get(c *fiber.Ctx) error {
	row, err := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"))
	if err != nil {
		return salespersonErr(c, err)
	}
	return utils.Ok(c, row, "OK")
}

func (ctrl *SalespersonController) Create(c *fiber.Ctx) error {
	var dto domain.CreateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.Create(middlewares.GetCompanyID(c), middlewares.GetUserID(c), &dto)
	if err != nil {
		return salespersonErr(c, err)
	}
	return utils.Created(c, row, "Salesperson added")
}

func (ctrl *SalespersonController) Update(c *fiber.Ctx) error {
	var dto domain.UpdateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.Update(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"), &dto)
	if err != nil {
		return salespersonErr(c, err)
	}
	return utils.Ok(c, row, "Salesperson updated")
}

func (ctrl *SalespersonController) Delete(c *fiber.Ctx) error {
	if err := ctrl.svc.Delete(middlewares.GetCompanyID(c), c.Params("id")); err != nil {
		return salespersonErr(c, err)
	}
	return utils.Deleted(c, "Salesperson deleted")
}

// NextCode — GET /salespersons/next-code: the code a new salesperson would get (a form preview).
func (ctrl *SalespersonController) NextCode(c *fiber.Ctx) error {
	code, err := ctrl.svc.NextCode(middlewares.GetCompanyID(c))
	if err != nil {
		return utils.InternalError(c, err)
	}
	return utils.Ok(c, fiber.Map{"code": code}, "OK")
}

// Mine — GET /salespersons/mine: the salesperson linked to the calling user, or null. A new sales
// order / invoice starts with it.
func (ctrl *SalespersonController) Mine(c *fiber.Ctx) error {
	row, err := ctrl.svc.Mine(middlewares.GetCompanyID(c), middlewares.GetUserID(c))
	if err != nil {
		return utils.InternalError(c, err)
	}
	return utils.Ok(c, row, "OK")
}

// TeamMembers — GET /salespersons/team-members: the company's members, for linking / "add from team".
func (ctrl *SalespersonController) TeamMembers(c *fiber.Ctx) error {
	rows, err := ctrl.svc.TeamMembers(middlewares.GetCompanyID(c))
	if err != nil {
		return utils.InternalError(c, err)
	}
	return utils.Ok(c, rows, "OK")
}

// FromMembers — POST /salespersons/from-members: one salesperson per picked team member.
func (ctrl *SalespersonController) FromMembers(c *fiber.Ctx) error {
	var dto domain.FromMembersDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	rows, err := ctrl.svc.CreateFromMembers(middlewares.GetCompanyID(c), middlewares.GetUserID(c), dto.UserIDs)
	if err != nil {
		return salespersonErr(c, err)
	}
	for _, s := range rows {
		ctrl.doc.record(c, audit.ActionCreated, s.ID, s.Code+" "+s.Name, "Created salesperson "+s.Code+" "+s.Name, nil)
	}
	return utils.Created(c, rows, fmt.Sprintf("%d salespersons added", len(rows)))
}

// setActiveOne switches one salesperson's active status and records it.
func (ctrl *SalespersonController) setActiveOne(c *fiber.Ctx, id string, active bool) error {
	companyID := middlewares.GetCompanyID(c)
	before, err := ctrl.svc.Get(companyID, id)
	if err != nil {
		return err
	}
	if bool(before.IsActive) == active {
		return nil // already so
	}
	after, err := ctrl.svc.Update(companyID, middlewares.GetUserID(c), id, &domain.UpdateDTO{IsActive: &active})
	if err != nil {
		return err
	}
	verb := "Deactivated"
	if active {
		verb = "Activated"
	}
	ctrl.doc.record(c, audit.ActionUpdated, after.ID, after.Code+" "+after.Name, verb+" salesperson "+after.Code+" "+after.Name, docDiff(before, after))
	return nil
}

// deleteOne — the guarded delete (a salesperson on documents is refused) + its audit entry.
func (ctrl *SalespersonController) deleteOne(c *fiber.Ctx, id string) error {
	before, _ := ctrl.svc.Get(middlewares.GetCompanyID(c), id)
	if err := ctrl.svc.Delete(middlewares.GetCompanyID(c), id); err != nil {
		return err
	}
	if before != nil {
		ctrl.doc.record(c, audit.ActionDeleted, before.ID, before.Code+" "+before.Name, "Deleted salesperson "+before.Code+" "+before.Name, nil)
	}
	return nil
}

// POST /salespersons/bulk-activate | bulk-deactivate | bulk-delete — {"ids": [...]}, up to 100 at once.
func (ctrl *SalespersonController) BulkActivate(c *fiber.Ctx) error {
	return ctrl.bulkSetActive(c, true)
}

func (ctrl *SalespersonController) BulkDeactivate(c *fiber.Ctx) error {
	return ctrl.bulkSetActive(c, false)
}

func (ctrl *SalespersonController) bulkSetActive(c *fiber.Ctx, active bool) error {
	ids, ok := parseBulkIDs(c)
	if !ok {
		return nil
	}
	return runBulk(c, ids, func(id string) error { return ctrl.setActiveOne(c, id, active) }, "Bulk status change processed")
}

func (ctrl *SalespersonController) BulkDelete(c *fiber.Ctx) error {
	ids, ok := parseBulkIDs(c)
	if !ok {
		return nil
	}
	return runBulk(c, ids, func(id string) error { return ctrl.deleteOne(c, id) }, "Bulk delete processed")
}

func salespersonErr(c *fiber.Ctx, err error) error {
	var inUse *utils.ErrInUse
	if errors.As(err, &inUse) {
		return utils.InUse(c, inUse.Message)
	}
	var nf *domain.ErrNotFound
	if errors.As(err, &nf) {
		return utils.NotFound(c, []string{err.Error()})
	}
	var conflict *domain.ErrConflict
	if errors.As(err, &conflict) {
		return utils.Conflict(c, []string{err.Error()})
	}
	var v *domain.ErrValidation
	if errors.As(err, &v) {
		return utils.ValidationFailed(c, []string{err.Error()})
	}
	return utils.InternalError(c, err)
}
