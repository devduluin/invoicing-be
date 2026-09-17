package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
)

// OnboardingRoutes — PRD §4. Mounted under baseRouter (identity + company
// context) but WITHOUT the membership/onboarding gates — this is where a company
// and its owner membership are created, in one shot.
func OnboardingRoutes(router fiber.Router, ctrl *controller.OnboardingController) {
	router.Post("/onboarding", ctrl.Submit)
}
