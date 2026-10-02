package service

import (
	"fmt"
	"strings"

	domain "duluin_invoice/app/domain/mitra"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// activationLimiter is the slice of ActivationService this service needs: refuse a new partner past
// the Free-tier cap, and let a qualifying partner trip the one-way activation flip right away.
type activationLimiter interface {
	CheckPartnerLimit(companyID string) error
	CheckPartnerCapacity(companyID string, adding int) error
	Recompute(companyID string)
}

type MitraService struct {
	repo       domain.IMitraRepository
	activation activationLimiter
}

func NewMitraService(repo domain.IMitraRepository, activation activationLimiter) domain.IMitraService {
	return &MitraService{repo: repo, activation: activation}
}

func (s *MitraService) Create(companyID, actorID string, dto *domain.CreateMitraDTO) (*model.Mitra, error) {
	dto.CompanyID = companyID
	if !model.IsValidMitraType(dto.Type) {
		return nil, &domain.ErrValidation{Message: "invalid partner type"}
	}
	if err := s.activation.CheckPartnerLimit(companyID); err != nil {
		return nil, err
	}
	m, err := s.repo.Create(dto, actorID)
	if err != nil {
		return nil, err
	}
	s.activation.Recompute(companyID)
	return m, nil
}

// Import creates every partner of an import file, all or nothing. Every row is checked and all
// problems come back together (*utils.ImportErrors). The Free-tier cap is checked for the whole file
// up front, so an import never stops half way at the limit.
func (s *MitraService) Import(companyID, actorID string, rows []domain.ImportMitraRow) ([]domain.ImportMitraResult, error) {
	var msgs []string
	items := make([]domain.ImportMitraItem, 0, len(rows))
	seen := make(map[string]int, len(rows))
	for i := range rows {
		r := &rows[i]
		r.CompanyID = companyID
		label := importRowLabel(r.Row, r.Name)
		if !model.IsValidMitraType(r.Type) {
			msgs = append(msgs, label+": invalid partner type")
			continue
		}
		// one file, one row per partner code: a second row would silently overwrite the first
		if code := strings.ToLower(strings.TrimSpace(r.Code)); code != "" {
			if first, dup := seen[code]; dup {
				msgs = append(msgs, fmt.Sprintf("%s: partner code %s is also used on row %d", label, strings.TrimSpace(r.Code), first))
				continue
			}
			seen[code] = r.Row
		}
		items = append(items, domain.ImportMitraItem{Label: label, DTO: &r.CreateMitraDTO})
	}
	if msgs == nil {
		// only new partners count against the Free-tier cap; a code the company already has is an update
		codes := make([]string, 0, len(items))
		for _, it := range items {
			codes = append(codes, it.DTO.Code)
		}
		existing, err := s.repo.ImportExistingCodes(companyID, codes)
		if err != nil {
			return nil, err
		}
		adding := 0
		for _, it := range items {
			if !existing[strings.ToLower(strings.TrimSpace(it.DTO.Code))] {
				adding++
			}
		}
		if err := s.activation.CheckPartnerCapacity(companyID, adding); err != nil {
			return nil, err
		}
	}
	// the rows are still saved (and rolled back) when the checks above found problems, so what only
	// the data can tell (code in use, company already linked) is reported in the same answer
	out, err := s.repo.ImportMany(items, actorID, msgs)
	if err != nil {
		return nil, err
	}
	s.activation.Recompute(companyID)
	return out, nil
}

// importRowLabel — how an import message points at a partner: "Row 3 (PT Maju)".
func importRowLabel(row int, name string) string {
	if name = strings.TrimSpace(name); name != "" {
		return fmt.Sprintf("Row %d (%s)", row, name)
	}
	return fmt.Sprintf("Row %d", row)
}

func (s *MitraService) Update(companyID, actorID, id string, dto *domain.UpdateMitraDTO) (*model.Mitra, error) {
	if dto.Type != "" && !model.IsValidMitraType(dto.Type) {
		return nil, &domain.ErrValidation{Message: "invalid partner type"}
	}
	return s.repo.Update(companyID, id, dto, actorID)
}

func (s *MitraService) Get(companyID, id string) (*model.Mitra, error) {
	return s.repo.FindByID(companyID, id)
}

func (s *MitraService) List(filter *domain.MitraFilter) (*utils.OffsetPaginationResult, error) {
	return s.repo.FindAll(filter)
}

func (s *MitraService) NextCode(companyID string) (string, error) {
	return s.repo.NextCode(companyID)
}

func (s *MitraService) Delete(companyID, id string) error {
	return s.repo.Delete(companyID, id)
}
