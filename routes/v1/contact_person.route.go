package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// ContactPersonRoutes mounts the contact persons of a partner under /mitra. It must be registered
// BEFORE MitraRoutes: "/contact-summary" would otherwise be swallowed by "/:id".
// Viewing follows invoice-mitra-list; changing has its own permissions.
func ContactPersonRoutes(router fiber.Router, ctrl *controller.ContactPersonController) {
	router.Get("/contact-summary", middlewares.RequirePermission("invoice-mitra-list"), ctrl.Summaries)
	router.Get("/:id/contact-persons", middlewares.RequirePermission("invoice-mitra-list"), ctrl.List)
	router.Get("/:id/contact-persons/:cid", middlewares.RequirePermission("invoice-mitra-list"), ctrl.Get)
	router.Post("/:id/contact-persons", middlewares.RequirePermission("invoice-mitra-contact-create"), ctrl.Create)
	router.Put("/:id/contact-persons/:cid", middlewares.RequirePermission("invoice-mitra-contact-update"), ctrl.Update)
	router.Delete("/:id/contact-persons/:cid", middlewares.RequirePermission("invoice-mitra-contact-delete"), ctrl.Delete)
}
