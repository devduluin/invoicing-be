package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
)

// MetaRoutes — read-only reference data. Auth only (no company membership):
// the bank directory is the same for everyone.
func MetaRoutes(router fiber.Router, ctrl *controller.MetaController) {
	g := router.Group("/meta")
	g.Get("/banks", ctrl.Banks)
}
