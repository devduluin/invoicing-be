package service

import (
	"errors"
	"testing"

	membership "duluin_invoice/app/domain/membership"
	"duluin_invoice/app/model"
	"duluin_invoice/app/notification"
	"duluin_invoice/app/sso"
)

func newMembership(repo *fakeMembershipRepo, ssoC *fakeSSO, cache *fakeCache, cfg MembershipConfig) *MembershipService {
	return NewMembershipService(repo, newFakeOnboardingRepo(), ssoC, cache, notification.NewLogService(), cfg, "http://web.test")
}

func mActor(userID string) membership.Actor {
	return membership.Actor{UserID: userID, Email: "u@x.com", Name: "U", ActiveCompanyID: "c1", Token: "Bearer t"}
}

func TestResolveAccess_NoMembership(t *testing.T) {
	svc := newMembership(newFakeMembershipRepo(), newFakeSSO(), newFakeCache(), MembershipConfig{UseLocalRBAC: true})
	res, err := svc.ResolveAccess(mActor("u1"), "c1", nil, nil)
	if err != nil {
		t.Fatalf("ResolveAccess: %v", err)
	}
	if res.HasAccess || res.IsUnresolved() {
		t.Fatalf("want no-access resolved, got %+v", res)
	}
}

func TestResolveAccess_RoleIDResolvesAndCaches(t *testing.T) {
	repo := newFakeMembershipRepo()
	rid := "role-1"
	repo.rows = append(repo.rows, &model.UserAccountSSO{
		ID: "m1", UserID: "u1", CompanyID: "c1", RoleID: &rid, IsActivated: true,
	})
	ssoC := newFakeSSO()
	ssoC.rolePerms[rid] = sso.RolePermissions{RoleID: rid, RoleName: "Kasir", Permissions: []string{"invoice-mitra-list", "invoice-mitra-list"}}
	svc := newMembership(repo, ssoC, newFakeCache(), MembershipConfig{UseLocalRBAC: true})

	res, err := svc.ResolveAccess(mActor("u1"), "c1", nil, nil)
	if err != nil {
		t.Fatalf("ResolveAccess: %v", err)
	}
	if !res.HasAccess || !res.IsActivated || len(res.Permissions) != 1 || res.Permissions[0] != "invoice-mitra-list" {
		t.Fatalf("bad resolution: %+v", res)
	}

	repo.calls = map[string]int{}
	ssoC.getPermsCalls = 0
	if _, err := svc.ResolveAccess(mActor("u1"), "c1", nil, nil); err != nil {
		t.Fatalf("second ResolveAccess: %v", err)
	}
	if repo.calls["FindByUserAndCompany"] != 0 || ssoC.getPermsCalls != 0 {
		t.Fatalf("second call must hit access cache, got repo=%d sso=%d", repo.calls["FindByUserAndCompany"], ssoC.getPermsCalls)
	}
}

func TestResolveAccess_TemporarySSOErrorRetried(t *testing.T) {
	repo := newFakeMembershipRepo()
	rid := "role-1"
	repo.rows = append(repo.rows, &model.UserAccountSSO{ID: "m1", UserID: "u1", CompanyID: "c1", RoleID: &rid, IsActivated: true})
	ssoC := newFakeSSO()
	ssoC.tempThenOK = true
	svc := newMembership(repo, ssoC, newFakeCache(), MembershipConfig{UseLocalRBAC: true})

	res, err := svc.ResolveAccess(mActor("u1"), "c1", nil, nil)
	if err != nil || res.IsUnresolved() {
		t.Fatalf("temporary error should be retried: %v %+v", err, res)
	}
	if ssoC.getPermsCalls != 2 {
		t.Fatalf("want exactly 1 retry (2 calls), got %d", ssoC.getPermsCalls)
	}
}

func TestResolveAccess_FallbackWhenRoleUnset(t *testing.T) {
	repo := newFakeMembershipRepo()
	repo.rows = append(repo.rows, &model.UserAccountSSO{ID: "m1", UserID: "u1", CompanyID: "c1", IsActivated: true})
	svc := newMembership(repo, newFakeSSO(), newFakeCache(), MembershipConfig{UseLocalRBAC: true, RBACMigrationFallback: true})

	res, err := svc.ResolveAccess(mActor("u1"), "c1", []string{"Invoice Owner"}, []string{"invoice-mitra-create"})
	if err != nil {
		t.Fatalf("ResolveAccess: %v", err)
	}
	if !res.UsedSSOFallback || len(res.Permissions) != 1 || res.Permissions[0] != "invoice-mitra-create" {
		t.Fatalf("want SSO fallback perms, got %+v", res)
	}
}

