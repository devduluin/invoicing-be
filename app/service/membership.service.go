package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	domain "duluin_invoice/app/domain/membership"
	"duluin_invoice/app/model"
	"duluin_invoice/app/notification"
	"duluin_invoice/app/sso"
	"duluin_invoice/utils"
)

// newInviteToken mints an opaque, unguessable invite-accept token.
func newInviteToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// MembershipConfig is injected, never read from a global.
type MembershipConfig struct {
	UseLocalRBAC          bool
	RBACMigrationFallback bool
	OwnerRoleID           string // optional pin for the global "Invoice Owner" role id
}

const (
	accessCacheTTL = 60 * time.Second
	rbacCacheTTL   = 10 * time.Minute

	accessKeyFmt = "invoice:access:%s:%s" // userID, companyID
	rbacKeyFmt   = "invoice:rbac:%s:%s"   // roleID, companyID
)

// MembershipService owns company membership + per-company RBAC resolution.
// Business rules only — persistence via repo, SSO catalog via sso, cache via
// cache. All dependencies are constructor-injected.
type MembershipService struct {
	repo      domain.IMembershipRepository
	companies domain.CompanyReader
	sso       RBACSSOClient
	cache     RBACCache
	notify    notification.Service
	cfg       MembershipConfig
	webURL    string
}

func NewMembershipService(
	repo domain.IMembershipRepository,
	companies domain.CompanyReader,
	ssoClient RBACSSOClient,
	cache RBACCache,
	notify notification.Service,
	cfg MembershipConfig,
	webURL string,
) *MembershipService {
	return &MembershipService{
		repo: repo, companies: companies, sso: ssoClient, cache: cache,
		notify: notify, cfg: cfg, webURL: strings.TrimRight(webURL, "/"),
	}
}

var _ domain.IMembershipService = (*MembershipService)(nil)

// ── ResolveAccess ──────────────────────────────────────────────────────────

func (s *MembershipService) ResolveAccess(a domain.Actor, companyID string, ssoRoles, ssoPerms []string) (*domain.AccessResolution, error) {
	userID := strings.TrimSpace(a.UserID)
	companyID = strings.TrimSpace(companyID)
	if userID == "" || companyID == "" {
		res := &domain.AccessResolution{HasAccess: false}
		res.MarkResolved()
		return res, nil
	}

	ctx := context.Background()
	accessKey := fmt.Sprintf(accessKeyFmt, userID, companyID)

	var cached domain.AccessResolution
	if s.cache.GetJSON(ctx, accessKey, &cached) && cached.ResolutionStatus != "" {
		return &cached, nil
	}

	m, err := s.repo.FindByUserAndCompany(userID, companyID)
	if err != nil {
		return nil, fmt.Errorf("resolve access: %w", err)
	}
	if m == nil {
		res := &domain.AccessResolution{HasAccess: false}
		res.MarkResolved()
		s.cache.SetJSON(ctx, accessKey, res, accessCacheTTL)
		return res, nil
	}

	res := &domain.AccessResolution{
		HasAccess:    true,
		IsActivated:  m.IsActivated,
		IsBanned:     m.IsBanned,
		BannedReason: m.BannedReason,
	}

	roleID := strval(m.RoleID)
	switch {
	case roleID == "":
		if s.allowSSOFallback(companyID) {
			s.applySSOFallback(res, ssoRoles, ssoPerms)
			res.MarkResolved()
		} else {
			res.MarkUnresolved("role has not been set for this membership")
		}
	default:
		perms, name, rErr := s.resolveRolePermissions(ctx, roleID, companyID, a.Token)
		if rErr != nil {
			if s.cfg.RBACMigrationFallback && len(ssoPerms) > 0 {
				s.applySSOFallback(res, ssoRoles, ssoPerms)
				res.MarkResolved()
			} else {
				res.MarkUnresolved("failed to fetch role permissions from SSO")
			}
		} else {
			res.RoleID = roleID
			res.RoleName = name
			res.Permissions = perms
			if name != "" {
				res.Roles = []string{name}
			}
			res.MarkResolved()
		}
	}

	if !res.IsUnresolved() {
		s.cache.SetJSON(ctx, accessKey, res, accessCacheTTL)
	}
	return res, nil
}

