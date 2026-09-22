package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/repository"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

// listPermission — what the caller needs to SEE each kind of document. A connected document the
// caller may not list is left out, so this endpoint never reveals more than the lists do.
var connectedListPermission = map[string]string{
	repository.ConnSalesOrder:      "invoice-sales-order-list",
	repository.ConnDownPayment:     "invoice-sales-invoice-list",
	repository.ConnSalesInvoice:    "invoice-sales-invoice-list",
	repository.ConnSalesReceipt:    "invoice-receipt-list",
	repository.ConnDeliveryNote:    "invoice-delivery-note-list",
	repository.ConnPurchaseOrder:   "invoice-purchase-order-list",
	repository.ConnPurchaseInvoice: "invoice-bill-list",
	repository.ConnPurchaseReceipt: "invoice-purchase-receipt-list",
	repository.ConnGoodsReceipt:    "invoice-goods-receipt-list",
}

type ConnectedDocumentController struct {
	repo *repository.ConnectedDocumentRepository
}

func NewConnectedDocumentController(repo *repository.ConnectedDocumentRepository) *ConnectedDocumentController {
	return &ConnectedDocumentController{repo: repo}
}

// List — GET /connected-documents/:type/:id
func (ctrl *ConnectedDocumentController) List(c *fiber.Ctx) error {
	docType := c.Params("type")
	perm, ok := connectedListPermission[docType]
	if !ok {
		return utils.BadRequest(c, []string{"Unknown document type"})
	}
	if !middlewares.HasAnyPermission(c, perm) {
		return utils.Forbidden(c, []string{"You don't have permission to access this resource"})
	}
	docs, err := ctrl.repo.List(middlewares.GetCompanyID(c), docType, c.Params("id"))
	if err != nil {
		if errors.Is(err, repository.ErrConnectedNotFound) {
			return utils.NotFound(c, []string{"Document not found"})
		}
		return utils.InternalError(c, err)
	}
	visible := make([]repository.ConnectedDocument, 0, len(docs))
	for _, d := range docs {
		if middlewares.HasAnyPermission(c, connectedListPermission[d.Type]) {
			visible = append(visible, d)
		}
	}
	return utils.Ok(c, visible, "OK")
}
