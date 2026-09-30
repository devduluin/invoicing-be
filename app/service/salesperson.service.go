package service

import (
	domain "duluin_invoice/app/domain/salesperson"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type SalespersonService struct{ repo domain.IRepository }

func NewSalespersonService(repo domain.IRepository) domain.IService {
	return &SalespersonService{repo: repo}
}

func (s *SalespersonService) Create(companyID, actorID string, dto *domain.CreateDTO) (*model.Salesperson, error) {
	dto.CompanyID = companyID
	return s.repo.Create(dto, actorID)
}

func (s *SalespersonService) Update(companyID, actorID, id string, dto *domain.UpdateDTO) (*model.Salesperson, error) {
	return s.repo.Update(companyID, id, dto, actorID)
}

func (s *SalespersonService) Get(companyID, id string) (*model.Salesperson, error) {
	return s.repo.FindByID(companyID, id)
}

func (s *SalespersonService) List(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	return s.repo.FindAll(f)
}

func (s *SalespersonService) Delete(companyID, id string) error {
	return s.repo.Delete(companyID, id)
}

func (s *SalespersonService) NextCode(companyID string) (string, error) {
	return s.repo.NextCode(companyID)
}

// Mine — the salesperson linked to the calling user (the default on a new sales document), or nil.
func (s *SalespersonService) Mine(companyID, userID string) (*model.Salesperson, error) {
	return s.repo.FindByUser(companyID, userID)
}

func (s *SalespersonService) TeamMembers(companyID string) ([]domain.TeamMember, error) {
	return s.repo.TeamMembers(companyID)
}

func (s *SalespersonService) CreateFromMembers(companyID, actorID string, userIDs []string) ([]model.Salesperson, error) {
	return s.repo.CreateFromMembers(companyID, userIDs, actorID)
}
