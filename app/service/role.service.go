package service

import (
	"context"
	"fmt"
	"strings"

	domain "duluin_invoice/app/domain/membership"
)

// RoleCacheInvalidator lets RoleService drop the membership permission cache
// after an edit without depending on the whole MembershipService.
type RoleCacheInvalidator interface {
	InvalidateRoleCache(companyID, roleID string)
}

// RoleService is a thin policy layer over the SSO role catalog: it gates custom
// roles to the active company and blocks deleting a role still in use.
type RoleService struct {
	sso   RBACSSOClient
	repo  domain.IMembershipRepository
	cache RoleCacheInvalidator
}

func NewRoleService(ssoClient RBACSSOClient, repo domain.IMembershipRepository, cache RoleCacheInvalidator) *RoleService {
	return &RoleService{sso: ssoClient, repo: repo, cache: cache}
}

var _ domain.IRoleService = (*RoleService)(nil)

func (s *RoleService) List(a domain.Actor, companyID string) ([]domain.RoleInfo, error) {
	roles, err := s.sso.ListRoles(context.Background(), strings.TrimSpace(companyID), a.Token)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	return toRoleInfos(roles), nil
}

func (s *RoleService) Get(a domain.Actor, companyID, roleID string) (*domain.RoleDetail, error) {
	if strings.TrimSpace(roleID) == "" {
		return nil, &domain.ErrInvalidRole{Msg: "role id is required"}
	}
	d, err := s.sso.GetRoleDetail(context.Background(), roleID, strings.TrimSpace(companyID), a.Token)
	if err != nil {
		return nil, fmt.Errorf("get role: %w", err)
	}
	return &domain.RoleDetail{
		Role: domain.RoleInfo{
			ID: d.Role.ID, Name: d.Role.Name, CompanyID: d.Role.CompanyID,
			IsCustom: d.Role.IsCustom, Permissions: d.Role.Permissions,
		},
		AllPermissions: d.AllPermissions,
	}, nil
}

func (s *RoleService) Create(a domain.Actor, companyID, name string, permissions []string) (*domain.RoleInfo, error) {
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return nil, &domain.ErrCompanyNotFound{}
	}
	if strings.TrimSpace(name) == "" {
		return nil, &domain.ErrInvalidRole{Msg: "role name is required"}
	}
	r, err := s.sso.CreateRole(context.Background(), companyID, a.Token, strings.TrimSpace(name), dedupeStrings(permissions))
	if err != nil {
		return nil, fmt.Errorf("create role: %w", err)
	}
	return &domain.RoleInfo{ID: r.ID, Name: r.Name, CompanyID: r.CompanyID, IsCustom: r.IsCustom, Permissions: r.Permissions}, nil
}

func (s *RoleService) Update(a domain.Actor, companyID, roleID, name string, permissions []string) (*domain.RoleInfo, error) {
	companyID = strings.TrimSpace(companyID)
	roleID = strings.TrimSpace(roleID)
	if companyID == "" {
		return nil, &domain.ErrCompanyNotFound{}
	}
	if roleID == "" {
		return nil, &domain.ErrInvalidRole{Msg: "role id is required"}
	}
	if err := s.assertCustomRoleOfCompany(a, companyID, roleID); err != nil {
		return nil, err
	}
	r, err := s.sso.UpdateRole(context.Background(), roleID, companyID, a.Token, strings.TrimSpace(name), dedupeStrings(permissions))
	if err != nil {
		return nil, fmt.Errorf("update role: %w", err)
	}
	if s.cache != nil {
		s.cache.InvalidateRoleCache(companyID, roleID)
	}
	return &domain.RoleInfo{ID: r.ID, Name: r.Name, CompanyID: r.CompanyID, IsCustom: r.IsCustom, Permissions: r.Permissions}, nil
}

func (s *RoleService) Delete(a domain.Actor, companyID, roleID string) error {
	companyID = strings.TrimSpace(companyID)
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return &domain.ErrInvalidRole{Msg: "role id is required"}
	}
	if err := s.assertCustomRoleOfCompany(a, companyID, roleID); err != nil {
		return err
	}
	n, err := s.repo.CountActiveByRole(companyID, roleID)
	if err != nil {
		return err
	}
	if n > 0 {
		return &domain.ErrRoleInUse{Count: int(n)}
	}
	if err := s.sso.DeleteRole(context.Background(), roleID, a.Token); err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	if s.cache != nil {
		s.cache.InvalidateRoleCache(companyID, roleID)
	}
	return nil
}

// assertCustomRoleOfCompany refuses edits/deletes on a global role or on a
// custom role that belongs to another company.
func (s *RoleService) assertCustomRoleOfCompany(a domain.Actor, companyID, roleID string) error {
	roles, err := s.sso.ListRoles(context.Background(), companyID, a.Token)
	if err != nil {
		return fmt.Errorf("verify role ownership: %w", err)
	}
	for _, r := range roles {
		if r.ID != roleID {
			continue
		}
		if !r.IsCustom {
			return &domain.ErrInvalidRole{Msg: "a built-in role can't be changed"}
		}
		if r.CompanyID != nil && strings.TrimSpace(*r.CompanyID) != "" && *r.CompanyID != companyID {
			return &domain.ErrInvalidRole{Msg: "role belongs to another company"}
		}
		return nil
	}
	return &domain.ErrMemberNotFound{Ref: roleID}
}
