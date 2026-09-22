package service

import (
	"math"
	"strings"

	domain "duluin_invoice/app/domain/purchaseinvoice"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type PurchaseInvoiceService struct {
	repo       domain.IRepository
	activation transactionLimiter
}

func NewPurchaseInvoiceService(repo domain.IRepository, activation transactionLimiter) domain.IService {
	return &PurchaseInvoiceService{repo: repo, activation: activation}
}

func (s *PurchaseInvoiceService) Create(companyID, actorID string, dto *domain.CreateDTO) (*model.PurchaseInvoice, error) {
	if err := s.checkMitra(companyID, dto.MitraID); err != nil {
		return nil, err
	}
	if err := s.activation.CheckTransactionLimit(companyID); err != nil {
		return nil, err
	}
	calc, err := s.calc(companyID, dto.Lines, dto.AdditionalDiscountType, dto.AdditionalDiscountValue, dto.ShippingCost)
	if err != nil {
		return nil, err
	}
	dto.CompanyID = companyID
	row, err := s.repo.Create(dto, calc, actorID)
	if err != nil {
		return nil, err
	}
	s.activation.Recompute(companyID)
	return row, nil
}

func (s *PurchaseInvoiceService) Update(companyID, actorID, id string, dto *domain.UpdateDTO) (*model.PurchaseInvoice, error) {
	if _, err := s.repo.FindByID(companyID, id); err != nil {
		return nil, err
	}
	if err := s.checkMitra(companyID, dto.MitraID); err != nil {
		return nil, err
	}
	calc, err := s.calc(companyID, dto.Lines, dto.AdditionalDiscountType, dto.AdditionalDiscountValue, dto.ShippingCost)
	if err != nil {
		return nil, err
	}
	return s.repo.Update(companyID, id, dto, calc, actorID)
}

func (s *PurchaseInvoiceService) Get(companyID, id string) (*model.PurchaseInvoice, error) {
	return s.repo.FindByID(companyID, id)
}

func (s *PurchaseInvoiceService) List(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	return s.repo.FindAll(f)
}

func (s *PurchaseInvoiceService) Delete(companyID, id string) error {
	if _, err := s.repo.FindByID(companyID, id); err != nil {
		return err
	}
	return s.repo.Delete(companyID, id)
}

func (s *PurchaseInvoiceService) Confirm(companyID, actorID, id string) (*model.PurchaseInvoice, error) {
	invoice, err := s.repo.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}
	if invoice.Status != model.PurchaseInvoiceStatusDraft {
		return nil, &domain.ErrInvalidTransition{Message: "only a draft invoice can be confirmed"}
	}
	// Re-validate before confirming — mirrors Purchase Order / Sales Invoice's Confirm().
	lines := make([]domain.LineDTO, len(invoice.Lines))
	for i, l := range invoice.Lines {
		lines[i] = domain.LineDTO{
			ProductName: l.ProductName, Description: l.Description, Quantity: l.Quantity,
			UnitPrice: l.UnitPrice, DiscountType: l.DiscountType, DiscountValue: l.DiscountValue, TaxIDs: l.TaxIDs,
		}
	}
	if _, err := s.calc(companyID, lines, invoice.AdditionalDiscountType, invoice.AdditionalDiscountValue, invoice.ShippingCost); err != nil {
		return nil, err
	}
	if err := s.repo.SetStatus(companyID, id, actorID, model.PurchaseInvoiceStatusConfirmed); err != nil {
		return nil, err
	}
	return s.repo.FindByID(companyID, id)
}

func (s *PurchaseInvoiceService) BackToDraft(companyID, actorID, id string) (*model.PurchaseInvoice, error) {
	invoice, err := s.repo.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}
	if invoice.Status == model.PurchaseInvoiceStatusDraft {
		return nil, &domain.ErrInvalidTransition{Message: "this invoice is already a draft"}
	}
	if err := s.repo.SetStatus(companyID, id, actorID, model.PurchaseInvoiceStatusDraft); err != nil {
		return nil, err
	}
	return s.repo.FindByID(companyID, id)
}

