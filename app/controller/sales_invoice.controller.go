package controller

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v2"

	audit "duluin_invoice/app/domain/audit"
	domain "duluin_invoice/app/domain/salesinvoice"
	"duluin_invoice/app/model"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type SalesInvoiceController struct {
	svc   domain.IService
	audit audit.ILogger
}

func NewSalesInvoiceController(svc domain.IService, auditSvc audit.ILogger) *SalesInvoiceController {
	return &SalesInvoiceController{svc: svc, audit: auditSvc}
}

// auditModule tells a regular Sales Invoice apart from a Down Payment invoice — same table, two
// modules in the audit trail, matching how the rest of the app already treats the two `Kind`s.
func auditModule(kind model.SalesInvoiceKind) string {
	if kind == model.SalesInvoiceKindDownPayment {
		return audit.ModuleDownPayment
	}
	return audit.ModuleSalesInvoice
}

func (ctrl *SalesInvoiceController) logInvoice(c *fiber.Ctx, action string, row *model.SalesInvoice, description string, changes map[string]audit.Change) {
	if row == nil {
		return
	}
	ctrl.audit.Log(auditActor(c), audit.Entry{
		Action:      action,
		Module:      auditModule(row.Kind),
		EntityType:  "sales_invoice",
		EntityID:    row.ID,
		EntityName:  row.Number,
		Description: description,
		Changes:     changes,
	})
}

func (ctrl *SalesInvoiceController) List(c *fiber.Ctx) error {
	res, err := ctrl.svc.List(&domain.Filter{
		CompanyID:     middlewares.GetCompanyID(c),
		Kind:          c.Query("kind"),
		Search:        c.Query("search"),
		MitraID:       c.Query("mitra_id"),
		Status:        c.Query("status"),
		PaymentStatus: c.Query("payment_status"),
		Overdue:       c.Query("overdue") == "true",
		Page:          c.QueryInt("page", 1),
		PageSize:      c.QueryInt("limit", 20),
		Sort:          c.Query("sort"),
		Order:         c.Query("order"),
		Fields:        utils.ParseCSVParam(c.Query("fields")),
	})
	if err != nil {
		return utils.InternalError(c, err)
	}
	return utils.List(c, res, "OK")
}

func (ctrl *SalesInvoiceController) Summary(c *fiber.Ctx) error {
	res, err := ctrl.svc.Summary(middlewares.GetCompanyID(c))
	if err != nil {
		return utils.InternalError(c, err)
	}
	return utils.Ok(c, res, "OK")
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
	ctrl.logInvoice(c, audit.ActionCreated, row, fmt.Sprintf("Created %s", row.Number), nil)
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
	before, _ := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"))
	row, err := ctrl.svc.Update(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"), &dto)
	if err != nil {
		return salesInvoiceErr(c, err)
	}
	ctrl.logInvoice(c, audit.ActionUpdated, row, fmt.Sprintf("Updated %s", row.Number), salesInvoiceDiff(before, row))
	return utils.Ok(c, row, "Invoice updated")
}

func (ctrl *SalesInvoiceController) Delete(c *fiber.Ctx) error {
	before, _ := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"))
	if err := ctrl.svc.Delete(middlewares.GetCompanyID(c), c.Params("id")); err != nil {
		return salesInvoiceErr(c, err)
	}
	if before != nil {
		ctrl.logInvoice(c, audit.ActionDeleted, before, fmt.Sprintf("Deleted %s", before.Number), nil)
	}
	return utils.Deleted(c, "Invoice deleted")
}

func (ctrl *SalesInvoiceController) Confirm(c *fiber.Ctx) error {
	row, err := ctrl.svc.Confirm(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"))
	if err != nil {
		return salesInvoiceErr(c, err)
	}
	ctrl.logInvoice(c, audit.ActionStatusChanged, row, fmt.Sprintf("Confirmed %s", row.Number), map[string]audit.Change{
		"status": {Before: string(model.SalesInvoiceStatusDraft), After: string(row.Status)},
	})
	return utils.Ok(c, row, "Invoice confirmed")
}

type setTemplateDTO struct {
	Template string `json:"template" validate:"required,oneof=template_1 template_2 template_3 template_4 template_5 template_6 template_7"`
}

func (ctrl *SalesInvoiceController) SetTemplate(c *fiber.Ctx) error {
	var dto setTemplateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.SetTemplate(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"), dto.Template)
	if err != nil {
		return salesInvoiceErr(c, err)
	}
	return utils.Ok(c, row, "Template updated")
}

func (ctrl *SalesInvoiceController) BackToDraft(c *fiber.Ctx) error {
	row, err := ctrl.svc.BackToDraft(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"))
	if err != nil {
		return salesInvoiceErr(c, err)
	}
	return utils.Ok(c, row, "Invoice moved back to draft")
}

func (ctrl *SalesInvoiceController) Cancel(c *fiber.Ctx) error {
	before, _ := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"))
	row, err := ctrl.svc.Cancel(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"))
	if err != nil {
		return salesInvoiceErr(c, err)
	}
	changes := map[string]audit.Change{"status": {After: string(row.Status)}}
	if before != nil {
		changes["status"] = audit.Change{Before: string(before.Status), After: string(row.Status)}
	}
	ctrl.logInvoice(c, audit.ActionCancelled, row, fmt.Sprintf("Cancelled %s", row.Number), changes)
	return utils.Ok(c, row, "Invoice cancelled")
}

func salesInvoiceErr(c *fiber.Ctx, err error) error {
	if handled, resp := handleActivationError(c, err); handled {
		return resp
	}
	var inUse *utils.ErrInUse
	if errors.As(err, &inUse) {
		return utils.InUse(c, inUse.Message)
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

// salesInvoiceDiff compares the fields a user can actually edit from the invoice form — not the
// derived/computed ones (outstanding amount, payment status, which only SalesPaymentRepository.Verify
// ever changes).
func salesInvoiceDiff(before, after *model.SalesInvoice) map[string]audit.Change {
	if before == nil || after == nil {
		return nil
	}
	changes := map[string]audit.Change{}
	if before.DueDate != nil || after.DueDate != nil {
		b, a := "", ""
		if before.DueDate != nil {
			b = before.DueDate.Format("2006-01-02")
		}
		if after.DueDate != nil {
			a = after.DueDate.Format("2006-01-02")
		}
		if b != a {
			changes["due_date"] = audit.Change{Before: b, After: a}
		}
	}
	if before.RefNo != after.RefNo {
		changes["ref_no"] = audit.Change{Before: before.RefNo, After: after.RefNo}
	}
	if before.Notes != after.Notes {
		changes["notes"] = audit.Change{Before: before.Notes, After: after.Notes}
	}
	if before.Terms != after.Terms {
		changes["terms"] = audit.Change{Before: before.Terms, After: after.Terms}
	}
	if before.GrandTotal != after.GrandTotal {
		changes["grand_total"] = audit.Change{Before: before.GrandTotal, After: after.GrandTotal}
	}
	return changes
}