func (s *MembershipService) allowSSOFallback(companyID string) bool {
	if s.cfg.RBACMigrationFallback {
		return true
	}
	c, err := s.companies.FindByID(companyID)
	return err == nil && c != nil && c.OnboardingStatus == model.OnboardingPending
}

func (s *MembershipService) applySSOFallback(res *domain.AccessResolution, roles, perms []string) {
	res.UsedSSOFallback = true
	res.Roles = dedupeStrings(roles)
	res.Permissions = dedupeStrings(perms)
}

// resolveRolePermissions reads role→perms with a 10m Redis cache and one retry
// on a temporary SSO error.
func (s *MembershipService) resolveRolePermissions(ctx context.Context, roleID, companyID, token string) (perms []string, name string, err error) {
	key := fmt.Sprintf(rbacKeyFmt, roleID, companyID)

	var cached domain.RolePerms
	if s.cache.GetJSON(ctx, key, &cached) && cached.RoleID == roleID {
		return cached.Permissions, cached.RoleName, nil
	}

	rp, err := s.sso.GetRolePermissions(ctx, roleID, companyID, token)
	if err != nil && ssoIsTemporary(err) {
		rp, err = s.sso.GetRolePermissions(ctx, roleID, companyID, token)
	}
	if err != nil {
		return nil, "", err
	}

	perms = dedupeStrings(rp.Permissions)
	name = rp.RoleName
	s.cache.SetJSON(ctx, key, domain.RolePerms{RoleID: roleID, RoleName: name, Permissions: perms}, rbacCacheTTL)
	return perms, name, nil
}

// ── LinkCompanyCreator ─────────────────────────────────────────────────────

// LinkCompanyCreator makes the actor the owner of a freshly created company:
// upserts an activated owner membership and best-effort syncs SSO (secondary_id
// = the user's own id per the SSO convention; coarse owner role).
func (s *MembershipService) LinkCompanyCreator(a domain.Actor, companyID string) error {
	userID := strings.TrimSpace(a.UserID)
	companyID = strings.TrimSpace(companyID)
	if userID == "" || companyID == "" {
		return fmt.Errorf("link company creator: missing user or company id")
	}
	ctx := context.Background()

	ownerRoleID := s.resolveOwnerRoleID(ctx, companyID, a.Token)

	now := time.Now()
	m := &model.UserAccountSSO{
		UserID:      userID,
		CompanyID:   companyID,
		SecondaryID: &userID,
		Email:       strings.ToLower(strings.TrimSpace(a.Email)),
		Name:        strings.TrimSpace(a.Name),
		IsActivated: true,
		ActivatedAt: &now,
	}
	if ownerRoleID != "" {
		m.RoleID = &ownerRoleID
	}
	if err := s.repo.Upsert(m); err != nil {
		return fmt.Errorf("link company creator: %w", err)
	}

	s.syncSecondaryID(ctx, userID)
	if err := s.sso.AssignRole(ctx, userID, model.SSORoleOwner); err != nil {
		log.Printf("[membership] assign owner role failed user=%s err=%v", userID, err)
	}

	s.purgeUserAccess(ctx, userID)
	return nil
}

// syncSecondaryID best-effort mirrors the SSO account's secondary_id to the
// user's own id (the SSO convention — it's an identity link, not a company).
// Idempotent, so it's safe to call on every membership write.
func (s *MembershipService) syncSecondaryID(ctx context.Context, userID string) {
	if err := s.sso.SetSecondaryID(ctx, userID, userID); err != nil {
		log.Printf("[membership] set_secondary_id failed user=%s err=%v", userID, err)
	}
}

// resolveOwnerRoleID finds the global "Invoice Owner" role id. Company list
// first (one call already needed elsewhere), then the global list, then the
// configured pin. Empty is tolerated — ResolveAccess falls back.
func (s *MembershipService) resolveOwnerRoleID(ctx context.Context, companyID, token string) string {
	if id := findOwnerRole(s.listRolesQuiet(ctx, companyID, token)); id != "" {
		return id
	}
	if id := findOwnerRole(s.listRolesQuiet(ctx, "", token)); id != "" {
		return id
	}
	return strings.TrimSpace(s.cfg.OwnerRoleID)
}