func (s *PurchaseInvoiceService) Cancel(companyID, actorID, id string) (*model.PurchaseInvoice, error) {
	invoice, err := s.repo.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}
	if invoice.Status == model.PurchaseInvoiceStatusCancelled {
		return nil, &domain.ErrInvalidTransition{Message: "this invoice is already cancelled"}
	}
	if err := s.repo.SetStatus(companyID, id, actorID, model.PurchaseInvoiceStatusCancelled); err != nil {
		return nil, err
	}
	return s.repo.FindByID(companyID, id)
}

func (s *PurchaseInvoiceService) checkMitra(companyID, mitraID string) error {
	ok, err := s.repo.MitraExists(companyID, mitraID)
	if err != nil {
		return err
	}
	if !ok {
		return &domain.ErrValidation{Message: "partner not found"}
	}
	return nil
}

// calc resolves referenced tax rates then delegates to the shared
// utils.CalcLines (also used by purchase_order.service.go).
func (s *PurchaseInvoiceService) calc(companyID string, lines []domain.LineDTO, discType string, discValue float64, shippingCost float64) (*utils.LinesCalc, error) {
	if shippingCost < 0 {
		return nil, &domain.ErrValidation{Message: "shipping cost cannot be negative"}
	}

	rates, err := s.resolveTaxRates(companyID, lines)
	if err != nil {
		return nil, err
	}

	inputs := make([]utils.LineInput, len(lines))
	for i, l := range lines {
		inputs[i] = utils.LineInput{
			ProductName: l.ProductName, Description: l.Description,
			Quantity: l.Quantity, UnitPrice: l.UnitPrice,
			DiscountType: l.DiscountType, DiscountValue: l.DiscountValue, TaxIDs: l.TaxIDs,
		}
	}

	calc, err := utils.CalcLines(inputs, rates, utils.AdditionalDiscount{Type: discType, Value: discValue})
	if err != nil {
		return nil, &domain.ErrValidation{Message: err.Error()}
	}
	// Shipping is a flat, untaxed fee added on top of the total, after the
	// additional discount — not part of the shared line-calc engine since
	// Sales/Purchase Order don't have this field.
	calc.GrandTotal = math.Round((calc.GrandTotal+shippingCost)*100) / 100
	return calc, nil
}

func (s *PurchaseInvoiceService) resolveTaxRates(companyID string, lines []domain.LineDTO) (map[string]utils.TaxRate, error) {
	taxIDs := make([]string, 0, len(lines))
	seen := make(map[string]bool, len(lines))
	for _, l := range lines {
		for _, raw := range l.TaxIDs {
			id := strings.TrimSpace(raw)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			taxIDs = append(taxIDs, id)
		}
	}
	if len(taxIDs) == 0 {
		return map[string]utils.TaxRate{}, nil
	}
	return s.repo.TaxRates(companyID, taxIDs)
}

func (s *PurchaseInvoiceService) PreviewNumber(companyID string) (string, error) {
	return s.repo.PreviewNumber(companyID)
}

// SetTemplate changes which layout the document prints with (any status; presentation only).
func (s *PurchaseInvoiceService) SetTemplate(companyID, actorID, id, template string) (*model.PurchaseInvoice, error) {
	if !model.IsValidSalesInvoiceTemplate(template) {
		return nil, &domain.ErrValidation{Message: "unknown template"}
	}
	if err := s.repo.SetTemplate(companyID, id, actorID, template); err != nil {
		return nil, err
	}
	return s.repo.FindByID(companyID, id)
}

// Summary returns the dashboard figures for the company's purchase invoices.
func (s *PurchaseInvoiceService) Summary(companyID string) (*domain.Summary, error) {
	return s.repo.Summary(companyID)
}
