package controller

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v2"

	audit "duluin_invoice/app/domain/audit"
	domain "duluin_invoice/app/domain/salesinvoice"
	"duluin_invoice/app/model"
	"duluin_invoice/app/sso"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type SalesInvoiceController struct {
	svc   domain.IService
	audit audit.ILogger
	sso   *sso.Client
}

func NewSalesInvoiceController(svc domain.IService, auditSvc audit.ILogger, ssoClient *sso.Client) *SalesInvoiceController {
	return &SalesInvoiceController{svc: svc, audit: auditSvc, sso: ssoClient}
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
		SalespersonID: c.Query("salesperson_id"),
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
	if err := uploadDocumentAttachment(c, ctrl.sso, "sales-invoice", "", &dto.AttachmentData); err != nil {
		return attachmentUploadFailed(c, err)
	}
	if err := uploadDocumentAttachment(c, ctrl.sso, "signature", "", &dto.SignatureData); err != nil {
		return attachmentUploadFailed(c, err)
	}
	row, err := ctrl.svc.Create(middlewares.GetCompanyID(c), middlewares.GetUserID(c), &dto)
	if err != nil {
		return salesInvoiceErr(c, err)
	}
	ctrl.logInvoice(c, audit.ActionCreated, row, fmt.Sprintf("Created %s", row.Number), nil)
	return utils.Created(c, row, "Invoice added")
}

// Import — POST /sales-invoices/import: the invoices of an import file (partner and taxes already
// resolved to ids by the client), saved all or nothing as draft regular invoices. Every row is
// validated first and all problems are returned together, each prefixed with its spreadsheet row.
func (ctrl *SalesInvoiceController) Import(c *fiber.Ctx) error {
	var body domain.ImportDTO
	if err := c.BodyParser(&body); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if len(body.Invoices) == 0 {
		return utils.ValidationFailed(c, []string{"The file has no invoices to import"})
	}
	if len(body.Invoices) > domain.MaxImportInvoices {
		return utils.ValidationFailed(c, []string{fmt.Sprintf("At most %d invoices can be imported at once", domain.MaxImportInvoices)})
	}
	var msgs []string
	for i := range body.Invoices {
		r := &body.Invoices[i]
		r.Kind = "invoice"
		for _, m := range validation.Struct(&r.CreateDTO) {
			msgs = append(msgs, fmt.Sprintf("Row %d: %s", r.Row, m))
		}
	}
	if msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}

	created, err := ctrl.svc.Import(middlewares.GetCompanyID(c), middlewares.GetUserID(c), body.Invoices)
	if err != nil {
		return salesInvoiceErr(c, err)
	}
	for _, inv := range created {
		ctrl.logInvoice(c, audit.ActionCreated, &model.SalesInvoice{ID: inv.ID, Number: inv.Number, Kind: model.SalesInvoiceKindInvoice}, fmt.Sprintf("Imported %s", inv.Number), nil)
	}
	return utils.Created(c, fiber.Map{"created": len(created), "invoices": created}, fmt.Sprintf("%d invoices imported", len(created)))
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
	existingAttachment, existingSignature := "", ""
	if before != nil {
		existingAttachment, existingSignature = before.AttachmentData, before.SignatureData
	}
	if err := uploadDocumentAttachment(c, ctrl.sso, "sales-invoice", existingAttachment, &dto.AttachmentData); err != nil {
		return attachmentUploadFailed(c, err)
	}
	if err := uploadDocumentAttachment(c, ctrl.sso, "signature", existingSignature, &dto.SignatureData); err != nil {
		return attachmentUploadFailed(c, err)
	}
	row, err := ctrl.svc.Update(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"), &dto)
	if err != nil {
		return salesInvoiceErr(c, err)
	}
	ctrl.logInvoice(c, audit.ActionUpdated, row, fmt.Sprintf("Updated %s", row.Number), salesInvoiceDiff(before, row))
	return utils.Ok(c, row, "Invoice updated")
}

