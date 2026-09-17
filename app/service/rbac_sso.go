package service

import (
	"context"

	"duluin_invoice/app/sso"
)

// RBACSSOClient is the subset of *sso.Client used by the membership and role
// services. Declared here (consumer side) so both services are unit-testable
// with a fake and never import net/http.
type RBACSSOClient interface {
	GetRolePermissions(ctx context.Context, roleID, companyID, token string) (sso.RolePermissions, error)
	ListRoles(ctx context.Context, companyID, token string) ([]sso.Role, error)
	GetRoleDetail(ctx context.Context, roleID, companyID, token string) (sso.RoleDetail, error)
	CreateRole(ctx context.Context, companyID, token, name string, permissionNames []string) (sso.Role, error)
	UpdateRole(ctx context.Context, roleID, companyID, token, name string, permissionNames []string) (sso.Role, error)
	DeleteRole(ctx context.Context, roleID, token string) error
	SetSecondaryID(ctx context.Context, ssoUserID, secondaryID string) error
	AssignRole(ctx context.Context, ssoUserID, roleName string) error
}

// ssoIsTemporary reports whether an SSO error is worth one retry.
func ssoIsTemporary(err error) bool { return sso.IsTemporary(err) }
