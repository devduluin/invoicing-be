package domain_mitra

import (
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type IMitraRepository interface {
	Create(dto *CreateMitraDTO, actorID string) (*model.Mitra, error)
	// ImportMany saves every partner of an import in one transaction: a code the company already has
	// updates that partner, a new or blank code creates one.
	ImportMany(items []ImportMitraItem, actorID string, prior []string) ([]ImportMitraResult, error)
	// ImportExistingCodes — which of these partner codes (lower-cased) the company already uses.
	ImportExistingCodes(companyID string, codes []string) (map[string]bool, error)
	Update(companyID, id string, dto *UpdateMitraDTO, actorID string) (*model.Mitra, error)
	FindByID(companyID, id string) (*model.Mitra, error)
	FindAll(filter *MitraFilter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
	// CountActive — how many partners this company has. Used by the activation milestone and the
	// Free-tier partner limit.
	CountActive(companyID string) (int64, error)
	// NextCode — what the next generated code would be right now (a preview, not a reservation).
	NextCode(companyID string) (string, error)
}

type IMitraService interface {
	Create(companyID, actorID string, dto *CreateMitraDTO) (*model.Mitra, error)
	Import(companyID, actorID string, rows []ImportMitraRow) ([]ImportMitraResult, error)
	Update(companyID, actorID, id string, dto *UpdateMitraDTO) (*model.Mitra, error)
	Get(companyID, id string) (*model.Mitra, error)
	List(filter *MitraFilter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
	NextCode(companyID string) (string, error)
}
