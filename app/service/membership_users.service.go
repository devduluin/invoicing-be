package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	domain_activation "duluin_invoice/app/domain/activation"
	domain "duluin_invoice/app/domain/membership"
	"duluin_invoice/app/model"
	"duluin_invoice/app/sso"
)

// Permission slugs (SSO DuluinInvoiceSeeder).
const (
	permUserList   = "invoice-user-list"
	permUserInvite = "invoice-user-invite"
	permUserCreate = "invoice-user-create"
	permUserUpdate = "invoice-user-update"
	permUserDelete = "invoice-user-delete"
)

// ssoUserChecker is the optional SSO capability used to tell "already has a
// Duluin account" from "brand new". The concrete client has it; test fakes need not.
type ssoUserChecker interface {
	ValidateUser(ctx context.Context, email string) (sso.UserInfo, error)
}

func memberStatus(m *model.UserAccountSSO) string {
	switch {
	case m.IsBanned:
		return domain.StatusInactive
	case m.IsActivated:
		return domain.StatusActive
	default:
		return domain.StatusPending
	}
}

// can reports whether the actor holds any of perms in the company (fail closed).
func (s *MembershipService) can(a domain.Actor, companyID string, perms ...string) bool {
	res, err := s.ResolveAccess(a, companyID, nil, nil)
	if err != nil || res == nil || !res.HasAccess || !res.IsActivated || res.IsBanned || res.IsUnresolved() {
		return false
	}
	for _, have := range res.Permissions {
		for _, want := range perms {
			if have == want {
				return true
			}
		}
	}
	return false
}

