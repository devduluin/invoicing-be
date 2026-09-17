package service

import (
	"strings"

	domain "duluin_invoice/app/domain/purchaseorder"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type PurchaseOrderService struct{ repo domain.IRepository }

func NewPurchaseOrderService(repo domain.IRepository) domain.IService {
	return &PurchaseOrderService{repo: repo}
}

func (s *PurchaseOrderService) Create(companyID, actorID string, dto *domain.CreateDTO) (*model.PurchaseOrder, error) {
	if err := s.checkMitra(companyID, dto.MitraID); err != nil {
		return nil, err
	}
	calc, err := s.calc(companyID, dto.Lines, dto.AdditionalDiscountType, dto.AdditionalDiscountValue)
	if err != nil {
		return nil, err
	}
	dto.CompanyID = companyID
	return s.repo.Create(dto, calc, actorID)
}

func (s *PurchaseOrderService) Update(companyID, actorID, id string, dto *domain.UpdateDTO) (*model.PurchaseOrder, error) {
	existing, err := s.repo.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}
	if existing.Status != model.PurchaseOrderStatusDraft {
		return nil, &domain.ErrNotEditable{}
	}
	if err := s.checkMitra(companyID, dto.MitraID); err != nil {
		return nil, err
	}
	calc, err := s.calc(companyID, dto.Lines, dto.AdditionalDiscountType, dto.AdditionalDiscountValue)
	if err != nil {
		return nil, err
	}
	return s.repo.Update(companyID, id, dto, calc, actorID)
}

func (s *PurchaseOrderService) Get(companyID, id string) (*model.PurchaseOrder, error) {
	return s.repo.FindByID(companyID, id)
}

func (s *PurchaseOrderService) List(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	return s.repo.FindAll(f)
}

func (s *PurchaseOrderService) Delete(companyID, id string) error {
	existing, err := s.repo.FindByID(companyID, id)
	if err != nil {
		return err
	}
	if existing.Status != model.PurchaseOrderStatusDraft {
		return &domain.ErrNotEditable{}
	}
	return s.repo.Delete(companyID, id)
}

func (s *PurchaseOrderService) Confirm(companyID, actorID, id string) (*model.PurchaseOrder, error) {
	order, err := s.repo.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}
	if order.Status != model.PurchaseOrderStatusDraft {
		return nil, &domain.ErrInvalidTransition{Message: "only a draft order can be confirmed"}
	}
	// Re-validate before confirming — mirrors Sales Order's Confirm().
	lines := make([]domain.LineDTO, len(order.Lines))
	for i, l := range order.Lines {
		lines[i] = domain.LineDTO{
			ProductName: l.ProductName, Description: l.Description, Quantity: l.Quantity,
			UnitPrice: l.UnitPrice, DiscountType: l.DiscountType, DiscountValue: l.DiscountValue, TaxIDs: l.TaxIDs,
		}
	}
	if _, err := s.calc(companyID, lines, order.AdditionalDiscountType, order.AdditionalDiscountValue); err != nil {
		return nil, err
	}
	if err := s.repo.SetStatus(companyID, id, actorID, model.PurchaseOrderStatusConfirmed); err != nil {
		return nil, err
	}
	return s.repo.FindByID(companyID, id)
}

func (s *PurchaseOrderService) BackToDraft(companyID, actorID, id string) (*model.PurchaseOrder, error) {
	order, err := s.repo.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}
	if order.Status == model.PurchaseOrderStatusDraft {
		return nil, &domain.ErrInvalidTransition{Message: "this order is already a draft"}
	}
	if err := s.repo.SetStatus(companyID, id, actorID, model.PurchaseOrderStatusDraft); err != nil {
		return nil, err
	}
	return s.repo.FindByID(companyID, id)
}

func (s *PurchaseOrderService) Cancel(companyID, actorID, id string) (*model.PurchaseOrder, error) {
	order, err := s.repo.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}
	if order.Status == model.PurchaseOrderStatusCancelled {
		return nil, &domain.ErrInvalidTransition{Message: "this order is already cancelled"}
	}
	if err := s.repo.SetStatus(companyID, id, actorID, model.PurchaseOrderStatusCancelled); err != nil {
		return nil, err
	}
	return s.repo.FindByID(companyID, id)
}

func (s *PurchaseOrderService) checkMitra(companyID, mitraID string) error {
	ok, err := s.repo.MitraExists(companyID, mitraID)
	if err != nil {
		return err
	}
	if !ok {
		return &domain.ErrValidation{Message: "partner not found"}
	}
	return nil
}

// calc resolves referenced tax rates then delegates the actual line math to
// the shared utils.CalcLines (also used by purchase_invoice.service.go),
// mapping domain_purchaseorder's DTOs to/from its generic Line{Input,Result} shape.
func (s *PurchaseOrderService) calc(companyID string, lines []domain.LineDTO, discType string, discValue float64) (*domain.OrderCalc, error) {
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

	result, err := utils.CalcLines(inputs, rates, utils.AdditionalDiscount{Type: discType, Value: discValue})
	if err != nil {
		return nil, &domain.ErrValidation{Message: err.Error()}
	}

	calc := &domain.OrderCalc{
		Lines:                    make([]domain.LineCalc, len(result.Lines)),
		Subtotal:                 result.Subtotal,
		DiscountTotal:            result.DiscountTotal,
		TaxTotal:                 result.TaxTotal,
		GrandTotal:               result.GrandTotal,
		AdditionalDiscountType:   result.AdditionalDiscountType,
		AdditionalDiscountValue:  result.AdditionalDiscountValue,
		AdditionalDiscountAmount: result.AdditionalDiscountAmount,
	}
	for i, l := range result.Lines {
		calc.Lines[i] = domain.LineCalc{
			ProductName: l.ProductName, Description: l.Description,
			Quantity: l.Quantity, UnitPrice: l.UnitPrice,
			DiscountType: l.DiscountType, DiscountValue: l.DiscountValue, TaxIDs: l.TaxIDs,
			LineSubtotal: l.LineSubtotal, LineTaxAmount: l.LineTaxAmount, LineTotal: l.LineTotal,
		}
	}
	return calc, nil
}

func (s *PurchaseOrderService) resolveTaxRates(companyID string, lines []domain.LineDTO) (map[string]utils.TaxRate, error) {
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
	info, err := s.repo.TaxRates(companyID, taxIDs)
	if err != nil {
		return nil, err
	}
	rates := make(map[string]utils.TaxRate, len(info))
	for id, t := range info {
		rates[id] = utils.TaxRate{Rate: t.Rate, CalcMethod: t.CalcMethod}
	}
	return rates, nil
}

func (s *PurchaseOrderService) PreviewNumber(companyID string) (string, error) {
	return s.repo.PreviewNumber(companyID)
}
