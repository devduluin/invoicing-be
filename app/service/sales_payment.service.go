package service

import (
	domain "duluin_invoice/app/domain/salespayment"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type SalesPaymentService struct{ repo domain.IRepository }

func NewSalesPaymentService(repo domain.IRepository) domain.IService {
	return &SalesPaymentService{repo: repo}
}

func (s *SalesPaymentService) Create(companyID, actorID string, dto *domain.CreateDTO) (*model.SalesPayment, error) {
	if !model.IsValidSalesReceiptPaymentMethod(dto.PaymentMethod) {
		return nil, &domain.ErrValidation{Message: "invalid payment method"}
	}
	invoice, err := s.repo.InvoiceForPayment(companyID, dto.SalesInvoiceID)
	if err != nil {
		return nil, err
	}
	if invoice.Status != model.SalesInvoiceStatusConfirmed {
		return nil, &domain.ErrValidation{Message: "only a confirmed invoice can receive payments"}
	}
	dto.CompanyID = companyID
	dto.MitraID = invoice.MitraID
	return s.repo.Create(dto, actorID)
}

func (s *SalesPaymentService) Get(companyID, id string) (*model.SalesPayment, error) {
	return s.repo.FindByID(companyID, id)
}

func (s *SalesPaymentService) List(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	return s.repo.FindAll(f)
}

func (s *SalesPaymentService) Verify(companyID, actorID, id string) (*model.SalesPayment, error) {
	return s.repo.Verify(companyID, actorID, id)
}
