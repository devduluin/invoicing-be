// Package service — application layer. No HTTP types leak in here; the HTTP
// context is flattened to domain_onboarding.Actor at the handler boundary.
package service

import (
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"

	membership "duluin_invoice/app/domain/membership"
	domain "duluin_invoice/app/domain/onboarding"
	"duluin_invoice/app/model"
)

// OnboardingService commits the PRD §4 wizard in one shot. Nothing is persisted
// before Submit, so an abandoned wizard leaves no trace.
type OnboardingService struct {
	repo    domain.IOnboardingRepository
	members domain.MembershipPort
	seeder  domain.MasterDataSeeder
}

func NewOnboardingService(
	repo domain.IOnboardingRepository,
	members domain.MembershipPort,
	seeder domain.MasterDataSeeder,
) *OnboardingService {
	return &OnboardingService{repo: repo, members: members, seeder: seeder}
}

var _ domain.IOnboardingService = (*OnboardingService)(nil)

// Submit — the single commit point. Creates the company (active), makes the
// caller its owner, then sends the invitations (best-effort). If owner-linking
// fails the just-created company is rolled back.
func (s *OnboardingService) Submit(actor domain.Actor, dto domain.SubmitDTO) (*domain.SubmitResult, error) {
	if err := domain.ValidateSubmit(dto); err != nil {
		return nil, err
	}
	needs, _ := domain.NormalizeNeeds(dto.KebutuhanUser) // already validated

	code, err := s.uniqueCompanyCode()
	if err != nil {
		return nil, err
	}

	accountType := model.AccountType(dto.TipeAkun)
	company := &model.Company{
		ID:                      uuid.NewString(),
		Name:                    strings.TrimSpace(dto.NamaPerusahaan),
		Code:                    code,
		OwnerName:               strings.TrimSpace(actor.Name),
		OnboardingStatus:        model.OnboardingActive,
		TipeAkun:                &accountType,
		JenisUsaha:              strings.TrimSpace(dto.JenisUsaha),
		JumlahKaryawan:          strings.TrimSpace(dto.JumlahKaryawan),
		Phone:                   strings.TrimSpace(dto.Telepon),
		Email:                   strings.TrimSpace(dto.Email),
		Npwp:                    strings.TrimSpace(dto.Npwp),
		Alamat:                  strings.TrimSpace(dto.Alamat),
		Kota:                    strings.TrimSpace(dto.Kota),
		Provinsi:                strings.TrimSpace(dto.Provinsi),
		KodePos:                 strings.TrimSpace(dto.KodePos),
		KebutuhanUser:           strings.Join(needs, ","),
		FreeTransactionLimitIDR: model.DefaultFreeTransactionLimitIDR,
		CreatedBy:               actor.SSOUserID,
		UpdatedBy:               actor.SSOUserID,
	}

	if err := s.repo.CreateCompany(company); err != nil {
		return nil, err
	}
	if err := s.members.LinkCompanyCreator(actor.ToMembership(), company.ID); err != nil {
		if delErr := s.repo.HardDeleteCompany(company.ID); delErr != nil {
			log.Printf("[onboarding] rollback of company %s failed: %v", company.ID, delErr)
		}
		return nil, fmt.Errorf("link company creator: %w", err)
	}

	// Template COA + taxes (PRD §7/§9). Best-effort — the company already exists.
	if s.seeder != nil {
		if err := s.seeder.SeedCompanyDefaults(company.ID, actor.SSOUserID); err != nil {
			log.Printf("[onboarding] seed master defaults failed company=%s: %v", company.ID, err)
		}
	}

	sent, failed := s.sendInvites(actor, company.ID, dto.Invites)
	return &domain.SubmitResult{
		Company:       company,
		InvitesSent:   sent,
		FailedInvites: failed,
	}, nil
}

// sendInvites is best-effort: the company + owner are already committed, so an
// invite failure is surfaced (FailedInvites) but never rolls anything back.
// Duplicate emails within the payload are collapsed.
func (s *OnboardingService) sendInvites(actor domain.Actor, companyID string, invites []domain.SubmitInvite) (sent int, failed []string) {
	seen := make(map[string]struct{}, len(invites))
	for _, inv := range invites {
		email := strings.ToLower(strings.TrimSpace(inv.Email))
		if email == "" {
			continue
		}
		if _, dup := seen[email]; dup {
			continue
		}
		seen[email] = struct{}{}

		_, err := s.members.Invite(actor.ToMembership(), companyID, membership.InviteInput{
			Email:  email,
			Name:   strings.TrimSpace(inv.Name),
			RoleID: strings.TrimSpace(inv.RoleID),
		})
		if err != nil {
			log.Printf("[onboarding] invite %s failed: %v", email, err)
			failed = append(failed, email)
			continue
		}
		sent++
	}
	return sent, failed
}

// uniqueCompanyCode mints the short, shareable Company ID (network-invoicing
// lookups, PRD §10). Random, not name-derived. The loop is a collision retry
// (like UUID minting), NOT a result-set scan: the 6-char space is ~10^9 so it
// runs exactly once in practice.
func (s *OnboardingService) uniqueCompanyCode() (string, error) {
	for i := 0; i < 8; i++ {
		code := domain.NewCompanyCode(6)
		exists, err := s.repo.CompanyCodeExists(code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return domain.NewCompanyCode(9), nil
}
