package service

import (
	domain "duluin_invoice/app/domain/mitra"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type MitraService struct {
	repo domain.IMitraRepository
}

func NewMitraService(repo domain.IMitraRepository) domain.IMitraService {
	return &MitraService{repo: repo}
}

func (s *MitraService) Create(companyID, actorID string, dto *domain.CreateMitraDTO) (*model.Mitra, error) {
	dto.CompanyID = companyID
	if !model.IsValidMitraType(dto.Type) {
		return nil, &domain.ErrValidation{Message: "invalid partner type"}
	}
	return s.repo.Create(dto, actorID)
}

func (s *MitraService) Update(companyID, actorID, id string, dto *domain.UpdateMitraDTO) (*model.Mitra, error) {
	if dto.Type != "" && !model.IsValidMitraType(dto.Type) {
		return nil, &domain.ErrValidation{Message: "invalid partner type"}
	}
	return s.repo.Update(companyID, id, dto, actorID)
}

func (s *MitraService) Get(companyID, id string) (*model.Mitra, error) {
	return s.repo.FindByID(companyID, id)
}

func (s *MitraService) List(filter *domain.MitraFilter) (*utils.OffsetPaginationResult, error) {
	return s.repo.FindAll(filter)
}

func (s *MitraService) Delete(companyID, id string) error {
	return s.repo.Delete(companyID, id)
}