// ownerRoleIDFromList reuses an already-fetched company role list (which
// includes the global roles) instead of a second SSO round-trip.
func (s *MembershipService) ownerRoleIDFromList(roles []domain.RoleInfo) string {
	if id := findOwnerRole(roles); id != "" {
		return id
	}
	return strings.TrimSpace(s.cfg.OwnerRoleID)
}

func (s *MembershipService) listRolesQuiet(ctx context.Context, companyID, token string) []domain.RoleInfo {
	roles, err := s.sso.ListRoles(ctx, companyID, token)
	if err != nil {
		log.Printf("[membership] list roles failed company=%q err=%v", companyID, err)
		return nil
	}
	return toRoleInfos(roles)
}

func findOwnerRole(roles []domain.RoleInfo) string {
	for _, r := range roles {
		if !r.IsCustom && strings.EqualFold(strings.TrimSpace(r.Name), model.SSORoleOwner) {
			return r.ID
		}
	}
	return ""
}

// ── Invite / AcceptInvite ──────────────────────────────────────────────────

func (s *MembershipService) Invite(a domain.Actor, companyID string, in domain.InviteInput) (*model.UserAccountSSO, error) {
	companyID = strings.TrimSpace(companyID)
	email := strings.ToLower(strings.TrimSpace(in.Email))
	roleID := strings.TrimSpace(in.RoleID)
	if companyID == "" {
		return nil, &domain.ErrCompanyNotFound{}
	}
	if email == "" {
		return nil, &domain.ErrInvalidRole{Msg: "email is required"}
	}
	if email == strings.ToLower(strings.TrimSpace(a.Email)) {
		return nil, &domain.ErrInvalidRole{Msg: "can't invite yourself"}
	}
	ctx := context.Background()

	roles := s.listRolesQuiet(ctx, companyID, a.Token)
	if !roleExists(roles, roleID) {
		return nil, &domain.ErrInvalidRole{Msg: "role not recognized for this company"}
	}
	ownerRoleID := s.ownerRoleIDFromList(roles)
	if ownerRoleID != "" && roleID == ownerRoleID {
		return nil, &domain.ErrInvalidRole{Msg: "can't invite a member as an owner"}
	}

	if existing, err := s.repo.FindActiveMemberByEmail(companyID, email); err != nil {
		return nil, err
	} else if existing != nil {
		return nil, &domain.ErrDuplicateInvite{Email: email}
	}
	if pending, err := s.repo.FindPendingInviteByEmail(companyID, email); err != nil {
		return nil, err
	} else if pending != nil {
		return nil, &domain.ErrDuplicateInvite{Email: email}
	}

	used, err := s.repo.CountConsumedInviteSlots(companyID, ownerRoleID)
	if err != nil {
		return nil, err
	}
	if used >= int64(model.FreeInviteQuota) {
		return nil, &domain.ErrInviteQuota{Limit: model.FreeInviteQuota}
	}

	now := time.Now()
	tok := newInviteToken()
	inviter := strings.TrimSpace(a.UserID)
	m := &model.UserAccountSSO{
		CompanyID:   companyID,
		RoleID:      &roleID,
		Email:       email,
		Name:        strings.TrimSpace(in.Name),
		IsActivated: false,
		InviteToken: &tok,
		InvitedBy:   &inviter,
		InvitedAt:   &now,
	}
	if err := s.repo.Upsert(m); err != nil {
		return nil, fmt.Errorf("invite: %w", err)
	}

	s.sendInviteEmail(ctx, m, a)
	return m, nil
}

func (s *MembershipService) sendInviteEmail(ctx context.Context, m *model.UserAccountSSO, a domain.Actor) {
	if s.notify == nil {
		return
	}
	companyName := ""
	if c, err := s.companies.FindByID(m.CompanyID); err == nil && c != nil {
		companyName = c.Name
	}
	token := ""
	if m.InviteToken != nil {
		token = *m.InviteToken
	}
	_ = s.notify.SendUserInvite(ctx, notification.UserInvite{
		ToEmail:     m.Email,
		ToName:      m.Name,
		InviterName: a.Name,
		TenantName:  companyName,
		AcceptURL:   s.acceptURL(token),
	})
}

func (s *MembershipService) acceptURL(token string) string {
	if s.webURL == "" || token == "" {
		return token
	}
	return s.webURL + "/invite/" + token
}

