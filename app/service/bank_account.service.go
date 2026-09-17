package service

import (
	domain "duluin_invoice/app/domain/bankaccount"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type BankAccountService struct{ repo domain.IRepository }

func NewBankAccountService(repo domain.IRepository) domain.IService {
	return &BankAccountService{repo: repo}
}

func (s *BankAccountService) Create(companyID, actorID string, dto *domain.CreateDTO) (*model.BankAccount, error) {
	dto.CompanyID = companyID
	b, err := s.repo.Create(dto, actorID)
	if err != nil {
		return nil, err
	}
	if b.IsPrimary {
		if err := s.repo.ClearPrimary(companyID, b.ID); err != nil {
			return nil, err
		}
	}
	return b, nil
}

func (s *BankAccountService) Update(companyID, actorID, id string, dto *domain.UpdateDTO) (*model.BankAccount, error) {
	b, err := s.repo.Update(companyID, id, dto, actorID)
	if err != nil {
		return nil, err
	}
	if b.IsPrimary {
		if err := s.repo.ClearPrimary(companyID, b.ID); err != nil {
			return nil, err
		}
	}
	return b, nil
}

func (s *BankAccountService) Get(companyID, id string) (*model.BankAccount, error) {
	return s.repo.FindByID(companyID, id)
}

func (s *BankAccountService) List(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	return s.repo.FindAll(f)
}

func (s *BankAccountService) Delete(companyID, id string) error {
	return s.repo.Delete(companyID, id)
}
