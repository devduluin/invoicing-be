package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
)

// ActivationRoutes — read-only Free-workspace activation checklist. Mounted on `business` (company
// context resolved, onboarding complete) with no extra permission: every teammate may see it.
func ActivationRoutes(router fiber.Router, ctrl *controller.ActivationController) {
	router.Get("/companies/me/activation", ctrl.Progress)
}
