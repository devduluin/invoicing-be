package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
	"duluin_invoice/middlewares"
)

// ReportRoutes — financial reports computed from posted journal entries.
func ReportRoutes(router fiber.Router, ctrl *controller.ReportController) {
	g := router.Group("/reports")
	g.Get("/trial-balance", middlewares.RequirePermission("invoice-report-trial-balance"), ctrl.TrialBalance)
	g.Get("/general-ledger", middlewares.RequirePermission("invoice-report-general-ledger"), ctrl.GeneralLedger)
	g.Get("/balance-sheet", middlewares.RequirePermission("invoice-report-balance-sheet"), ctrl.BalanceSheet)
	g.Get("/profit-loss", middlewares.RequirePermission("invoice-report-profit-loss"), ctrl.ProfitLoss)
}