func TestLinkCompanyCreator_ResolvesOwnerAndSyncs(t *testing.T) {
	repo := newFakeMembershipRepo()
	ssoC := newFakeSSO()
	ssoC.roles[""] = []sso.Role{{ID: "owner-role", Name: "Invoice Owner"}}
	svc := newMembership(repo, ssoC, newFakeCache(), MembershipConfig{UseLocalRBAC: true})

	if err := svc.LinkCompanyCreator(mActor("u1"), "c1"); err != nil {
		t.Fatalf("LinkCompanyCreator: %v", err)
	}
	m, _ := repo.FindByUserAndCompany("u1", "c1")
	if m == nil || m.RoleID == nil || *m.RoleID != "owner-role" || !m.IsActivated {
		t.Fatalf("owner membership wrong: %+v", m)
	}
	// secondary_id mirrors the user's own id (SSO identity link), not the company.
	if len(ssoC.secondaryCalls) != 1 || ssoC.secondaryCalls[0] != "u1:u1" {
		t.Fatalf("secondary_id should be user id, got: %v", ssoC.secondaryCalls)
	}
	if m.SecondaryID == nil || *m.SecondaryID != "u1" {
		t.Fatalf("local secondary_id should be user id, got %v", m.SecondaryID)
	}
	if len(ssoC.assignCalls) != 1 || ssoC.assignCalls[0] != "u1:Invoice Owner" {
		t.Fatalf("owner role not assigned: %v", ssoC.assignCalls)
	}
}

func TestInvite_Guards(t *testing.T) {
	repo := newFakeMembershipRepo()
	ssoC := newFakeSSO()
	ssoC.roles["c1"] = []sso.Role{{ID: "owner-role", Name: "Invoice Owner"}, {ID: "viewer", Name: "Viewer", IsCustom: true}}
	ssoC.roles[""] = []sso.Role{{ID: "owner-role", Name: "Invoice Owner"}}
	svc := newMembership(repo, ssoC, newFakeCache(), MembershipConfig{UseLocalRBAC: true})
	a := mActor("owner")

	// invalid role
	if _, err := svc.Invite(a, "c1", membership.InviteInput{Email: "x@x.com", RoleID: "ghost"}); err == nil {
		t.Fatal("unknown role must be rejected")
	}
	// happy path
	if _, err := svc.Invite(a, "c1", membership.InviteInput{Email: "x@x.com", RoleID: "viewer"}); err != nil {
		t.Fatalf("Invite: %v", err)
	}
	// duplicate pending
	_, err := svc.Invite(a, "c1", membership.InviteInput{Email: "X@x.com", RoleID: "viewer"})
	var dup *membership.ErrDuplicateInvite
	if !errors.As(err, &dup) {
		t.Fatalf("want ErrDuplicateInvite, got %v", err)
	}

	// quota
	for i := 0; i < model.FreeInviteQuota; i++ {
		vid := "viewer"
		repo.rows = append(repo.rows, &model.UserAccountSSO{ID: uuidLike(i), CompanyID: "c1", RoleID: &vid, Email: uuidLike(i) + "@x.com"})
	}
	_, err = svc.Invite(a, "c1", membership.InviteInput{Email: "over@x.com", RoleID: "viewer"})
	var q *membership.ErrInviteQuota
	if !errors.As(err, &q) {
		t.Fatalf("want ErrInviteQuota, got %v", err)
	}
}

func TestUpdateMemberRole_LastOwnerGuard(t *testing.T) {
	repo := newFakeMembershipRepo()
	owner := "owner-role"
	repo.rows = append(repo.rows, &model.UserAccountSSO{ID: "m1", UserID: "u1", CompanyID: "c1", RoleID: &owner, IsActivated: true})
	ssoC := newFakeSSO()
	ssoC.roles["c1"] = []sso.Role{{ID: "owner-role", Name: "Invoice Owner"}, {ID: "viewer", Name: "Viewer", IsCustom: true}}
	ssoC.roles[""] = []sso.Role{{ID: "owner-role", Name: "Invoice Owner"}}
	svc := newMembership(repo, ssoC, newFakeCache(), MembershipConfig{UseLocalRBAC: true})

	_, err := svc.UpdateMemberRole(mActor("u1"), "c1", "m1", "viewer")
	var lo *membership.ErrLastOwner
	if !errors.As(err, &lo) {
		t.Fatalf("want ErrLastOwner, got %v", err)
	}
}

func uuidLike(i int) string { return "row" + string(rune('a'+i)) }
