package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	audit "duluin_invoice/app/domain/audit"
	domain "duluin_invoice/app/domain/goodsreceipt"
	"duluin_invoice/app/sso"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type GoodsReceiptController struct {
	doc documentAudit
	svc domain.IService
	sso *sso.Client
}

func NewGoodsReceiptController(svc domain.IService, ssoClient *sso.Client, auditSvc audit.ILogger) *GoodsReceiptController {
	return &GoodsReceiptController{svc: svc, sso: ssoClient, doc: documentAudit{log: auditSvc, module: audit.ModuleGoodsReceipt, entityType: "goods_receipt"}}
}

func (ctrl *GoodsReceiptController) List(c *fiber.Ctx) error {
	res, err := ctrl.svc.List(&domain.Filter{
		CompanyID:   middlewares.GetCompanyID(c),
		WithDetails: c.Query("with") == "details",
		Search:      c.Query("search"),
		MitraID:     c.Query("mitra_id"),
		Page:        c.QueryInt("page", 1),
		PageSize:    c.QueryInt("limit", 20),
		Sort:        c.Query("sort"),
		Order:       c.Query("order"),
		Fields:      utils.ParseCSVParam(c.Query("fields")),
	})
	if err != nil {
		return utils.InternalError(c, err)
	}
	return utils.List(c, res, "OK")
}

func (ctrl *GoodsReceiptController) PreviewNumber(c *fiber.Ctx) error {
	number, err := ctrl.svc.PreviewNumber(middlewares.GetCompanyID(c))
	if err != nil {
		return goodsReceiptErr(c, err)
	}
	return utils.Ok(c, fiber.Map{"number": number}, "OK")
}

func (ctrl *GoodsReceiptController) Get(c *fiber.Ctx) error {
	row, err := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"))
	if err != nil {
		return goodsReceiptErr(c, err)
	}
	return utils.Ok(c, row, "OK")
}

func (ctrl *GoodsReceiptController) Create(c *fiber.Ctx) error {
	var dto domain.CreateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	if err := uploadDocumentAttachment(c, ctrl.sso, "goods-receipt", "", &dto.AttachmentData); err != nil {
		return attachmentUploadFailed(c, err)
	}
	row, err := ctrl.svc.Create(middlewares.GetCompanyID(c), middlewares.GetUserID(c), &dto)
	if err != nil {
		return goodsReceiptErr(c, err)
	}
	ctrl.doc.record(c, audit.ActionCreated, row.ID, row.Number, "Created "+row.Number, nil)
	return utils.Created(c, row, "Goods receipt added")
}

func (ctrl *GoodsReceiptController) Update(c *fiber.Ctx) error {
	var dto domain.UpdateDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	before, _ := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"))
	existingAttachment := ""
	if before != nil {
		existingAttachment = before.AttachmentData
	}
	if err := uploadDocumentAttachment(c, ctrl.sso, "goods-receipt", existingAttachment, &dto.AttachmentData); err != nil {
		return attachmentUploadFailed(c, err)
	}
	row, err := ctrl.svc.Update(middlewares.GetCompanyID(c), middlewares.GetUserID(c), c.Params("id"), &dto)
	if err != nil {
		return goodsReceiptErr(c, err)
	}
	ctrl.doc.record(c, audit.ActionUpdated, row.ID, row.Number, "Updated "+row.Number, docDiff(before, row))
	return utils.Ok(c, row, "Goods receipt updated")
}

// deleteOne — the guarded delete + its audit entry, shared by Delete and BulkDelete.
func (ctrl *GoodsReceiptController) deleteOne(c *fiber.Ctx, id string) error {
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

func (ctrl *GoodsReceiptController) Delete(c *fiber.Ctx) error {
	if err := ctrl.deleteOne(c, c.Params("id")); err != nil {
		return goodsReceiptErr(c, err)
	}
	return utils.Deleted(c, "Goods receipt deleted")
}

// POST bulk-delete — {"ids": [...]}, up to 100 at once. Each id goes through the exact same guarded
// delete as the single-row endpoint; a row that's still referenced fails on its own without blocking
// the rest of the batch.
func (ctrl *GoodsReceiptController) BulkDelete(c *fiber.Ctx) error {
	ids, ok := parseBulkIDs(c)
	if !ok {
		return nil
	}
	return runBulk(c, ids, func(id string) error { return ctrl.deleteOne(c, id) }, "Bulk delete processed")
}

func goodsReceiptErr(c *fiber.Ctx, err error) error {
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
