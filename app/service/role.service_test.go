package service

import (
	"errors"
	"testing"

	membership "duluin_invoice/app/domain/membership"
	"duluin_invoice/app/model"
	"duluin_invoice/app/sso"
)

func newRole(repo *fakeMembershipRepo, ssoC *fakeSSO) *RoleService {
	return NewRoleService(ssoC, repo, nil)
}

func TestRoleService_CreateAndList(t *testing.T) {
	ssoC := newFakeSSO()
	svc := newRole(newFakeMembershipRepo(), ssoC)
	a := membership.Actor{Token: "Bearer t"}

	r, err := svc.Create(a, "c1", "Kasir", []string{"invoice-sales-invoice-list"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !r.IsCustom || r.CompanyID == nil || *r.CompanyID != "c1" {
		t.Fatalf("created role wrong: %+v", r)
	}
}

func TestRoleService_DeleteBlockedWhenInUse(t *testing.T) {
	repo := newFakeMembershipRepo()
	rid := "role-x"
	repo.rows = append(repo.rows, &model.UserAccountSSO{ID: "m1", CompanyID: "c1", RoleID: &rid, IsActivated: true})
	ssoC := newFakeSSO()
	cid := "c1"
	ssoC.roles["c1"] = []sso.Role{{ID: "role-x", Name: "X", CompanyID: &cid, IsCustom: true}}
	svc := newRole(repo, ssoC)

	err := svc.Delete(membership.Actor{Token: "t"}, "c1", "role-x")
	var inUse *membership.ErrRoleInUse
	if !errors.As(err, &inUse) {
		t.Fatalf("want ErrRoleInUse, got %v", err)
	}
}

func TestRoleService_DeleteRejectsGlobalRole(t *testing.T) {
	ssoC := newFakeSSO()
	ssoC.roles["c1"] = []sso.Role{{ID: "owner-role", Name: "Invoice Owner"}}
	svc := newRole(newFakeMembershipRepo(), ssoC)

	err := svc.Delete(membership.Actor{Token: "t"}, "c1", "owner-role")
	var invalid *membership.ErrInvalidRole
	if !errors.As(err, &invalid) {
		t.Fatalf("want ErrInvalidRole for global role, got %v", err)
	}
}
