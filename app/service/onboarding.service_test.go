package service

import (
	"errors"
	"testing"

	onboarding "duluin_invoice/app/domain/onboarding"
	"duluin_invoice/app/model"
)

func onbActor(userID, email string) onboarding.Actor {
	return onboarding.Actor{SSOUserID: userID, Email: email, Name: "Owner"}
}

type fakeSeeder struct{ calls int }

func (f *fakeSeeder) SeedCompanyDefaults(companyID, actorID string) error {
	f.calls++
	return nil
}

func newOnb(repo *fakeOnboardingRepo, port *fakeMembershipPort) *OnboardingService {
	return NewOnboardingService(repo, port, &fakeSeeder{})
}

func submitDTO() onboarding.SubmitDTO {
	return onboarding.SubmitDTO{
		NamaPerusahaan: "Toko Maju",
		TipeAkun:       "perseorangan",
		JenisUsaha:     "Retail",
		JumlahKaryawan: "11-50",
		Telepon:        "0811",
		Email:          "owner@toko.co.id",
		KebutuhanUser:  []string{"invoicing"},
	}
}

func TestSubmit_CreatesActiveCompanyAndLinksOwner(t *testing.T) {
	repo, port := newFakeOnboardingRepo(), newFakeMembershipPort()
	svc := newOnb(repo, port)

	res, err := svc.Submit(onbActor("u1", "o@x.com"), submitDTO())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if res.Company == nil || res.Company.OnboardingStatus != model.OnboardingActive {
		t.Fatalf("company not active: %+v", res.Company)
	}
	if repo.calls["CreateCompany"] != 1 || len(port.linkCalls) != 1 {
		t.Fatalf("want 1 create + 1 link, got create=%d link=%d", repo.calls["CreateCompany"], len(port.linkCalls))
	}
	if len(res.Company.Code) != 6 {
		t.Fatalf("code = %q, want 6 chars", res.Company.Code)
	}
	if repo.calls["CompanyCodeExists"] != 1 {
		t.Fatalf("code uniqueness should be one probe, got %d", repo.calls["CompanyCodeExists"])
	}
}

func TestSubmit_RejectsInvalidPayload(t *testing.T) {
	svc := newOnb(newFakeOnboardingRepo(), newFakeMembershipPort())
	dto := submitDTO()
	dto.JumlahKaryawan = ""
	_, err := svc.Submit(onbActor("u1", "o@x.com"), dto)
	var v *onboarding.ErrValidation
	if !errors.As(err, &v) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

func TestSubmit_RollsBackCompanyWhenLinkFails(t *testing.T) {
	repo, port := newFakeOnboardingRepo(), newFakeMembershipPort()
	port.failLink = true
	svc := newOnb(repo, port)

	if _, err := svc.Submit(onbActor("u1", "o@x.com"), submitDTO()); err == nil {
		t.Fatal("Submit should fail when owner-linking fails")
	}
	if len(repo.companies) != 0 {
		t.Fatalf("company must be rolled back, still have %d", len(repo.companies))
	}
	if repo.calls["HardDeleteCompany"] != 1 {
		t.Fatalf("expected 1 compensating delete, got %d", repo.calls["HardDeleteCompany"])
	}
}

func TestSubmit_SendsInvitesBestEffort(t *testing.T) {
	repo, port := newFakeOnboardingRepo(), newFakeMembershipPort()
	port.failInvite = errors.New("bad role")
	svc := newOnb(repo, port)

	dto := submitDTO()
	dto.Invites = []onboarding.SubmitInvite{
		{Email: "a@x.com", RoleID: "r1"},
		{Email: "a@x.com", RoleID: "r1"}, // duplicate, skipped
		{Email: "b@x.com", RoleID: "r1"},
	}
	res, err := svc.Submit(onbActor("u1", "o@x.com"), dto)
	if err != nil {
		t.Fatalf("invite failure must not fail the submit: %v", err)
	}
	if res.Company == nil {
		t.Fatal("company should still be created")
	}
	if len(res.FailedInvites) != 2 || res.InvitesSent != 0 {
		t.Fatalf("both unique invites fail: want 2 failed / 0 sent, got failed=%v sent=%d", res.FailedInvites, res.InvitesSent)
	}
}
