package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
)

func MeRoutes(router fiber.Router, ctrl *controller.MeController) {
	router.Get("/me", ctrl.Get)
}
