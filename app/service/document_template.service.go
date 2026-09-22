package service

import (
	domain "duluin_invoice/app/domain/documenttemplate"
	"duluin_invoice/app/model"
)

type DocumentTemplateService struct{ repo domain.IRepository }

func NewDocumentTemplateService(repo domain.IRepository) domain.IService {
	return &DocumentTemplateService{repo: repo}
}

func (s *DocumentTemplateService) List(companyID string) ([]domain.Item, error) {
	return s.repo.List(companyID)
}

func (s *DocumentTemplateService) Set(companyID, actorID, docType string, dto *domain.SetDTO) (*domain.Item, error) {
	if !model.IsValidDocumentTemplateType(docType) {
		return nil, &domain.ErrValidation{Message: "this document type has no template"}
	}
	if !model.IsValidSalesInvoiceTemplate(dto.Template) {
		return nil, &domain.ErrValidation{Message: "unknown template"}
	}
	return s.repo.Set(companyID, docType, dto.Template, actorID)
}
