package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	membership "duluin_invoice/app/domain/membership"
	"duluin_invoice/app/model"
	"duluin_invoice/app/sso"
)

// ── fake onboarding repo ───────────────────────────────────────────────────

type fakeOnboardingRepo struct {
	companies map[string]*model.Company
	calls     map[string]int
}

func newFakeOnboardingRepo() *fakeOnboardingRepo {
	return &fakeOnboardingRepo{companies: map[string]*model.Company{}, calls: map[string]int{}}
}

func (f *fakeOnboardingRepo) hit(name string) { f.calls[name]++ }

func (f *fakeOnboardingRepo) FindCompanyByID(id string) (*model.Company, error) {
	f.hit("FindCompanyByID")
	return f.companies[strings.TrimSpace(id)], nil
}

func (f *fakeOnboardingRepo) FindByID(id string) (*model.Company, error) {
	return f.FindCompanyByID(id)
}

func (f *fakeOnboardingRepo) CreateCompany(c *model.Company) error {
	f.hit("CreateCompany")
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	c.CreatedAt = time.Now()
	f.companies[c.ID] = c
	return nil
}

func (f *fakeOnboardingRepo) HardDeleteCompany(id string) error {
	f.hit("HardDeleteCompany")
	delete(f.companies, strings.TrimSpace(id))
	return nil
}

func (f *fakeOnboardingRepo) CompanyCodeExists(code string) (bool, error) {
	f.hit("CompanyCodeExists")
	for _, c := range f.companies {
		if strings.EqualFold(c.Code, code) {
			return true, nil
		}
	}
	return false, nil
}

// ── fake membership port (for onboarding tests) ────────────────────────────

type fakeMembershipPort struct {
	linkCalls  []string // companyID
	invites    []membership.InviteInput
	failLink   bool
	failInvite error
}

func newFakeMembershipPort() *fakeMembershipPort {
	return &fakeMembershipPort{}
}

func (f *fakeMembershipPort) LinkCompanyCreator(a membership.Actor, companyID string) error {
	if f.failLink {
		return errors.New("link failed")
	}
	f.linkCalls = append(f.linkCalls, companyID)
	return nil
}

func (f *fakeMembershipPort) Invite(a membership.Actor, companyID string, in membership.InviteInput) (*model.UserAccountSSO, error) {
	if f.failInvite != nil {
		return nil, f.failInvite
	}
	f.invites = append(f.invites, in)
	return &model.UserAccountSSO{ID: uuid.NewString(), CompanyID: companyID, Email: in.Email, Name: in.Name, RoleID: &in.RoleID}, nil
}

// ── fake membership repo ──────────────────────────────────────────────────

type fakeMembershipRepo struct {
	rows  []*model.UserAccountSSO
	calls map[string]int
}

func newFakeMembershipRepo() *fakeMembershipRepo {
	return &fakeMembershipRepo{calls: map[string]int{}}
}

func (f *fakeMembershipRepo) hit(n string) { f.calls[n]++ }

func (f *fakeMembershipRepo) FindByUserAndCompany(userID, companyID string) (*model.UserAccountSSO, error) {
	f.hit("FindByUserAndCompany")
	for _, r := range f.rows {
		if r.DeletedAt.Valid {
			continue
		}
		if r.UserID == userID && r.CompanyID == companyID {
			return r, nil
		}
	}
	return nil, nil
}

