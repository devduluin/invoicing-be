package service

import (
	domain "duluin_invoice/app/domain/tax"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type TaxService struct{ repo domain.IRepository }

func NewTaxService(repo domain.IRepository) domain.IService { return &TaxService{repo: repo} }

func (s *TaxService) Create(companyID, actorID string, dto *domain.CreateDTO) (*model.Tax, error) {
	dto.CompanyID = companyID
	if !dto.IsCompound {
		if dto.Kind != "" && !model.IsValidTaxKind(dto.Kind) {
			return nil, &domain.ErrValidation{Message: "invalid tax kind"}
		}
		if dto.CalcMethod != "" && !model.IsValidTaxCalcMethod(dto.CalcMethod) {
			return nil, &domain.ErrValidation{Message: "invalid calculation method"}
		}
	}
	return s.repo.Create(dto, actorID)
}

func (s *TaxService) Update(companyID, actorID, id string, dto *domain.UpdateDTO) (*model.Tax, error) {
	if dto.Kind != "" && !model.IsValidTaxKind(dto.Kind) {
		return nil, &domain.ErrValidation{Message: "invalid tax kind"}
	}
	if dto.CalcMethod != "" && !model.IsValidTaxCalcMethod(dto.CalcMethod) {
		return nil, &domain.ErrValidation{Message: "invalid calculation method"}
	}
	return s.repo.Update(companyID, id, dto, actorID)
}

func (s *TaxService) Get(companyID, id string) (*model.Tax, error) {
	return s.repo.FindByID(companyID, id)
}

func (s *TaxService) List(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	res, err := s.repo.FindAll(f)
	if err != nil {
		return nil, err
	}
	// Self-heal: a company created before this module existed has no taxes yet.
	if res.Meta.TotalItems == 0 && f.Search == "" {
		if seedErr := s.repo.SeedDefaults(f.CompanyID, ""); seedErr == nil {
			return s.repo.FindAll(f)
		}
	}
	return res, nil
}

func (s *TaxService) Delete(companyID, id string) error { return s.repo.Delete(companyID, id) }
