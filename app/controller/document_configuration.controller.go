package controller

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v2"

	audit "duluin_invoice/app/domain/audit"
	domain "duluin_invoice/app/domain/documentconfig"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type DocumentConfigurationController struct {
	svc   domain.IService
	audit audit.ILogger
}

func NewDocumentConfigurationController(svc domain.IService, auditSvc audit.ILogger) *DocumentConfigurationController {
	return &DocumentConfigurationController{svc: svc, audit: auditSvc}
}

func (ctrl *DocumentConfigurationController) fail(c *fiber.Ctx, err error) error {
	var v *domain.ErrValidation
	if errors.As(err, &v) {
		return utils.ValidationFailed(c, []string{err.Error()})
	}
	return utils.InternalError(c, err)
}

func (ctrl *DocumentConfigurationController) List(c *fiber.Ctx) error {
	items, err := ctrl.svc.List(middlewares.GetCompanyID(c))
	if err != nil {
		return ctrl.fail(c, err)
	}
	return utils.Ok(c, items, "OK")
}

func (ctrl *DocumentConfigurationController) Get(c *fiber.Ctx) error {
	item, err := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("doc_type"))
	if err != nil {
		return ctrl.fail(c, err)
	}
	return utils.Ok(c, item, "OK")
}

func (ctrl *DocumentConfigurationController) Save(c *fiber.Ctx) error {
	var cfg domain.Config
	if err := c.BodyParser(&cfg); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	docType := c.Params("doc_type")
	before, _ := ctrl.svc.Get(middlewares.GetCompanyID(c), docType) // best-effort, for the diff only

	item, err := ctrl.svc.Save(middlewares.GetCompanyID(c), middlewares.GetUserID(c), docType, &cfg)
	if err != nil {
		return ctrl.fail(c, err)
	}

	changes := map[string]audit.Change{}
	if before != nil {
		changes = jsonFieldDiff(string(before.Config), string(item.Config))
	}
	ctrl.audit.Log(auditActor(c), audit.Entry{
		Action:      audit.ActionUpdated,
		Module:      audit.ModuleSettings,
		EntityType:  "document_configuration",
		EntityID:    docType,
		EntityName:  docType,
		Description: fmt.Sprintf("Updated document settings for %s", docType),
		Changes:     changes,
	})
	return utils.Ok(c, item, "Configuration saved successfully")
}

func (ctrl *DocumentConfigurationController) Reset(c *fiber.Ctx) error {
	docType := c.Params("doc_type")
	if err := ctrl.svc.Reset(middlewares.GetCompanyID(c), docType); err != nil {
		return ctrl.fail(c, err)
	}
	ctrl.audit.Log(auditActor(c), audit.Entry{
		Action:      audit.ActionUpdated,
		Module:      audit.ModuleSettings,
		EntityType:  "document_configuration",
		EntityID:    docType,
		EntityName:  docType,
		Description: fmt.Sprintf("Reset document settings for %s to default", docType),
	})
	return utils.Ok(c, nil, "Configuration reset to default")
}
