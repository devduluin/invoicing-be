package controller

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v2"

	audit "duluin_invoice/app/domain/audit"
	domain "duluin_invoice/app/domain/purchaseinvoice"
	"duluin_invoice/app/model"
	"duluin_invoice/app/sso"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type PurchaseInvoiceController struct {
	doc documentAudit
	svc domain.IService
	sso *sso.Client
}

func NewPurchaseInvoiceController(svc domain.IService, ssoClient *sso.Client, auditSvc audit.ILogger) *PurchaseInvoiceController {
	return &PurchaseInvoiceController{svc: svc, sso: ssoClient, doc: documentAudit{log: auditSvc, module: audit.ModulePurchaseInvoice, entityType: "purchase_invoice"}}
}

func (ctrl *PurchaseInvoiceController) List(c *fiber.Ctx) error {
	res, err := ctrl.svc.List(&domain.Filter{
		CompanyID:     middlewares.GetCompanyID(c),
		WithDetails:   c.Query("with") == "details",
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

func (ctrl *PurchaseInvoiceController) PreviewNumber(c *fiber.Ctx) error {
	number, err := ctrl.svc.PreviewNumber(middlewares.GetCompanyID(c))
	if err != nil {
		return purchaseInvoiceErr(c, err)
	}
	return utils.Ok(c, fiber.Map{"number": number}, "OK")
}

func (ctrl *PurchaseInvoiceController) Get(c *fiber.Ctx) error {
	row, err := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"))
	if err != nil {
		return purchaseInvoiceErr(c, err)
	}
	return utils.Ok(c, row, "OK")
}

func (ctrl *PurchaseInvoiceController) Create(c *fiber.Ctx) error {
	var dto domain.CreateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	if err := uploadDocumentAttachment(c, ctrl.sso, "purchase-invoice", "", &dto.AttachmentData); err != nil {
		return attachmentUploadFailed(c, err)
	}
	if err := uploadDocumentAttachment(c, ctrl.sso, "signature", "", &dto.SignatureData); err != nil {
		return attachmentUploadFailed(c, err)
	}
	row, err := ctrl.svc.Create(middlewares.GetCompanyID(c), middlewares.GetUserID(c), &dto)
	if err != nil {
		return purchaseInvoiceErr(c, err)
	}
	ctrl.doc.record(c, audit.ActionCreated, row.ID, row.Number, "Created "+row.Number, nil)
	return utils.Created(c, row, "Invoice added")
}

// Import — POST /purchase-invoices/import: the invoices of an import file (supplier and taxes already
// resolved to ids by the client), saved all or nothing as drafts. Every row is validated first and
// all problems are returned together, each prefixed with its spreadsheet row.
func (ctrl *PurchaseInvoiceController) Import(c *fiber.Ctx) error {
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
		for _, m := range validation.Struct(&r.CreateDTO) {
			msgs = append(msgs, fmt.Sprintf("Row %d: %s", r.Row, m))
		}
	}
	if msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}

	created, err := ctrl.svc.Import(middlewares.GetCompanyID(c), middlewares.GetUserID(c), body.Invoices)
	if err != nil {
		return purchaseInvoiceErr(c, err)
	}
	added, updated := 0, 0
	for _, inv := range created {
		if inv.Updated {
			updated++
			ctrl.doc.record(c, audit.ActionUpdated, inv.ID, inv.Number, "Updated "+inv.Number+" from an import", nil)
			continue
		}
		added++
		ctrl.doc.record(c, audit.ActionCreated, inv.ID, inv.Number, "Imported "+inv.Number, nil)
	}
	return utils.Created(c, fiber.Map{"created": added, "updated": updated, "invoices": created}, importSummary(added, updated, "invoice"))
}

func (ctrl *PurchaseInvoiceController) Update(c *fiber.Ctx) error {
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
	if err := uploadDocumentAttachment(c, ctrl.sso, "purchase-invoice", existingAttachment, &dto.AttachmentData); err != nil {
		return attachmentUploadFailed(c, err)
	}
	if err := uploadDocumentAttachment(c, ctrl.sso, "signature", existingSignature, &dto.SignatureData); err != nil {
		return attachmentUploadFailed(c, err)
	}
	row, err := ctrl.svc.Update(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"), &dto)
	if err != nil {
		return purchaseInvoiceErr(c, err)
	}
	ctrl.doc.record(c, audit.ActionUpdated, row.ID, row.Number, "Updated "+row.Number, docDiff(before, row))
	return utils.Ok(c, row, "Invoice updated")
}

// deleteOne — the guarded delete + its audit entry, shared by Delete and BulkDelete.
func (ctrl *PurchaseInvoiceController) deleteOne(c *fiber.Ctx, id string) error {
	companyID := middlewares.GetCompanyID(c)
	before, _ := ctrl.svc.Get(companyID, id)
	if err := ctrl.svc.Delete(companyID, id); err != nil {
		return err
	}
	if before != nil {
		ctrl.doc.record(c, audit.ActionDeleted, before.ID, before.Number, "Deleted "+before.Number, nil)
	}
	return nil
}

func (ctrl *PurchaseInvoiceController) Delete(c *fiber.Ctx) error {
	if err := ctrl.deleteOne(c, c.Params("id")); err != nil {
		return purchaseInvoiceErr(c, err)
	}
	return utils.Deleted(c, "Invoice deleted")
}

// POST bulk-delete — {"ids": [...]}, up to 100 at once. Each id goes through the exact same guarded
// delete as the single-row endpoint; a row that's still referenced fails on its own without blocking
// the rest of the batch.
func (ctrl *PurchaseInvoiceController) BulkDelete(c *fiber.Ctx) error {
	ids, ok := parseBulkIDs(c)
	if !ok {
		return nil
	}
	return runBulk(c, ids, func(id string) error { return ctrl.deleteOne(c, id) }, "Bulk delete processed")
}

// confirmOne / draftOne — the status change + its audit entry, shared by the single-row and bulk endpoints.
func (ctrl *PurchaseInvoiceController) confirmOne(c *fiber.Ctx, id string) (*model.PurchaseInvoice, error) {
	row, err := ctrl.svc.Confirm(middlewares.GetCompanyID(c), middlewares.GetUserID(c), id)
	if err != nil {
		return nil, err
	}
	if !confirmsJustCreated(c, row.CreatedAt, row.CreatedBy) {
		ctrl.doc.statusChange(c, audit.ActionStatusChanged, row.ID, row.Number, "Confirmed", "draft", string(row.Status))
	}
	return row, nil
}

func (ctrl *PurchaseInvoiceController) draftOne(c *fiber.Ctx, id string) (*model.PurchaseInvoice, error) {
	row, err := ctrl.svc.BackToDraft(middlewares.GetCompanyID(c), middlewares.GetUserID(c), id)
	if err != nil {
		return nil, err
	}
	ctrl.doc.statusChange(c, audit.ActionStatusChanged, row.ID, row.Number, "Moved back to draft:", "confirmed", string(row.Status))
	return row, nil
}

func (ctrl *PurchaseInvoiceController) Confirm(c *fiber.Ctx) error {
	row, err := ctrl.confirmOne(c, c.Params("id"))
	if err != nil {
		return purchaseInvoiceErr(c, err)
	}
	return utils.Ok(c, row, "Invoice confirmed")
}

// POST bulk-confirm — {"ids": [...]}: confirms each draft on its own (a non-draft or invalid one fails
// alone, the rest go through).
func (ctrl *PurchaseInvoiceController) BulkConfirm(c *fiber.Ctx) error {
	ids, ok := parseBulkIDs(c)
	if !ok {
		return nil
	}
	return runBulk(c, ids, func(id string) error {
		_, err := ctrl.confirmOne(c, id)
		return err
	}, "Bulk confirm processed")
}

func (ctrl *PurchaseInvoiceController) BackToDraft(c *fiber.Ctx) error {
	row, err := ctrl.draftOne(c, c.Params("id"))
	if err != nil {
		return purchaseInvoiceErr(c, err)
	}
	return utils.Ok(c, row, "Invoice moved back to draft")
}

// POST bulk-draft — {"ids": [...]}: moves each document back to draft on its own.
func (ctrl *PurchaseInvoiceController) BulkBackToDraft(c *fiber.Ctx) error {
	ids, ok := parseBulkIDs(c)
	if !ok {
		return nil
	}
	return runBulk(c, ids, func(id string) error {
		_, err := ctrl.draftOne(c, id)
		return err
	}, "Bulk back to draft processed")
}

func (ctrl *PurchaseInvoiceController) Cancel(c *fiber.Ctx) error {
	row, err := ctrl.svc.Cancel(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"))
	if err != nil {
		return purchaseInvoiceErr(c, err)
	}
	ctrl.doc.statusChange(c, audit.ActionCancelled, row.ID, row.Number, "Cancelled", "", string(row.Status))
	return utils.Ok(c, row, "Invoice cancelled")
}

func purchaseInvoiceErr(c *fiber.Ctx, err error) error {
	if handled, resp := handleActivationError(c, err); handled {
		return resp
	}
	var importErrs *utils.ImportErrors
	if errors.As(err, &importErrs) {
		return utils.ValidationFailed(c, importErrs.Messages)
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
	var inUse *utils.ErrInUse
	if errors.As(err, &inUse) {
		return utils.InUse(c, inUse.Message)
	}
	var v *domain.ErrValidation
	if errors.As(err, &v) {
		return utils.ValidationFailed(c, []string{err.Error()})
	}
	return utils.InternalError(c, err)
}

type setPurchaseInvoiceTemplateDTO struct {
	Template string `json:"template" validate:"required,oneof=template_1 template_2 template_3 template_4 template_5 template_6 template_7"`
}

func (ctrl *PurchaseInvoiceController) SetTemplate(c *fiber.Ctx) error {
	var dto setPurchaseInvoiceTemplateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.SetTemplate(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"), dto.Template)
	if err != nil {
		return purchaseInvoiceErr(c, err)
	}
	return utils.Ok(c, row, "Template updated")
}

func (ctrl *PurchaseInvoiceController) Summary(c *fiber.Ctx) error {
	res, err := ctrl.svc.Summary(middlewares.GetCompanyID(c))
	if err != nil {
		return utils.InternalError(c, err)
	}
	return utils.Ok(c, res, "OK")
}
