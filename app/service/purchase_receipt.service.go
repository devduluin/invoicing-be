package service

import (
	domain "duluin_invoice/app/domain/purchasereceipt"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type PurchaseReceiptService struct{ repo domain.IRepository }

func NewPurchaseReceiptService(repo domain.IRepository) domain.IService {
	return &PurchaseReceiptService{repo: repo}
}

func (s *PurchaseReceiptService) Create(companyID, actorID string, dto *domain.CreateDTO) (*model.PurchaseReceipt, error) {
	if !model.IsValidPurchaseReceiptPaymentMethod(dto.PaymentMethod) {
		return nil, &domain.ErrValidation{Message: "invalid payment method"}
	}
	ok, err := s.repo.MitraExists(companyID, dto.MitraID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, &domain.ErrValidation{Message: "partner not found"}
	}
	dto.CompanyID = companyID
	return s.repo.Create(dto, actorID)
}

func (s *PurchaseReceiptService) Get(companyID, id string) (*model.PurchaseReceipt, error) {
	return s.repo.FindByID(companyID, id)
}

func (s *PurchaseReceiptService) List(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	return s.repo.FindAll(f)
}

func (s *PurchaseReceiptService) PreviewNumber(companyID string) (string, error) {
	return s.repo.PreviewNumber(companyID)
}

func (s *PurchaseReceiptService) Update(companyID, actorID, id string, dto *domain.UpdateDTO) (*model.PurchaseReceipt, error) {
	if !model.IsValidPurchaseReceiptPaymentMethod(dto.PaymentMethod) {
		return nil, &domain.ErrValidation{Message: "invalid payment method"}
	}
	ok, err := s.repo.MitraExists(companyID, dto.MitraID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, &domain.ErrValidation{Message: "partner not found"}
	}
	dto.CompanyID = companyID
	return s.repo.Update(companyID, id, dto, actorID)
}

func (s *PurchaseReceiptService) Delete(companyID, id string) error {
	return s.repo.Delete(companyID, id)
}
