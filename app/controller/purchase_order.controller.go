package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	audit "duluin_invoice/app/domain/audit"
	domain "duluin_invoice/app/domain/purchaseorder"
	"duluin_invoice/app/model"
	"duluin_invoice/app/sso"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type PurchaseOrderController struct {
	doc documentAudit
	svc domain.IService
	sso *sso.Client
}

func NewPurchaseOrderController(svc domain.IService, ssoClient *sso.Client, auditSvc audit.ILogger) *PurchaseOrderController {
	return &PurchaseOrderController{svc: svc, sso: ssoClient, doc: documentAudit{log: auditSvc, module: audit.ModulePurchaseOrder, entityType: "purchase_order"}}
}

func (ctrl *PurchaseOrderController) List(c *fiber.Ctx) error {
	res, err := ctrl.svc.List(&domain.Filter{
		CompanyID: middlewares.GetCompanyID(c),
		Search:    c.Query("search"),
		MitraID:   c.Query("mitra_id"),
		Status:    c.Query("status"),
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

func (ctrl *PurchaseOrderController) PreviewNumber(c *fiber.Ctx) error {
	number, err := ctrl.svc.PreviewNumber(middlewares.GetCompanyID(c))
	if err != nil {
		return purchaseOrderErr(c, err)
	}
	return utils.Ok(c, fiber.Map{"number": number}, "OK")
}

func (ctrl *PurchaseOrderController) Get(c *fiber.Ctx) error {
	row, err := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"))
	if err != nil {
		return purchaseOrderErr(c, err)
	}
	return utils.Ok(c, row, "OK")
}

func (ctrl *PurchaseOrderController) Create(c *fiber.Ctx) error {
	var dto domain.CreateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	if err := uploadDocumentAttachment(c, ctrl.sso, "purchase-order", "", &dto.AttachmentData); err != nil {
		return attachmentUploadFailed(c, err)
	}
	if err := uploadDocumentAttachment(c, ctrl.sso, "signature", "", &dto.SignatureData); err != nil {
		return attachmentUploadFailed(c, err)
	}
	row, err := ctrl.svc.Create(middlewares.GetCompanyID(c), middlewares.GetUserID(c), &dto)
	if err != nil {
		return purchaseOrderErr(c, err)
	}
	ctrl.doc.record(c, audit.ActionCreated, row.ID, row.Number, "Created "+row.Number, nil)
	return utils.Created(c, row, "Purchase order added")
}

func (ctrl *PurchaseOrderController) Update(c *fiber.Ctx) error {
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
	if err := uploadDocumentAttachment(c, ctrl.sso, "purchase-order", existingAttachment, &dto.AttachmentData); err != nil {
		return attachmentUploadFailed(c, err)
	}
	if err := uploadDocumentAttachment(c, ctrl.sso, "signature", existingSignature, &dto.SignatureData); err != nil {
		return attachmentUploadFailed(c, err)
	}
	row, err := ctrl.svc.Update(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"), &dto)
	if err != nil {
		return purchaseOrderErr(c, err)
	}
	ctrl.doc.record(c, audit.ActionUpdated, row.ID, row.Number, "Updated "+row.Number, docDiff(before, row))
	return utils.Ok(c, row, "Purchase order updated")
}

// deleteOne — the guarded delete + its audit entry, shared by Delete and BulkDelete.
func (ctrl *PurchaseOrderController) deleteOne(c *fiber.Ctx, id string) error {
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

func (ctrl *PurchaseOrderController) Delete(c *fiber.Ctx) error {
	if err := ctrl.deleteOne(c, c.Params("id")); err != nil {
		return purchaseOrderErr(c, err)
	}
	return utils.Deleted(c, "Purchase order deleted")
}

// POST bulk-delete — {"ids": [...]}, up to 100 at once. Each id goes through the exact same guarded
// delete as the single-row endpoint; a row that's still referenced fails on its own without blocking
// the rest of the batch.
func (ctrl *PurchaseOrderController) BulkDelete(c *fiber.Ctx) error {
	ids, ok := parseBulkIDs(c)
	if !ok {
		return nil
	}
	return runBulk(c, ids, func(id string) error { return ctrl.deleteOne(c, id) }, "Bulk delete processed")
}

// confirmOne / draftOne — the status change + its audit entry, shared by the single-row and bulk endpoints.
func (ctrl *PurchaseOrderController) confirmOne(c *fiber.Ctx, id string) (*model.PurchaseOrder, error) {
	row, err := ctrl.svc.Confirm(middlewares.GetCompanyID(c), middlewares.GetUserID(c), id)
	if err != nil {
		return nil, err
	}
	if !confirmsJustCreated(c, row.CreatedAt, row.CreatedBy) {
		ctrl.doc.statusChange(c, audit.ActionStatusChanged, row.ID, row.Number, "Confirmed", "draft", string(row.Status))
	}
	return row, nil
}

func (ctrl *PurchaseOrderController) draftOne(c *fiber.Ctx, id string) (*model.PurchaseOrder, error) {
	row, err := ctrl.svc.BackToDraft(middlewares.GetCompanyID(c), middlewares.GetUserID(c), id)
	if err != nil {
		return nil, err
	}
	ctrl.doc.statusChange(c, audit.ActionStatusChanged, row.ID, row.Number, "Moved back to draft:", "confirmed", string(row.Status))
	return row, nil
}

func (ctrl *PurchaseOrderController) Confirm(c *fiber.Ctx) error {
	row, err := ctrl.confirmOne(c, c.Params("id"))
	if err != nil {
		return purchaseOrderErr(c, err)
	}
	return utils.Ok(c, row, "Purchase order confirmed")
}

// POST bulk-confirm — {"ids": [...]}: confirms each draft on its own (a non-draft or invalid one fails
// alone, the rest go through).
func (ctrl *PurchaseOrderController) BulkConfirm(c *fiber.Ctx) error {
	ids, ok := parseBulkIDs(c)
	if !ok {
		return nil
	}
	return runBulk(c, ids, func(id string) error {
		_, err := ctrl.confirmOne(c, id)
		return err
	}, "Bulk confirm processed")
}

func (ctrl *PurchaseOrderController) BackToDraft(c *fiber.Ctx) error {
	row, err := ctrl.draftOne(c, c.Params("id"))
	if err != nil {
		return purchaseOrderErr(c, err)
	}
	return utils.Ok(c, row, "Purchase order moved back to draft")
}

// POST bulk-draft — {"ids": [...]}: moves each document back to draft on its own.
func (ctrl *PurchaseOrderController) BulkBackToDraft(c *fiber.Ctx) error {
	ids, ok := parseBulkIDs(c)
	if !ok {
		return nil
	}
	return runBulk(c, ids, func(id string) error {
		_, err := ctrl.draftOne(c, id)
		return err
	}, "Bulk back to draft processed")
}

func (ctrl *PurchaseOrderController) Cancel(c *fiber.Ctx) error {
	row, err := ctrl.svc.Cancel(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"))
	if err != nil {
		return purchaseOrderErr(c, err)
	}
	ctrl.doc.statusChange(c, audit.ActionCancelled, row.ID, row.Number, "Cancelled", "", string(row.Status))
	return utils.Ok(c, row, "Purchase order cancelled")
}

func purchaseOrderErr(c *fiber.Ctx, err error) error {
	if handled, resp := handleActivationError(c, err); handled {
		return resp
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

type setPurchaseOrderTemplateDTO struct {
	Template string `json:"template" validate:"required,oneof=template_1 template_2 template_3 template_4 template_5 template_6 template_7"`
}

func (ctrl *PurchaseOrderController) SetTemplate(c *fiber.Ctx) error {
	var dto setPurchaseOrderTemplateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	row, err := ctrl.svc.SetTemplate(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"), dto.Template)
	if err != nil {
		return purchaseOrderErr(c, err)
	}
	return utils.Ok(c, row, "Template updated")
}
