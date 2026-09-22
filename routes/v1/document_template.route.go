package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// DocumentTemplateRoutes — the company's default template per document type. Reading needs
// invoice-template-list, changing it invoice-template-update.
func DocumentTemplateRoutes(router fiber.Router, ctrl *controller.DocumentTemplateController) {
	g := router.Group("/document-templates")
	g.Get("/", middlewares.RequirePermission("invoice-template-list"), ctrl.List)
	g.Put("/:doc_type", middlewares.RequirePermission("invoice-template-update"), ctrl.Set)
}
