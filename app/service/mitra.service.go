package service

import (
	domain "duluin_invoice/app/domain/mitra"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// activationLimiter is the slice of ActivationService this service needs: refuse a new partner past
// the Free-tier cap, and let a qualifying partner trip the one-way activation flip right away.
type activationLimiter interface {
	CheckPartnerLimit(companyID string) error
	Recompute(companyID string)
}

type MitraService struct {
	repo       domain.IMitraRepository
	activation activationLimiter
}

func NewMitraService(repo domain.IMitraRepository, activation activationLimiter) domain.IMitraService {
	return &MitraService{repo: repo, activation: activation}
}

func (s *MitraService) Create(companyID, actorID string, dto *domain.CreateMitraDTO) (*model.Mitra, error) {
	dto.CompanyID = companyID
	if !model.IsValidMitraType(dto.Type) {
		return nil, &domain.ErrValidation{Message: "invalid partner type"}
	}
	if err := s.activation.CheckPartnerLimit(companyID); err != nil {
		return nil, err
	}
	m, err := s.repo.Create(dto, actorID)
	if err != nil {
		return nil, err
	}
	s.activation.Recompute(companyID)
	return m, nil
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
