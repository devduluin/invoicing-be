package service

import (
	domain "duluin_invoice/app/domain/salesreceipt"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type SalesReceiptService struct{ repo domain.IRepository }

func NewSalesReceiptService(repo domain.IRepository) domain.IService {
	return &SalesReceiptService{repo: repo}
}

func (s *SalesReceiptService) Create(companyID, actorID string, dto *domain.CreateDTO) (*model.SalesReceipt, error) {
	if !model.IsValidSalesReceiptPaymentMethod(dto.PaymentMethod) {
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

func (s *SalesReceiptService) Get(companyID, id string) (*model.SalesReceipt, error) {
	return s.repo.FindByID(companyID, id)
}

func (s *SalesReceiptService) List(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	return s.repo.FindAll(f)
}

func (s *SalesReceiptService) PreviewNumber(companyID string) (string, error) {
	return s.repo.PreviewNumber(companyID)
}

func (s *SalesReceiptService) Update(companyID, actorID, id string, dto *domain.UpdateDTO) (*model.SalesReceipt, error) {
	if !model.IsValidSalesReceiptPaymentMethod(dto.PaymentMethod) {
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

func (s *SalesReceiptService) Delete(companyID, id string) error { return s.repo.Delete(companyID, id) }
