package service

import (
	"strings"

	domain "duluin_invoice/app/domain/contactperson"
	"duluin_invoice/app/model"
)

type ContactPersonService struct{ repo domain.IRepository }

func NewContactPersonService(repo domain.IRepository) domain.IService {
	return &ContactPersonService{repo: repo}
}

func clean(in *domain.Input) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return &domain.ErrValidation{Message: "contact person name is required"}
	}
	return nil
}

func (s *ContactPersonService) List(companyID, mitraID, search string) ([]model.ContactPerson, error) {
	return s.repo.List(companyID, mitraID, search)
}

func (s *ContactPersonService) Get(companyID, mitraID, id string) (*model.ContactPerson, error) {
	return s.repo.Get(companyID, mitraID, id)
}

func (s *ContactPersonService) Create(companyID, mitraID, actorID string, in *domain.Input) (*model.ContactPerson, error) {
	if err := clean(in); err != nil {
		return nil, err
	}
	return s.repo.Create(companyID, mitraID, actorID, in)
}

func (s *ContactPersonService) Update(companyID, mitraID, id, actorID string, in *domain.Input) (*model.ContactPerson, error) {
	if err := clean(in); err != nil {
		return nil, err
	}
	return s.repo.Update(companyID, mitraID, id, actorID, in)
}

func (s *ContactPersonService) Delete(companyID, mitraID, id string) error {
	return s.repo.Delete(companyID, mitraID, id)
}

func (s *ContactPersonService) Summaries(companyID string) ([]domain.Summary, error) {
	return s.repo.Summaries(companyID)
}
