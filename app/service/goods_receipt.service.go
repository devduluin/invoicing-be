package service

import (
	domain "duluin_invoice/app/domain/goodsreceipt"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type GoodsReceiptService struct{ repo domain.IRepository }

func NewGoodsReceiptService(repo domain.IRepository) domain.IService {
	return &GoodsReceiptService{repo: repo}
}

func (s *GoodsReceiptService) Create(companyID, actorID string, dto *domain.CreateDTO) (*model.GoodsReceipt, error) {
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

func (s *GoodsReceiptService) Get(companyID, id string) (*model.GoodsReceipt, error) {
	return s.repo.FindByID(companyID, id)
}

func (s *GoodsReceiptService) List(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	return s.repo.FindAll(f)
}

func (s *GoodsReceiptService) Update(companyID, actorID, id string, dto *domain.UpdateDTO) (*model.GoodsReceipt, error) {
	ok, err := s.repo.MitraExists(companyID, dto.MitraID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, &domain.ErrValidation{Message: "partner not found"}
	}
	return s.repo.Update(companyID, id, dto, actorID)
}

func (s *GoodsReceiptService) Delete(companyID, id string) error { return s.repo.Delete(companyID, id) }