// ManageableCompanies returns the actor's own companies where they hold perm.
func (s *MembershipService) ManageableCompanies(a domain.Actor, perm string) ([]domain.ManageableCompany, error) {
	m, err := s.manageableIDs(a, perm)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.ListActiveByUser(a.UserID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ManageableCompany, 0, len(m))
	for i := range rows {
		if c, ok := m[rows[i].CompanyID]; ok {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *MembershipService) manageableIDs(a domain.Actor, perms ...string) (map[string]domain.ManageableCompany, error) {
	rows, err := s.repo.ListActiveByUser(a.UserID)
	if err != nil {
		return nil, err
	}
	out := map[string]domain.ManageableCompany{}
	for i := range rows {
		c := rows[i].Company
		if c != nil && s.can(a, rows[i].CompanyID, perms...) {
			out[c.ID] = domain.ManageableCompany{ID: c.ID, Name: c.Name, Code: c.Code}
		}
	}
	return out, nil
}

func idsOf(m map[string]domain.ManageableCompany) []string {
	out := make([]string, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	return out
}

// ── validate ───────────────────────────────────────────────────────────────

func (s *MembershipService) ValidateUser(a domain.Actor, email string) (*domain.ValidateResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, &domain.ErrInvalidRole{Msg: "email is required"}
	}
	res := &domain.ValidateResult{MemberCompanyIDs: []string{}}
	if email == strings.ToLower(strings.TrimSpace(a.Email)) {
		res.Status = "already_member"
		res.ExistsInSSO = true
		return res, nil
	}

	companies, err := s.manageableIDs(a, permUserInvite, permUserCreate)
	if err != nil {
		return nil, err
	}
	// The invite goes to the company the caller is logged into, so that is the only one that counts.
	if active := strings.TrimSpace(a.ActiveCompanyID); active != "" {
		only := map[string]domain.ManageableCompany{}
		if co, ok := companies[active]; ok {
			only[active] = co
		}
		companies = only
	}
	rows, err := s.repo.ListByEmailInCompanies([]string{email}, idsOf(companies))
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for i := range rows {
		if !seen[rows[i].CompanyID] {
			seen[rows[i].CompanyID] = true
			res.MemberCompanyIDs = append(res.MemberCompanyIDs, rows[i].CompanyID)
		}
		if rows[i].Pending() {
			res.HasPendingInvite = true
		}
	}
	// SSO is the source of truth for "is this person registered" and their name/phone.
	if chk, ok := s.sso.(ssoUserChecker); ok {
		info, err := chk.ValidateUser(context.Background(), email)
		if err != nil {
			return nil, &domain.ErrSSOUnavailable{}
		}
		res.ExistsInSSO = info.Exists
		res.UserName, res.UserPhone = info.Name, info.Phone
	}
	// Not in SSO (or SSO not consulted): fall back to what an earlier invite recorded locally.
	if res.UserName == "" || res.UserPhone == "" {
		if known, err := s.repo.FindAnyByEmail(email); err == nil && known != nil {
			if res.UserName == "" {
				res.UserName = known.Name
			}
			if res.UserPhone == "" {
				res.UserPhone = known.Phone
			}
		}
	}

	switch {
	case len(companies) > 0 && len(seen) >= len(companies):
		res.Status = "already_member"
	case res.ExistsInSSO:
		res.Status = "existing_user"
	default:
		res.Status = "new_user"
	}
	res.CanInvite = res.Status != "already_member"
	return res, nil
}

// ── multi-company invite ───────────────────────────────────────────────────

func (s *MembershipService) InviteMulti(a domain.Actor, in domain.InviteMultiInput) ([]domain.MemberView, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email == "" {
		return nil, &domain.ErrInvalidRole{Msg: "email is required"}
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, &domain.ErrInvalidRole{Msg: "name is required"}
	}
	if email == strings.ToLower(strings.TrimSpace(a.Email)) {
		return nil, &domain.ErrInvalidRole{Msg: "can't invite yourself"}
	}
	if len(in.Grants) == 0 {
		return nil, &domain.ErrInvalidRole{Msg: "select at least one company"}
	}
	allowed, err := s.manageableIDs(a, permUserInvite, permUserCreate)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	type plan struct {
		grant domain.CompanyAssignment
		names map[string]string
	}
	var plans []plan
	seen := map[string]bool{}
	for _, g := range in.Grants {
		g.CompanyID, g.RoleID = strings.TrimSpace(g.CompanyID), strings.TrimSpace(g.RoleID)
		if seen[g.CompanyID] {
			continue
		}
		seen[g.CompanyID] = true
		co, ok := allowed[g.CompanyID]
		if !ok {
			return nil, &domain.ErrNoMembership{}
		}
		if g.RoleID == "" {
			return nil, &domain.ErrInvalidRole{Msg: "select a role for " + co.Name}
		}
		roles := s.listRolesQuiet(ctx, g.CompanyID, a.Token)
		if !roleExists(roles, g.RoleID) {
			return nil, &domain.ErrInvalidRole{Msg: "role not recognized for " + co.Name}
		}
		owner := s.ownerRoleIDFromList(roles)
		if ex, err := s.repo.FindActiveMemberByEmail(g.CompanyID, email); err != nil {
			return nil, err
		} else if ex != nil {
			return nil, &domain.ErrDuplicateInvite{Email: email}
		}
		if pe, err := s.repo.FindPendingInviteByEmail(g.CompanyID, email); err != nil {
			return nil, err
		} else if pe != nil {
			return nil, &domain.ErrDuplicateInvite{Email: email}
		}
		used, err := s.repo.CountConsumedInviteSlots(g.CompanyID, owner)
		if err != nil {
			return nil, err
		}
		limit := s.userLimit(g.CompanyID)
		if domain_activation.Reached(used, limit) {
			return nil, &domain.ErrInviteQuota{Limit: limit}
		}
		plans = append(plans, plan{grant: g, names: roleNameIndex(roles)})
	}

	now := time.Now()
	inviter := strings.TrimSpace(a.UserID)
	out := make([]domain.MemberView, 0, len(plans))
	for _, p := range plans {
		tok := newInviteToken()
		roleID := p.grant.RoleID
		m := &model.UserAccountSSO{
			CompanyID:   p.grant.CompanyID,
			RoleID:      &roleID,
			Email:       email,
			Name:        strings.TrimSpace(in.Name),
			Phone:       strings.TrimSpace(in.Phone),
			IsActivated: false,
			IsBanned:    !in.Active,
			InviteToken: &tok,
			InvitedBy:   &inviter,
			InvitedAt:   &now,
		}
		if !in.Active {
			m.BannedReason = "deactivated"
		}
		if err := s.repo.Upsert(m); err != nil {
			return nil, fmt.Errorf("invite: %w", err)
		}
		v := toMemberView(m, p.names)
		if in.SendEmail {
			if url := s.sendInviteEmail(ctx, m, a); s.cfg.ExposeInviteURL {
				v.InviteURL = url
			}
		}
		out = append(out, v)
	}
	return out, nil
}

// ── detail / assignments ───────────────────────────────────────────────────

func (s *MembershipService) loadMember(companyID, id string) (*model.UserAccountSSO, error) {
	m, err := s.repo.FindByIDInCompany(strings.TrimSpace(companyID), id)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, &domain.ErrMemberNotFound{Ref: id}
	}
	return m, nil
}

// companiesByEmail lists memberships of the given emails in the companies the
// actor may see, grouped by lower-cased email.
func (s *MembershipService) companiesByEmail(a domain.Actor, emails []string, visible map[string]domain.ManageableCompany) map[string][]domain.MemberCompany {
	out := map[string][]domain.MemberCompany{}
	if len(visible) == 0 || len(emails) == 0 {
		return out
	}
	rows, err := s.repo.ListByEmailInCompanies(emails, idsOf(visible))
	if err != nil {
		return out
	}
	ctx := context.Background()
	nameCache := map[string]map[string]string{}
	for i := range rows {
		r := &rows[i]
		names, ok := nameCache[r.CompanyID]
		if !ok {
			names = roleNameIndex(s.listRolesQuiet(ctx, r.CompanyID, a.Token))
			nameCache[r.CompanyID] = names
		}
		co := visible[r.CompanyID]
		key := strings.ToLower(r.Email)
		out[key] = append(out[key], domain.MemberCompany{
			MemberID: r.ID, CompanyID: r.CompanyID, CompanyName: co.Name, CompanyCode: co.Code,
			RoleID: strval(r.RoleID), RoleName: names[strval(r.RoleID)], Status: memberStatus(r),
		})
	}
	return out
}

func (s *MembershipService) attachCompanies(a domain.Actor, views []domain.MemberView) {
	if len(views) == 0 {
		return
	}
	visible, err := s.manageableIDs(a, permUserList)
	if err != nil {
		return
	}
	emails := make([]string, 0, len(views))
	for _, v := range views {
		emails = append(emails, v.Email)
	}
	byEmail := s.companiesByEmail(a, emails, visible)
	for i := range views {
		views[i].Companies = byEmail[strings.ToLower(views[i].Email)]
	}
}

func (s *MembershipService) GetMember(a domain.Actor, companyID, id string) (*domain.MemberDetail, error) {
	m, err := s.loadMember(companyID, id)
	if err != nil {
		return nil, err
	}
	names := roleNameIndex(s.listRolesQuiet(context.Background(), m.CompanyID, a.Token))
	d := &domain.MemberDetail{MemberView: toMemberView(m, names)}
	if m.ActivatedAt != nil {
		t := m.ActivatedAt.UTC().Format(time.RFC3339)
		d.AcceptedAt = &t
	}
	visible, err := s.manageableIDs(a, permUserList)
	if err != nil {
		return nil, err
	}
	d.Companies = s.companiesByEmail(a, []string{m.Email}, visible)[strings.ToLower(m.Email)]
	if d.Companies == nil {
		d.Companies = []domain.MemberCompany{}
	}
	d.MemberView.Companies = d.Companies
	return d, nil
}

type assignChange struct {
	kind string // add | role | remove
	row  *model.UserAccountSSO
	cid  string
	rid  string
}

func (s *MembershipService) SyncAssignments(a domain.Actor, companyID, id string, grants []domain.CompanyAssignment) (*domain.MemberDetail, error) {
	m, err := s.loadMember(companyID, id)
	if err != nil {
		return nil, err
	}
	if s.isSelf(a, m) {
		return nil, &domain.ErrInvalidRole{Msg: "you can't change your own company access"}
	}
	if len(grants) == 0 {
		return nil, &domain.ErrInvalidRole{Msg: "at least one company is required"}
	}
	visible, err := s.manageableIDs(a, permUserList)
	if err != nil {
		return nil, err
	}
	existingRows, err := s.repo.ListByEmailInCompanies([]string{m.Email}, idsOf(visible))
	if err != nil {
		return nil, err
	}
	existing := map[string]*model.UserAccountSSO{}
	for i := range existingRows {
		existing[existingRows[i].CompanyID] = &existingRows[i]
	}

	ctx := context.Background()
	want := map[string]string{}
	for _, g := range grants {
		cid, rid := strings.TrimSpace(g.CompanyID), strings.TrimSpace(g.RoleID)
		co, ok := visible[cid]
		if !ok {
			return nil, &domain.ErrNoMembership{}
		}
		if rid == "" {
			return nil, &domain.ErrInvalidRole{Msg: "select a role for " + co.Name}
		}
		want[cid] = rid
	}

	// Validate everything before writing anything.
	var changes []assignChange
	for cid, rid := range want {
		roles := s.listRolesQuiet(ctx, cid, a.Token)
		if !roleExists(roles, rid) {
			return nil, &domain.ErrInvalidRole{Msg: "role not recognized for " + visible[cid].Name}
		}
		owner := s.ownerRoleIDFromList(roles)
		row := existing[cid]
		if row == nil {
			if !s.can(a, cid, permUserInvite, permUserCreate) {
				return nil, &domain.ErrNoMembership{}
			}
			used, err := s.repo.CountConsumedInviteSlots(cid, owner)
			if err != nil {
				return nil, err
			}
			limit := s.userLimit(cid)
			if domain_activation.Reached(used, limit) {
				return nil, &domain.ErrInviteQuota{Limit: limit}
			}
			changes = append(changes, assignChange{kind: "add", cid: cid, rid: rid})
			continue
		}
		if strval(row.RoleID) != rid {
			if !s.can(a, cid, permUserUpdate) {
				return nil, &domain.ErrNoMembership{}
			}
			if err := s.guardLastOwner(cid, owner, strval(row.RoleID), rid); err != nil {
				return nil, err
			}
			changes = append(changes, assignChange{kind: "role", row: row, cid: cid, rid: rid})
		}
	}
	for cid, row := range existing {
		if _, keep := want[cid]; keep {
			continue
		}
		if !s.can(a, cid, permUserDelete) {
			return nil, &domain.ErrNoMembership{}
		}
		owner := s.resolveOwnerRoleID(ctx, cid, a.Token)
		if err := s.guardLastOwner(cid, owner, strval(row.RoleID), ""); err != nil {
			return nil, err
		}
		changes = append(changes, assignChange{kind: "remove", row: row, cid: cid})
	}

	now := time.Now()
	inviter := strings.TrimSpace(a.UserID)
	for _, c := range changes {
		switch c.kind {
		case "role":
			old := strval(c.row.RoleID)
			rid := c.rid
			c.row.RoleID = &rid
			if err := s.repo.Save(c.row); err != nil {
				return nil, fmt.Errorf("update member role: %w", err)
			}
			s.invalidateMemberCaches(ctx, c.row.UserID, c.cid, old, rid)
		case "remove":
			if err := s.repo.SoftDeleteByID(c.row.ID); err != nil {
				return nil, fmt.Errorf("remove member: %w", err)
			}
			s.invalidateMemberCaches(ctx, c.row.UserID, c.cid, strval(c.row.RoleID), "")
		case "add":
			rid := c.rid
			n := &model.UserAccountSSO{
				CompanyID: c.cid, RoleID: &rid, Email: m.Email, Name: m.Name, Phone: m.Phone,
				InvitedBy: &inviter, InvitedAt: &now, IsBanned: m.IsBanned,
			}
			if m.UserID != "" {
				// A known person: the admin's grant takes effect straight away.
				uid := m.UserID
				n.UserID, n.SecondaryID, n.IsActivated, n.ActivatedAt = uid, &uid, true, &now
			} else {
				tok := newInviteToken()
				n.InviteToken = &tok
			}
			if err := s.repo.Upsert(n); err != nil {
				return nil, fmt.Errorf("add company access: %w", err)
			}
			if n.UserID != "" {
				s.purgeUserAccess(ctx, n.UserID)
			} else {
				s.sendInviteEmail(ctx, n, a)
			}
		}
	}
	return s.GetMember(a, companyID, id)
}

func (s *MembershipService) isSelf(a domain.Actor, m *model.UserAccountSSO) bool {
	return strings.EqualFold(strings.TrimSpace(m.Email), strings.TrimSpace(a.Email)) ||
		(m.UserID != "" && m.UserID == a.UserID)
}

// ── status / resend / profile ──────────────────────────────────────────────

func (s *MembershipService) SetMemberActive(a domain.Actor, companyID, id string, active bool) (*domain.MemberView, error) {
	m, err := s.loadMember(companyID, id)
	if err != nil {
		return nil, err
	}
	if s.isSelf(a, m) {
		return nil, &domain.ErrInvalidRole{Msg: "you can't deactivate yourself"}
	}
	ctx := context.Background()
	roles := s.listRolesQuiet(ctx, m.CompanyID, a.Token)
	if !active {
		if err := s.guardLastOwner(m.CompanyID, s.ownerRoleIDFromList(roles), strval(m.RoleID), ""); err != nil {
			return nil, err
		}
	}
	m.IsBanned = !active
	if active {
		m.BannedReason = ""
	} else {
		m.BannedReason = "deactivated"
	}
	if err := s.repo.Save(m); err != nil {
		return nil, fmt.Errorf("set member status: %w", err)
	}
	s.invalidateMemberCaches(ctx, m.UserID, m.CompanyID, strval(m.RoleID), "")
	v := toMemberView(m, roleNameIndex(roles))
	return &v, nil
}

// ResendInvite mints a fresh token (the old link stops working) and emails the invitation again. The
// returned link is only filled outside production.
func (s *MembershipService) ResendInvite(a domain.Actor, companyID, id string) (string, error) {
	m, err := s.loadMember(companyID, id)
	if err != nil {
		return "", err
	}
	if !m.Pending() {
		return "", &domain.ErrInvalidRole{Msg: "this user has no pending invitation"}
	}
	tok := newInviteToken()
	now := time.Now()
	m.InviteToken, m.InvitedAt = &tok, &now
	if err := s.repo.Save(m); err != nil {
		return "", fmt.Errorf("resend invite: %w", err)
	}
	url := s.sendInviteEmail(context.Background(), m, a)
	if !s.cfg.ExposeInviteURL {
		url = ""
	}
	return url, nil
}

func (s *MembershipService) UpdateMemberProfile(a domain.Actor, companyID, id, name, phone string) (*domain.MemberView, error) {
	m, err := s.loadMember(companyID, id)
	if err != nil {
		return nil, err
	}
	name, phone = strings.TrimSpace(name), strings.TrimSpace(phone)
	if name == "" {
		return nil, &domain.ErrInvalidRole{Msg: "name is required"}
	}
	m.Name, m.Phone = name, phone
	if err := s.repo.Save(m); err != nil {
		return nil, fmt.Errorf("update member profile: %w", err)
	}
	v := toMemberView(m, roleNameIndex(s.listRolesQuiet(context.Background(), m.CompanyID, a.Token)))
	return &v, nil
}

// userLimit is the Free-tier "how many people may hold access to this company" cap — 8 before the
// activation milestone (company profile + 3 partners + 1 invoice), unlimited once it's met. Falls
// back to the old fixed quota if the company row can't be read, so a transient lookup failure never
// widens or (more importantly) silently shrinks the limit a caller already validated against.
func (s *MembershipService) userLimit(companyID string) int {
	c, err := s.companies.FindByID(companyID)
	if err != nil || c == nil {
		return model.FreeInviteQuota
	}
	return domain_activation.LimitsFor(c.ActivationStatus).Users
}
