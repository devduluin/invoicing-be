package controller

import (
	"github.com/gofiber/fiber/v2"

	meta "duluin_invoice/app/domain/meta"
	"duluin_invoice/utils"
)

type MetaController struct{ banks meta.IBankDirectory }

func NewMetaController(banks meta.IBankDirectory) *MetaController {
	return &MetaController{banks: banks}
}

// Banks — the Indonesian bank directory for the "Akun Bank" dropdown.
func (ctrl *MetaController) Banks(c *fiber.Ctx) error {
	rows, err := ctrl.banks.ListBanks(c.Context())
	if err != nil {
		return utils.InternalError(c, err)
	}
	return utils.Ok(c, rows, "OK")
}
