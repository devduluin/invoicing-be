package controller

import (
	"github.com/gofiber/fiber/v2"

	activation "duluin_invoice/app/domain/activation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type activationReader interface {
	Progress(companyID string) (*activation.Progress, error)
}

type ActivationController struct {
	svc activationReader
}

func NewActivationController(svc activationReader) *ActivationController {
	return &ActivationController{svc: svc}
}

// GET /api/v1/companies/me/activation — the three-step Free-workspace activation checklist
// (company profile, 3 partners, 1 invoice) and the limits that apply right now. No permission gate
// beyond being an active member of the company: every teammate can see it, not just the owner.
func (ctrl *ActivationController) Progress(c *fiber.Ctx) error {
	p, err := ctrl.svc.Progress(middlewares.GetCompanyID(c))
	if err != nil {
		if handled, resp := handleActivationError(c, err); handled {
			return resp
		}
		return utils.InternalError(c, err)
	}
	return utils.Ok(c, p, "OK")
}
