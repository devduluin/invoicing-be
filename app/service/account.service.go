package service

import (
	domain "duluin_invoice/app/domain/account"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type AccountService struct{ repo domain.IRepository }

func NewAccountService(repo domain.IRepository) domain.IService { return &AccountService{repo: repo} }

func (s *AccountService) Create(companyID, actorID string, dto *domain.CreateDTO) (*model.Account, error) {
	dto.CompanyID = companyID
	if !model.IsValidAccountGroup(dto.Group) {
		return nil, &domain.ErrValidation{Message: "invalid account classification"}
	}
	return s.repo.Create(dto, actorID)
}

func (s *AccountService) Update(companyID, actorID, id string, dto *domain.UpdateDTO) (*model.Account, error) {
	if dto.Group != "" && !model.IsValidAccountGroup(dto.Group) {
		return nil, &domain.ErrValidation{Message: "invalid account classification"}
	}
	return s.repo.Update(companyID, id, dto, actorID)
}

func (s *AccountService) Get(companyID, id string) (*model.Account, error) {
	return s.repo.FindByID(companyID, id)
}

func (s *AccountService) List(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	res, err := s.repo.FindAll(f)
	if err != nil {
		return nil, err
	}
	// Self-heal: a company created before this module existed has no chart yet.
	if res.Meta.TotalItems == 0 && f.Search == "" && f.Group == "" {
		if seedErr := s.repo.SeedDefaults(f.CompanyID, ""); seedErr == nil {
			return s.repo.FindAll(f)
		}
	}
	return res, nil
}

func (s *AccountService) Delete(companyID, id string) error {
	return s.repo.Delete(companyID, id)
}
