package service

import (
	domain "duluin_invoice/app/domain/deliverynote"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type DeliveryNoteService struct{ repo domain.IRepository }

func NewDeliveryNoteService(repo domain.IRepository) domain.IService {
	return &DeliveryNoteService{repo: repo}
}

func (s *DeliveryNoteService) Create(companyID, actorID string, dto *domain.CreateDTO) (*model.DeliveryNote, error) {
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

func (s *DeliveryNoteService) Get(companyID, id string) (*model.DeliveryNote, error) {
	return s.repo.FindByID(companyID, id)
}

func (s *DeliveryNoteService) List(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	return s.repo.FindAll(f)
}

func (s *DeliveryNoteService) Update(companyID, actorID, id string, dto *domain.UpdateDTO) (*model.DeliveryNote, error) {
	ok, err := s.repo.MitraExists(companyID, dto.MitraID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, &domain.ErrValidation{Message: "partner not found"}
	}
	return s.repo.Update(companyID, id, dto, actorID)
}

func (s *DeliveryNoteService) Delete(companyID, id string) error { return s.repo.Delete(companyID, id) }
