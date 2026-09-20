package service

import (
	domain "duluin_invoice/app/domain/unit"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type UnitService struct{ repo domain.IRepository }

func NewUnitService(repo domain.IRepository) domain.IService { return &UnitService{repo: repo} }

func (s *UnitService) Create(companyID, actorID string, dto *domain.CreateDTO) (*model.Unit, error) {
	dto.CompanyID = companyID
	return s.repo.Create(dto, actorID)
}

func (s *UnitService) Update(companyID, actorID, id string, dto *domain.UpdateDTO) (*model.Unit, error) {
	return s.repo.Update(companyID, id, dto, actorID)
}

func (s *UnitService) Get(companyID, id string) (*model.Unit, error) {
	return s.repo.FindByID(companyID, id)
}

func (s *UnitService) List(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	res, err := s.repo.FindAll(f)
	if err != nil {
		return nil, err
	}
	// Self-heal: a company created before this module existed has no units yet.
	if res.Meta.TotalItems == 0 && f.Search == "" {
		if seedErr := s.repo.SeedDefaults(f.CompanyID, ""); seedErr == nil {
			return s.repo.FindAll(f)
		}
	}
	return res, nil
}

func (s *UnitService) Delete(companyID, id string) error { return s.repo.Delete(companyID, id) }