// deleteOne — the guarded delete + its audit entry, shared by Delete and BulkDelete.
func (ctrl *SalesInvoiceController) deleteOne(c *fiber.Ctx, id string) error {
	companyID := middlewares.GetCompanyID(c)
	before, _ := ctrl.svc.Get(companyID, id)
	if err := ctrl.svc.Delete(companyID, id); err != nil {
		return err
	}
	if before != nil {
		ctrl.logInvoice(c, audit.ActionDeleted, before, fmt.Sprintf("Deleted %s", before.Number), nil)
	}
	return nil
}

func (ctrl *SalesInvoiceController) Delete(c *fiber.Ctx) error {
	if err := ctrl.deleteOne(c, c.Params("id")); err != nil {
		return salesInvoiceErr(c, err)
	}
	return utils.Deleted(c, "Invoice deleted")
}

// POST bulk-delete — {"ids": [...]}, up to 100 at once. Each id goes through the exact same guarded
// delete as the single-row endpoint; a row that's still referenced fails on its own without blocking
// the rest of the batch.
func (ctrl *SalesInvoiceController) BulkDelete(c *fiber.Ctx) error {
	ids, ok := parseBulkIDs(c)
	if !ok {
		return nil
	}
	return runBulk(c, ids, func(id string) error { return ctrl.deleteOne(c, id) }, "Bulk delete processed")
}

// confirmOne / draftOne — the status change + its audit entry, shared by the single-row and bulk endpoints.
func (ctrl *SalesInvoiceController) confirmOne(c *fiber.Ctx, id string) (*model.SalesInvoice, error) {
	row, err := ctrl.svc.Confirm(middlewares.GetCompanyID(c), middlewares.GetUserID(c), id)
	if err != nil {
		return nil, err
	}
	// "Save & Confirm" is one action to the user: the trail shows the document being created, not a
	// second, separate confirmation a moment later.
	if !confirmsJustCreated(c, row.CreatedAt, row.CreatedBy) {
		ctrl.logInvoice(c, audit.ActionStatusChanged, row, fmt.Sprintf("Confirmed %s", row.Number), map[string]audit.Change{
			"status": {Before: string(model.SalesInvoiceStatusDraft), After: string(row.Status)},
		})
	}
	return row, nil
}

func (ctrl *SalesInvoiceController) draftOne(c *fiber.Ctx, id string) (*model.SalesInvoice, error) {
	row, err := ctrl.svc.BackToDraft(middlewares.GetCompanyID(c), middlewares.GetUserID(c), id)
	if err != nil {
		return nil, err
	}
	ctrl.logInvoice(c, audit.ActionStatusChanged, row, fmt.Sprintf("Moved %s back to draft", row.Number), map[string]audit.Change{
		"status": {Before: string(model.SalesInvoiceStatusConfirmed), After: string(row.Status)},
	})
	return row, nil
}

func (ctrl *SalesInvoiceController) Confirm(c *fiber.Ctx) error {
	row, err := ctrl.confirmOne(c, c.Params("id"))
	if err != nil {
		return salesInvoiceErr(c, err)
	}
	return utils.Ok(c, row, "Invoice confirmed")
}

// POST bulk-confirm — {"ids": [...]}: confirms each draft on its own (a non-draft or invalid one fails
// alone, the rest go through).
func (ctrl *SalesInvoiceController) BulkConfirm(c *fiber.Ctx) error {
	ids, ok := parseBulkIDs(c)
	if !ok {
		return nil
	}
	return runBulk(c, ids, func(id string) error {
		_, err := ctrl.confirmOne(c, id)
		return err
	}, "Bulk confirm processed")
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
	row, err := ctrl.draftOne(c, c.Params("id"))
	if err != nil {
		return salesInvoiceErr(c, err)
	}
	return utils.Ok(c, row, "Invoice moved back to draft")
}

// POST bulk-draft — {"ids": [...]}: moves each document back to draft on its own.
func (ctrl *SalesInvoiceController) BulkBackToDraft(c *fiber.Ctx) error {
	ids, ok := parseBulkIDs(c)
	if !ok {
		return nil
	}
	return runBulk(c, ids, func(id string) error {
		_, err := ctrl.draftOne(c, id)
		return err
	}, "Bulk back to draft processed")
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
