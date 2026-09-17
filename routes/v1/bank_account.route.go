package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// BankAccountRoutes — company receiving-bank master data (PRD §11).
func BankAccountRoutes(router fiber.Router, ctrl *controller.BankAccountController) {
	g := router.Group("/bank-accounts")
	g.Get("/", middlewares.RequirePermission("invoice-bank-account-list"), ctrl.List)
	g.Get("/:id", middlewares.RequirePermission("invoice-bank-account-list"), ctrl.Get)
	g.Post("/", middlewares.RequirePermission("invoice-bank-account-create"), ctrl.Create)
	g.Put("/:id", middlewares.RequirePermission("invoice-bank-account-update"), ctrl.Update)
	g.Delete("/:id", middlewares.RequirePermission("invoice-bank-account-delete"), ctrl.Delete)
}
