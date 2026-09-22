package service

import (
	"errors"
	"testing"
	"time"

	membership "duluin_invoice/app/domain/membership"
	"duluin_invoice/app/model"
	"duluin_invoice/app/sso"
)

// userMgmtFixture: caller u1 is an active member of companies c1 and c2 with a role that
// holds every invoice-user permission; each company has an Owner and an Admin role.
func userMgmtFixture(t *testing.T) (*MembershipService, *fakeMembershipRepo, *fakeSSO) {
	t.Helper()
	repo := newFakeMembershipRepo()
	ssoC := newFakeSSO()
	all := []string{"invoice-user-list", "invoice-user-invite", "invoice-user-create", "invoice-user-update", "invoice-user-delete"}
	admin := "role-admin"
	ssoC.rolePerms[admin] = sso.RolePermissions{RoleID: admin, RoleName: "Invoice Admin", Permissions: all}
	for _, cid := range []string{"c1", "c2"} {
		repo.rows = append(repo.rows, &model.UserAccountSSO{
			ID: "m-" + cid, UserID: "u1", CompanyID: cid, RoleID: &admin, IsActivated: true, Email: "u@x.com",
			Company: &model.Company{ID: cid, Name: "Company " + cid, Code: "CODE" + cid},
		})
		ssoC.roles[cid] = []sso.Role{
			{ID: "role-owner", Name: model.SSORoleOwner},
			{ID: admin, Name: "Invoice Admin"},
			{ID: "role-viewer", Name: "Invoice Viewer"},
		}
	}
	ssoC.users = map[string]sso.UserInfo{"new@person.com": {Exists: true, Name: "New Person", Phone: "0899"}}
	return newMembership(repo, ssoC, newFakeCache(), MembershipConfig{UseLocalRBAC: true}), repo, ssoC
}

func TestValidateUser_NewThenAlreadyMember(t *testing.T) {
	svc, repo, ssoC := userMgmtFixture(t)
	_ = ssoC
	a := mActor("u1")

	res, err := svc.ValidateUser(a, "Brand@New.com")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "new_user" || !res.CanInvite || len(res.MemberCompanyIDs) != 0 {
		t.Fatalf("new user: %+v", res)
	}

	// pending in another company only → still invitable here
	now := time.Now()
	viewer := "role-viewer"
	repo.rows = append(repo.rows, &model.UserAccountSSO{ID: "p2", CompanyID: "c2", Email: "new@person.com", RoleID: &viewer, InvitedAt: &now})
	res, _ = svc.ValidateUser(a, "new@person.com")
	if res.Status != "existing_user" || !res.CanInvite || res.HasPendingInvite ||
		res.UserName != "New Person" || res.UserPhone != "0899" {
		t.Fatalf("member elsewhere only: %+v", res)
	}

	// pending in the active company → blocked, no duplicate
	repo.rows = append(repo.rows, &model.UserAccountSSO{ID: "p1", CompanyID: "c1", Email: "new@person.com", RoleID: &viewer, InvitedAt: &now})
	res, _ = svc.ValidateUser(a, "new@person.com")
	if res.Status != "already_member" || res.CanInvite || !res.HasPendingInvite {
		t.Fatalf("pending here: %+v", res)
	}

	// inviting yourself is never allowed
	res, _ = svc.ValidateUser(a, "u@x.com")
	if res.CanInvite {
		t.Fatalf("self must not be invitable: %+v", res)
	}
}

func TestInviteMulti_PerCompanyRolesAndGuards(t *testing.T) {
	svc, repo, ssoC := userMgmtFixture(t)
	_ = ssoC
	a := mActor("u1")

	views, err := svc.InviteMulti(a, membership.InviteMultiInput{
		Email: "Bob@Corp.com", Name: "Bob", Phone: "0812", Active: true, SendEmail: false,
		Grants: []membership.CompanyAssignment{{CompanyID: "c1", RoleID: "role-viewer"}, {CompanyID: "c2", RoleID: "role-admin"}},
	})
	if err != nil || len(views) != 2 {
		t.Fatalf("invite: %v %v", err, views)
	}
	if views[0].Status != membership.StatusPending || views[0].Email != "bob@corp.com" {
		t.Fatalf("view: %+v", views[0])
	}

	// duplicate → conflict, nothing extra written
	before := len(repo.rows)
	_, err = svc.InviteMulti(a, membership.InviteMultiInput{
		Email: "bob@corp.com", Name: "Bob", Active: true, Grants: []membership.CompanyAssignment{{CompanyID: "c1", RoleID: "role-viewer"}},
	})
	var dup *membership.ErrDuplicateInvite
	if !errors.As(err, &dup) || len(repo.rows) != before {
		t.Fatalf("want duplicate, got %v", err)
	}

	// unknown company and missing role are refused
	for name, g := range map[string]membership.CompanyAssignment{
		"foreign": {CompanyID: "zzz", RoleID: "role-viewer"},
		"norole":  {CompanyID: "c2", RoleID: ""},
	} {
		if _, err := svc.InviteMulti(a, membership.InviteMultiInput{Email: "x@y.com", Name: "X", Active: true, Grants: []membership.CompanyAssignment{g}}); err == nil {
			t.Fatalf("%s grant must be refused", name)
		}
	}
}

func TestSetMemberActive_BlocksSelfAndLastOwner(t *testing.T) {
	svc, repo, ssoC := userMgmtFixture(t)
	_ = ssoC
	a := mActor("u1")
	if _, err := svc.SetMemberActive(a, "c1", "m-c1", false); err == nil {
		t.Fatal("deactivating yourself must be refused")
	}

	viewer := "role-viewer"
	repo.rows = append(repo.rows, &model.UserAccountSSO{ID: "v1", UserID: "u9", CompanyID: "c1", RoleID: &viewer, IsActivated: true, Email: "v@x.com"})
	v, err := svc.SetMemberActive(a, "c1", "v1", false)
	if err != nil || v.Status != membership.StatusInactive {
		t.Fatalf("deactivate: %v %+v", err, v)
	}
	v, err = svc.SetMemberActive(a, "c1", "v1", true)
	if err != nil || v.Status != membership.StatusActive {
		t.Fatalf("activate: %v %+v", err, v)
	}
}

func TestValidateUser_SSODownIsNotNewUser(t *testing.T) {
	svc, _, ssoC := userMgmtFixture(t)
	ssoC.validateErr = errors.New("boom")
	_, err := svc.ValidateUser(mActor("u1"), "someone@else.com")
	var down *membership.ErrSSOUnavailable
	if !errors.As(err, &down) {
		t.Fatalf("want ErrSSOUnavailable, got %v", err)
	}
}