func (s *MembershipService) AcceptInvite(userID, name, token string) (*model.UserAccountSSO, error) {
	userID = strings.TrimSpace(userID)
	token = strings.TrimSpace(token)
	if userID == "" || token == "" {
		return nil, &domain.ErrInviteInvalid{}
	}
	m, err := s.repo.FindInviteByToken(token)
	if err != nil {
		return nil, err
	}
	if m == nil || m.IsActivated || strings.TrimSpace(m.UserID) != "" {
		return nil, &domain.ErrInviteInvalid{}
	}

	ctx := context.Background()
	now := time.Now()
	m.UserID = userID
	m.IsActivated = true
	m.ActivatedAt = &now
	m.InviteToken = nil
	m.SecondaryID = &userID
	if name != "" && strings.TrimSpace(m.Name) == "" {
		m.Name = strings.TrimSpace(name)
	}
	if err := s.repo.Save(m); err != nil {
		return nil, fmt.Errorf("accept invite: %w", err)
	}

	s.syncSecondaryID(ctx, userID)
	s.purgeUserAccess(ctx, userID)
	return m, nil
}

// ── team page ──────────────────────────────────────────────────────────────

func (s *MembershipService) ListMembers(a domain.Actor, f domain.MemberFilter) (*utils.OffsetPaginationResult, error) {
	f.CompanyID = strings.TrimSpace(f.CompanyID)
	if f.CompanyID == "" {
		return nil, &domain.ErrCompanyNotFound{}
	}
	rows, total, err := s.repo.ListByCompany(f)
	if err != nil {
		return nil, err
	}

	names := roleNameIndex(s.listRolesQuiet(context.Background(), f.CompanyID, a.Token))
	out := make([]domain.MemberView, 0, len(rows))
	for i := range rows {
		out = append(out, toMemberView(&rows[i], names))
	}
	return &utils.OffsetPaginationResult{
		Data:       utils.ToJSONMaps(out),
		Columns:    domain.MemberListColumns,
		Attributes: domain.MemberListColumns,
		Meta:       utils.BuildOffsetMeta(total, f.Page, f.PageSize),
	}, nil
}

func (s *MembershipService) UpdateMemberRole(a domain.Actor, companyID, targetMemberID, roleID string) (*domain.MemberView, error) {
	companyID = strings.TrimSpace(companyID)
	roleID = strings.TrimSpace(roleID)
	ctx := context.Background()

	roles := s.listRolesQuiet(ctx, companyID, a.Token)
	if !roleExists(roles, roleID) {
		return nil, &domain.ErrInvalidRole{Msg: "role not recognized for this company"}
	}
	m, err := s.repo.FindByIDInCompany(companyID, targetMemberID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, &domain.ErrMemberNotFound{Ref: targetMemberID}
	}

	ownerRoleID := s.ownerRoleIDFromList(roles)
	oldRoleID := strval(m.RoleID)
	if err := s.guardLastOwner(companyID, ownerRoleID, oldRoleID, roleID); err != nil {
		return nil, err
	}

	m.RoleID = &roleID
	if err := s.repo.Save(m); err != nil {
		return nil, fmt.Errorf("update member role: %w", err)
	}

	s.invalidateMemberCaches(ctx, m.UserID, companyID, oldRoleID, roleID)
	view := toMemberView(m, roleNameIndex(roles))
	return &view, nil
}

func (s *MembershipService) RemoveMember(a domain.Actor, companyID, targetMemberID string) error {
	companyID = strings.TrimSpace(companyID)
	ctx := context.Background()

	m, err := s.repo.FindByIDInCompany(companyID, targetMemberID)
	if err != nil {
		return err
	}
	if m == nil {
		return &domain.ErrMemberNotFound{Ref: targetMemberID}
	}
	ownerRoleID := s.resolveOwnerRoleID(ctx, companyID, a.Token)
	if err := s.guardLastOwner(companyID, ownerRoleID, strval(m.RoleID), ""); err != nil {
		return err
	}
	if err := s.repo.SoftDeleteByID(m.ID); err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	s.invalidateMemberCaches(ctx, m.UserID, companyID, strval(m.RoleID), "")
	return nil
}

