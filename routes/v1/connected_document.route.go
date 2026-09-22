package v1

import (
	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/controller"
)

// ConnectedDocumentRoutes — the real, stored relationships of one document. Permission is checked
// per document type inside the handler (the caller needs the list permission of the document being
// opened, and only sees connected documents they could list themselves).
func ConnectedDocumentRoutes(router fiber.Router, ctrl *controller.ConnectedDocumentController) {
	router.Get("/connected-documents/:type/:id", ctrl.List)
}