func (f *fakeMembershipRepo) ListActiveByUser(userID string) ([]model.UserAccountSSO, error) {
	f.hit("ListActiveByUser")
	var out []model.UserAccountSSO
	for _, r := range f.rows {
		if !r.DeletedAt.Valid && r.UserID == userID && r.IsActivated && !r.IsBanned {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (f *fakeMembershipRepo) DefaultCompanyID(userID string) (string, error) {
	f.hit("DefaultCompanyID")
	for _, r := range f.rows {
		if !r.DeletedAt.Valid && r.UserID == userID && r.IsActivated && !r.IsBanned {
			return r.CompanyID, nil
		}
	}
	return "", nil
}

func (f *fakeMembershipRepo) ListByCompany(flt membership.MemberFilter) ([]model.UserAccountSSO, int64, error) {
	f.hit("ListByCompany")
	var out []model.UserAccountSSO
	for _, r := range f.rows {
		if !r.DeletedAt.Valid && r.CompanyID == flt.CompanyID {
			out = append(out, *r)
		}
	}
	return out, int64(len(out)), nil
}

func (f *fakeMembershipRepo) FindByIDInCompany(companyID, id string) (*model.UserAccountSSO, error) {
	f.hit("FindByIDInCompany")
	for _, r := range f.rows {
		if !r.DeletedAt.Valid && r.CompanyID == companyID && r.ID == id {
			return r, nil
		}
	}
	return nil, nil
}

func (f *fakeMembershipRepo) FindActiveMemberByEmail(companyID, email string) (*model.UserAccountSSO, error) {
	f.hit("FindActiveMemberByEmail")
	for _, r := range f.rows {
		if !r.DeletedAt.Valid && r.CompanyID == companyID && strings.EqualFold(r.Email, email) && r.IsActivated {
			return r, nil
		}
	}
	return nil, nil
}

func (f *fakeMembershipRepo) FindPendingInviteByEmail(companyID, email string) (*model.UserAccountSSO, error) {
	f.hit("FindPendingInviteByEmail")
	for _, r := range f.rows {
		if !r.DeletedAt.Valid && r.CompanyID == companyID && strings.EqualFold(r.Email, email) && !r.IsActivated && r.InvitedAt != nil {
			return r, nil
		}
	}
	return nil, nil
}

func (f *fakeMembershipRepo) FindInviteByToken(token string) (*model.UserAccountSSO, error) {
	f.hit("FindInviteByToken")
	for _, r := range f.rows {
		if !r.DeletedAt.Valid && r.InviteToken != nil && *r.InviteToken == token {
			return r, nil
		}
	}
	return nil, nil
}

func (f *fakeMembershipRepo) Upsert(m *model.UserAccountSSO) error {
	f.hit("Upsert")
	for _, r := range f.rows {
		if r.DeletedAt.Valid {
			continue
		}
		if (m.UserID != "" && r.UserID == m.UserID && r.CompanyID == m.CompanyID) ||
			(m.UserID == "" && r.CompanyID == m.CompanyID && strings.EqualFold(r.Email, m.Email)) {
			r.RoleID, r.SecondaryID, r.IsActivated = m.RoleID, m.SecondaryID, m.IsActivated
			r.InviteToken, r.InvitedAt, r.InvitedBy = m.InviteToken, m.InvitedAt, m.InvitedBy
			r.Name, r.ActivatedAt = m.Name, m.ActivatedAt
			m.ID = r.ID
			return nil
		}
	}
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	cp := *m
	f.rows = append(f.rows, &cp)
	return nil
}

func (f *fakeMembershipRepo) Save(m *model.UserAccountSSO) error {
	f.hit("Save")
	for i, r := range f.rows {
		if r.ID == m.ID {
			cp := *m
			f.rows[i] = &cp
			return nil
		}
	}
	return errors.New("row not found")
}

func (f *fakeMembershipRepo) SoftDeleteByID(id string) error {
	f.hit("SoftDeleteByID")
	for _, r := range f.rows {
		if r.ID == id {
			r.DeletedAt.Valid = true
			r.DeletedAt.Time = time.Now()
		}
	}
	return nil
}

func (f *fakeMembershipRepo) CountActiveByRole(companyID, roleID string) (int64, error) {
	f.hit("CountActiveByRole")
	var n int64
	for _, r := range f.rows {
		if !r.DeletedAt.Valid && r.CompanyID == companyID && r.IsActivated && r.RoleID != nil && *r.RoleID == roleID {
			n++
		}
	}
	return n, nil
}

func (f *fakeMembershipRepo) CountConsumedInviteSlots(companyID, ownerRoleID string) (int64, error) {
	f.hit("CountConsumedInviteSlots")
	var n int64
	for _, r := range f.rows {
		if r.DeletedAt.Valid || r.CompanyID != companyID {
			continue
		}
		if ownerRoleID != "" && r.RoleID != nil && *r.RoleID == ownerRoleID {
			continue
		}
		n++
	}
	return n, nil
}

// ── fake sso client ───────────────────────────────────────────────────────

type fakeSSO struct {
	roles          map[string][]sso.Role // key: companyID ("" = global)
	rolePerms      map[string]sso.RolePermissions
	secondaryCalls []string
	assignCalls    []string
	getPermsCalls  int
	failGetPerms   error
	tempThenOK     bool
	users          map[string]sso.UserInfo // SSO user-validation by lower-case email
	validateErr    error
}

func newFakeSSO() *fakeSSO {
	return &fakeSSO{roles: map[string][]sso.Role{}, rolePerms: map[string]sso.RolePermissions{}}
}

func (f *fakeSSO) GetRolePermissions(_ context.Context, roleID, companyID, _ string) (sso.RolePermissions, error) {
	f.getPermsCalls++
	if f.tempThenOK && f.getPermsCalls == 1 {
		return sso.RolePermissions{}, &sso.Error{Path: "x", Status: 503, Temporary: true}
	}
	if f.failGetPerms != nil {
		return sso.RolePermissions{}, f.failGetPerms
	}
	if rp, ok := f.rolePerms[roleID]; ok {
		return rp, nil
	}
	return sso.RolePermissions{RoleID: roleID, RoleName: "Role", Permissions: []string{"invoice-mitra-list"}}, nil
}

func (f *fakeSSO) ListRoles(_ context.Context, companyID, _ string) ([]sso.Role, error) {
	return f.roles[companyID], nil
}

func (f *fakeSSO) GetRoleDetail(_ context.Context, roleID, companyID, _ string) (sso.RoleDetail, error) {
	return sso.RoleDetail{Role: sso.Role{ID: roleID}, AllPermissions: []string{"invoice-mitra-list"}}, nil
}

func (f *fakeSSO) CreateRole(_ context.Context, companyID, _, name string, perms []string) (sso.Role, error) {
	cid := companyID
	return sso.Role{ID: uuid.NewString(), Name: name, CompanyID: &cid, IsCustom: true, Permissions: perms}, nil
}

func (f *fakeSSO) UpdateRole(_ context.Context, roleID, companyID, _, name string, perms []string) (sso.Role, error) {
	cid := companyID
	return sso.Role{ID: roleID, Name: name, CompanyID: &cid, IsCustom: true, Permissions: perms}, nil
}

func (f *fakeSSO) DeleteRole(_ context.Context, roleID, _ string) error { return nil }

func (f *fakeSSO) SetSecondaryID(_ context.Context, userID, secondaryID string) error {
	f.secondaryCalls = append(f.secondaryCalls, userID+":"+secondaryID)
	return nil
}

func (f *fakeSSO) AssignRole(_ context.Context, userID, roleName string) error {
	f.assignCalls = append(f.assignCalls, userID+":"+roleName)
	return nil
}

// ── fake cache ────────────────────────────────────────────────────────────

type fakeCache struct {
	data map[string][]byte
}

func newFakeCache() *fakeCache { return &fakeCache{data: map[string][]byte{}} }

func (f *fakeCache) GetJSON(_ context.Context, key string, dst any) bool {
	raw, ok := f.data[key]
	if !ok {
		return false
	}
	return json.Unmarshal(raw, dst) == nil
}

func (f *fakeCache) SetJSON(_ context.Context, key string, val any, _ time.Duration) {
	if raw, err := json.Marshal(val); err == nil {
		f.data[key] = raw
	}
}

func (f *fakeCache) Del(_ context.Context, keys ...string) {
	for _, k := range keys {
		delete(f.data, k)
	}
}

func (f *fakeCache) DelByPattern(_ context.Context, pattern string) {
	prefix := strings.TrimSuffix(pattern, "*")
	for k := range f.data {
		if strings.HasPrefix(k, prefix) {
			delete(f.data, k)
		}
	}
}

func (f *fakeMembershipRepo) ListByEmailInCompanies(emails, companyIDs []string) ([]model.UserAccountSSO, error) {
	f.hit("ListByEmailInCompanies")
	var out []model.UserAccountSSO
	for _, r := range f.rows {
		if r.DeletedAt.Valid {
			continue
		}
		for _, e := range emails {
			for _, c := range companyIDs {
				if strings.EqualFold(r.Email, e) && r.CompanyID == c {
					out = append(out, *r)
				}
			}
		}
	}
	return out, nil
}

func (f *fakeMembershipRepo) FindAnyByEmail(email string) (*model.UserAccountSSO, error) {
	f.hit("FindAnyByEmail")
	for _, r := range f.rows {
		if !r.DeletedAt.Valid && strings.EqualFold(r.Email, email) {
			return r, nil
		}
	}
	return nil, nil
}

func (f *fakeSSO) ValidateUser(_ context.Context, email string) (sso.UserInfo, error) {
	if f.validateErr != nil {
		return sso.UserInfo{}, f.validateErr
	}
	return f.users[strings.ToLower(email)], nil
}