// guardLastOwner blocks demoting/removing the last active owner.
func (s *MembershipService) guardLastOwner(companyID, ownerRoleID, currentRoleID, nextRoleID string) error {
	if ownerRoleID == "" || currentRoleID != ownerRoleID || nextRoleID == ownerRoleID {
		return nil
	}
	n, err := s.repo.CountActiveByRole(companyID, ownerRoleID)
	if err != nil {
		return err
	}
	if n <= 1 {
		return &domain.ErrLastOwner{}
	}
	return nil
}

func (s *MembershipService) ListMyCompanies(a domain.Actor) ([]domain.CompanyMembership, error) {
	userID := strings.TrimSpace(a.UserID)
	if userID == "" {
		return []domain.CompanyMembership{}, nil
	}
	rows, err := s.repo.ListActiveByUser(userID)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	globalNames := roleNameIndex(s.listRolesQuiet(ctx, "", a.Token))
	ownerRoleID := findOwnerRole(s.listRolesQuiet(ctx, "", a.Token))

	out := make([]domain.CompanyMembership, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		roleID := strval(row.RoleID)
		name := globalNames[roleID]
		if name == "" {
			// company-scoped custom role — cache-only lookup, no per-row SSO call.
			var rp domain.RolePerms
			if s.cache.GetJSON(ctx, fmt.Sprintf(rbacKeyFmt, roleID, row.CompanyID), &rp) {
				name = rp.RoleName
			}
		}
		out = append(out, domain.CompanyMembership{
			Company:     row.Company,
			RoleID:      roleID,
			RoleName:    name,
			IsActivated: row.IsActivated,
			IsOwner:     ownerRoleID != "" && roleID == ownerRoleID,
		})
	}
	return out, nil
}

// ── cache invalidation ─────────────────────────────────────────────────────

func (s *MembershipService) purgeUserAccess(ctx context.Context, userID string) {
	s.cache.DelByPattern(ctx, fmt.Sprintf("invoice:access:%s:*", userID))
}

func (s *MembershipService) invalidateMemberCaches(ctx context.Context, userID, companyID, oldRoleID, newRoleID string) {
	if userID != "" {
		s.cache.Del(ctx, fmt.Sprintf(accessKeyFmt, userID, companyID))
	}
	for _, rid := range []string{oldRoleID, newRoleID} {
		if rid != "" {
			s.cache.Del(ctx, fmt.Sprintf(rbacKeyFmt, rid, companyID))
		}
	}
}

// InvalidateRoleCache drops the cached permission set for a role in a company —
// called by RoleService after a role edit.
func (s *MembershipService) InvalidateRoleCache(companyID, roleID string) {
	if roleID == "" || companyID == "" {
		return
	}
	s.cache.Del(context.Background(), fmt.Sprintf(rbacKeyFmt, roleID, companyID))
}

// ── small helpers ──────────────────────────────────────────────────────────

func strval(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func toRoleInfos(in []sso.Role) []domain.RoleInfo {
	out := make([]domain.RoleInfo, 0, len(in))
	for _, r := range in {
		out = append(out, domain.RoleInfo{
			ID: r.ID, Name: r.Name, CompanyID: r.CompanyID,
			IsCustom: r.IsCustom, Permissions: r.Permissions,
		})
	}
	return out
}

func roleExists(roles []domain.RoleInfo, id string) bool {
	if id == "" {
		return false
	}
	for _, r := range roles {
		if r.ID == id {
			return true
		}
	}
	return false
}

func roleNameIndex(roles []domain.RoleInfo) map[string]string {
	m := make(map[string]string, len(roles))
	for _, r := range roles {
		m[r.ID] = r.Name
	}
	return m
}

func toMemberView(m *model.UserAccountSSO, names map[string]string) domain.MemberView {
	roleID := strval(m.RoleID)
	var invitedAt *string
	if m.InvitedAt != nil {
		s := m.InvitedAt.UTC().Format(time.RFC3339)
		invitedAt = &s
	}
	return domain.MemberView{
		ID:          m.ID,
		UserID:      m.UserID,
		Email:       m.Email,
		Name:        m.Name,
		RoleID:      roleID,
		RoleName:    names[roleID],
		IsActivated: m.IsActivated,
		IsBanned:    m.IsBanned,
		Pending:     m.Pending(),
		InvitedAt:   invitedAt,
	}
}
