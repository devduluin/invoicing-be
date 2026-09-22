package controller

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	domain "duluin_invoice/app/domain/audit"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type AuditController struct {
	svc domain.IService
}

func NewAuditController(svc domain.IService) *AuditController {
	return &AuditController{svc: svc}
}

func parseAuditDate(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return &t
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return &t
	}
	return nil
}

func splitCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// GET /api/v1/audit-log
func (ctrl *AuditController) List(c *fiber.Ctx) error {
	to := parseAuditDate(c.Query("to"))
	if to != nil {
		end := to.Add(24 * time.Hour) // "to" is inclusive of the whole day
		to = &end
	}
	res, err := ctrl.svc.List(domain.Filter{
		CompanyID: middlewares.GetCompanyID(c),
		Search:    c.Query("search"),
		Actions:   splitCSV(c.Query("action")),
		Modules:   splitCSV(c.Query("module")),
		UserID:    c.Query("user_id"),
		From:      parseAuditDate(c.Query("from")),
		To:        to,
		Page:      c.QueryInt("page", 1),
		PageSize:  c.QueryInt("limit", 20),
	})
	if err != nil {
		return utils.InternalError(c, err)
	}
	return utils.List(c, res, "OK")
}

// GET /api/v1/audit-log/:id
func (ctrl *AuditController) Get(c *fiber.Ctx) error {
	v, err := ctrl.svc.Get(middlewares.GetCompanyID(c), c.Params("id"))
	if err != nil {
		return utils.InternalError(c, err)
	}
	if v == nil {
		return utils.NotFound(c, []string{"audit log entry not found"})
	}
	return utils.Ok(c, v, "OK")
}

type sessionEventBody struct {
	Event string `json:"event"` // "login" | "logout"
}

// POST /api/v1/audit-log/session — records the caller's own login/logout. There is no real
// server-side session here (auth is SSO/Launchpad), so the frontend calls this at the two moments
// that stand in for it: once per browser session right after CompanyGate confirms a valid identity
// + company, and from the /auth/logout route handler just before it revokes the SSO token and
// clears cookies (while the Authorization header is still valid).
func (ctrl *AuditController) LogSession(c *fiber.Ctx) error {
	var body sessionEventBody
	_ = c.BodyParser(&body)
	action := domain.ActionLogin
	description := "Signed in"
	if body.Event == "logout" {
		action = domain.ActionLogout
		description = "Signed out"
	}
	ctrl.svc.Log(auditActor(c), domain.Entry{
		Action:      action,
		Module:      domain.ModuleUserManagement,
		Description: description,
	})
	return utils.Ok(c, fiber.Map{"logged": true}, "OK")
}
