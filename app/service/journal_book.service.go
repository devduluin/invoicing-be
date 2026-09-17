package service

import (
	domain "duluin_invoice/app/domain/journalbook"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type JournalBookService struct{ repo domain.IRepository }

func NewJournalBookService(repo domain.IRepository) domain.IService {
	return &JournalBookService{repo: repo}
}

func (s *JournalBookService) Create(companyID, actorID string, dto *domain.CreateDTO) (*model.JournalBook, error) {
	dto.CompanyID = companyID
	if !model.IsValidJournalBookType(dto.Type) {
		return nil, &domain.ErrValidation{Message: "invalid journal book type"}
	}
	return s.repo.Create(dto, actorID)
}

func (s *JournalBookService) Update(companyID, actorID, id string, dto *domain.UpdateDTO) (*model.JournalBook, error) {
	if dto.Type != "" && !model.IsValidJournalBookType(dto.Type) {
		return nil, &domain.ErrValidation{Message: "invalid journal book type"}
	}
	return s.repo.Update(companyID, id, dto, actorID)
}

func (s *JournalBookService) Get(companyID, id string) (*model.JournalBook, error) {
	return s.repo.FindByID(companyID, id)
}

func (s *JournalBookService) List(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	res, err := s.repo.FindAll(f)
	if err != nil {
		return nil, err
	}
	// Self-heal: a company created before this module existed has no books yet.
	if res.Meta.TotalItems == 0 && f.Search == "" && f.Type == "" {
		if seedErr := s.repo.SeedDefaults(f.CompanyID, ""); seedErr == nil {
			return s.repo.FindAll(f)
		}
	}
	return res, nil
}

func (s *JournalBookService) Delete(companyID, id string) error {
	return s.repo.Delete(companyID, id)
}
