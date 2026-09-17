package domain_mitra

import (
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type IMitraRepository interface {
	Create(dto *CreateMitraDTO, actorID string) (*model.Mitra, error)
	Update(companyID, id string, dto *UpdateMitraDTO, actorID string) (*model.Mitra, error)
	FindByID(companyID, id string) (*model.Mitra, error)
	FindAll(filter *MitraFilter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
}

type IMitraService interface {
	Create(companyID, actorID string, dto *CreateMitraDTO) (*model.Mitra, error)
	Update(companyID, actorID, id string, dto *UpdateMitraDTO) (*model.Mitra, error)
	Get(companyID, id string) (*model.Mitra, error)
	List(filter *MitraFilter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
}
