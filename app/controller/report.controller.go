package controller

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"

	domain "duluin_invoice/app/domain/report"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type ReportController struct{ svc domain.IService }

func NewReportController(svc domain.IService) *ReportController {
	return &ReportController{svc: svc}
}

// parseReportDate parses "YYYY-MM-DD", falling back to `def` when blank or
// unparsable — reports render best-effort with a sane range rather than
// erroring on a malformed query param.
func parseReportDate(raw string, def time.Time) time.Time {
	if raw == "" {
		return def
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return def
	}
	return t
}

func startOfYear(t time.Time) time.Time {
	return time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, t.Location())
}

func (ctrl *ReportController) TrialBalance(c *fiber.Ctx) error {
	now := time.Now()
	f := &domain.Filter{
		CompanyID: middlewares.GetCompanyID(c),
		StartDate: parseReportDate(c.Query("start_date"), startOfYear(now)),
		EndDate:   parseReportDate(c.Query("end_date"), now),
	}
	res, err := ctrl.svc.TrialBalance(f)
	if err != nil {
		return reportErr(c, err)
	}
	return utils.Ok(c, res, "OK")
}

func (ctrl *ReportController) GeneralLedger(c *fiber.Ctx) error {
	now := time.Now()
	f := &domain.Filter{
		CompanyID: middlewares.GetCompanyID(c),
		StartDate: parseReportDate(c.Query("start_date"), startOfYear(now)),
		EndDate:   parseReportDate(c.Query("end_date"), now),
		AccountID: c.Query("account_id"),
	}
	res, err := ctrl.svc.GeneralLedger(f)
	if err != nil {
		return reportErr(c, err)
	}
	return utils.Ok(c, res, "OK")
}

func (ctrl *ReportController) BalanceSheet(c *fiber.Ctx) error {
	now := time.Now()
	f := &domain.Filter{
		CompanyID: middlewares.GetCompanyID(c),
		EndDate:   parseReportDate(c.Query("as_of_date"), now), // BalanceSheet reads its as-of date from EndDate
	}
	res, err := ctrl.svc.BalanceSheet(f)
	if err != nil {
		return reportErr(c, err)
	}
	return utils.Ok(c, res, "OK")
}

func (ctrl *ReportController) ProfitLoss(c *fiber.Ctx) error {
	now := time.Now()
	f := &domain.Filter{
		CompanyID: middlewares.GetCompanyID(c),
		StartDate: parseReportDate(c.Query("start_date"), startOfYear(now)),
		EndDate:   parseReportDate(c.Query("end_date"), now),
	}
	res, err := ctrl.svc.ProfitLoss(f)
	if err != nil {
		return reportErr(c, err)
	}
	return utils.Ok(c, res, "OK")
}

func reportErr(c *fiber.Ctx, err error) error {
	var v *domain.ErrValidation
	if errors.As(err, &v) {
		return utils.ValidationFailed(c, []string{err.Error()})
	}
	return utils.InternalError(c, err)
}
