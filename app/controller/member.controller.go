package controller

import (
	"fmt"

	"github.com/gofiber/fiber/v2"

	audit "duluin_invoice/app/domain/audit"
	membership "duluin_invoice/app/domain/membership"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type MemberController struct {
	svc   membership.IMembershipService
	audit audit.ILogger
}

func NewMemberController(svc membership.IMembershipService, auditSvc audit.ILogger) *MemberController {
	return &MemberController{svc: svc, audit: auditSvc}
}

// GET /api/v1/members
func (ctrl *MemberController) List(c *fiber.Ctx) error {
	res, err := ctrl.svc.ListMembers(membershipActor(c), membership.MemberFilter{
		CompanyID: middlewares.GetCompanyID(c),
		Search:    c.Query("search"),
		Page:      c.QueryInt("page", 1),
		PageSize:  c.QueryInt("limit", 20),
		Sort:      c.Query("sort"),
		Order:     c.Query("order"),
		Fields:    utils.ParseCSVParam(c.Query("fields")),
	})
	if err != nil {
		return handleMembershipError(c, err)
	}
	return utils.List(c, res, "OK")
}

type inviteBody struct {
	Email  string `json:"email" validate:"required,email,max=150"`
	Name   string `json:"name" validate:"omitempty,max=255"`
	RoleID string `json:"role_id" validate:"required,max=64"`
}

// POST /api/v1/members/invite
func (ctrl *MemberController) Invite(c *fiber.Ctx) error {
	var body inviteBody
	if err := c.BodyParser(&body); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&body); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	invite, err := ctrl.svc.Invite(membershipActor(c), middlewares.GetCompanyID(c), membership.InviteInput{
		Email:  body.Email,
		Name:   body.Name,
		RoleID: body.RoleID,
	})
	if err != nil {
		return handleMembershipError(c, err)
	}
	return utils.Created(c, invite, "Undangan terkirim")
}

type roleBody struct {
	RoleID string `json:"role_id" validate:"required,max=64"`
}

// PATCH /api/v1/members/:id/role
func (ctrl *MemberController) UpdateRole(c *fiber.Ctx) error {
	var body roleBody
	if err := c.BodyParser(&body); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&body); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	view, err := ctrl.svc.UpdateMemberRole(membershipActor(c), middlewares.GetCompanyID(c), c.Params("id"), body.RoleID)
	if err != nil {
		return handleMembershipError(c, err)
	}
	return utils.Ok(c, view, "Member role updated")
}

// DELETE /api/v1/members/:id
func (ctrl *MemberController) Remove(c *fiber.Ctx) error {
	if err := ctrl.svc.RemoveMember(membershipActor(c), middlewares.GetCompanyID(c), c.Params("id")); err != nil {
		return handleMembershipError(c, err)
	}
	return utils.Deleted(c, "Member removed")
}

type acceptBody struct {
	Token string `json:"token" validate:"required,max=128"`
}

// POST /api/v1/members/me/accept — accept an invitation (membership-exempt).
func (ctrl *MemberController) Accept(c *fiber.Ctx) error {
	var body acceptBody
	if err := c.BodyParser(&body); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&body); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	m, err := ctrl.svc.AcceptInvite(middlewares.GetUserID(c), middlewares.GetName(c), body.Token)
	if err != nil {
		return handleMembershipError(c, err)
	}
	// The membership-exempt route has no active-company header yet — the membership just accepted
	// IS the company this action belongs to.
	ctrl.audit.Log(audit.Actor{
		CompanyID: m.CompanyID,
		UserID:    middlewares.GetUserID(c),
		Name:      middlewares.GetName(c),
		Email:     middlewares.GetEmail(c),
		IPAddress: c.IP(),
		UserAgent: c.Get("User-Agent"),
	}, audit.Entry{
		Action:      audit.ActionAcceptedInvitation,
		Module:      audit.ModuleUserManagement,
		EntityType:  "member",
		EntityID:    m.ID,
		EntityName:  displayName(m.Name, m.Email),
		Description: "Accepted invitation to join the workspace",
	})
	return utils.Ok(c, m, "Undangan diterima")
}

type validateBody struct {
	Email string `json:"email" validate:"required,email,max=150"`
}

// POST /api/v1/members/validate — what is already known about this email.
func (ctrl *MemberController) Validate(c *fiber.Ctx) error {
	var body validateBody
	if err := c.BodyParser(&body); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&body); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	res, err := ctrl.svc.ValidateUser(membershipActor(c), body.Email)
	if err != nil {
		return handleMembershipError(c, err)
	}
	return utils.Ok(c, res, "User status validated")
}

// GET /api/v1/members/companies?permission=invite|update — companies the caller can grant access to.
func (ctrl *MemberController) Companies(c *fiber.Ctx) error {
	perm := "invoice-user-invite"
	if c.Query("permission") == "update" {
		perm = "invoice-user-update"
	}
	list, err := ctrl.svc.ManageableCompanies(membershipActor(c), perm)
	if err != nil {
		return handleMembershipError(c, err)
	}
	return utils.Ok(c, list, "OK")
}

type grantBody struct {
	CompanyID string `json:"company_id" validate:"required,max=64"`
	RoleID    string `json:"role_id" validate:"required,max=64"`
}

type inviteMultiBody struct {
	Email       string      `json:"email" validate:"required,email,max=150"`
	Name        string      `json:"name" validate:"required,max=255"`
	Phone       string      `json:"phone" validate:"omitempty,max=32"`
	IsActive    *bool       `json:"is_active"`
	SendEmail   *bool       `json:"send_email"`
	Memberships []grantBody `json:"memberships" validate:"required,min=1,dive"`
}

func toGrants(in []grantBody) []membership.CompanyAssignment {
	out := make([]membership.CompanyAssignment, 0, len(in))
	for _, g := range in {
		out = append(out, membership.CompanyAssignment{CompanyID: g.CompanyID, RoleID: g.RoleID})
	}
	return out
}

func boolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

// POST /api/v1/members/invite-multi — one person, several companies, a role each.
func (ctrl *MemberController) InviteMulti(c *fiber.Ctx) error {
	var body inviteMultiBody
	if err := c.BodyParser(&body); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&body); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	views, err := ctrl.svc.InviteMulti(membershipActor(c), membership.InviteMultiInput{
		Email:     body.Email,
		Name:      body.Name,
		Phone:     body.Phone,
		Active:    boolOr(body.IsActive, true),
		SendEmail: boolOr(body.SendEmail, true),
		Grants:    toGrants(body.Memberships),
	})
	if err != nil {
		return handleMembershipError(c, err)
	}
	for i := range views {
		ctrl.audit.Log(auditActor(c), audit.Entry{
			Action:      audit.ActionInvitedUser,
			Module:      audit.ModuleUserManagement,
			EntityType:  "member",
			EntityID:    views[i].ID,
			EntityName:  displayName(views[i].Name, views[i].Email),
			Description: fmt.Sprintf("Invited %s as %s", views[i].Email, views[i].RoleName),
		})
	}
	return utils.Created(c, views, "Undangan terkirim")
}

// GET /api/v1/members/:id
func (ctrl *MemberController) Get(c *fiber.Ctx) error {
	d, err := ctrl.svc.GetMember(membershipActor(c), middlewares.GetCompanyID(c), c.Params("id"))
	if err != nil {
		return handleMembershipError(c, err)
	}
	return utils.Ok(c, d, "OK")
}

type profileBody struct {
	Name  string `json:"name" validate:"required,max=255"`
	Phone string `json:"phone" validate:"omitempty,max=32"`
}

// PATCH /api/v1/members/:id
func (ctrl *MemberController) UpdateProfile(c *fiber.Ctx) error {
	var body profileBody
	if err := c.BodyParser(&body); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&body); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	v, err := ctrl.svc.UpdateMemberProfile(membershipActor(c), middlewares.GetCompanyID(c), c.Params("id"), body.Name, body.Phone)
	if err != nil {
		return handleMembershipError(c, err)
	}
	return utils.Ok(c, v, "User updated")
}

type assignmentsBody struct {
	Memberships []grantBody `json:"memberships" validate:"required,min=1,dive"`
}

// PUT /api/v1/members/:id/assignments — sync the person's company access.
func (ctrl *MemberController) SyncAssignments(c *fiber.Ctx) error {
	var body assignmentsBody
	if err := c.BodyParser(&body); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&body); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	actor := membershipActor(c)
	before, _ := ctrl.svc.GetMember(actor, middlewares.GetCompanyID(c), c.Params("id"))
	d, err := ctrl.svc.SyncAssignments(actor, middlewares.GetCompanyID(c), c.Params("id"), toGrants(body.Memberships))
	if err != nil {
		return handleMembershipError(c, err)
	}
	ctrl.audit.Log(auditActor(c), audit.Entry{
		Action:      audit.ActionRoleChanged,
		Module:      audit.ModuleUserManagement,
		EntityType:  "member",
		EntityID:    d.ID,
		EntityName:  displayName(d.Name, d.Email),
		Description: fmt.Sprintf("Updated company access for %s", displayName(d.Name, d.Email)),
		Changes:     companyAccessDiff(before, d),
	})
	return utils.Ok(c, d, "Access updated")
}

type statusBody struct {
	Active *bool `json:"active" validate:"required"`
}

// PATCH /api/v1/members/:id/status — activate / deactivate.
func (ctrl *MemberController) SetStatus(c *fiber.Ctx) error {
	var body statusBody
	if err := c.BodyParser(&body); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&body); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	v, err := ctrl.svc.SetMemberActive(membershipActor(c), middlewares.GetCompanyID(c), c.Params("id"), *body.Active)
	if err != nil {
		return handleMembershipError(c, err)
	}
	stateWord := map[bool]string{true: "Activated", false: "Deactivated"}[*body.Active]
	ctrl.audit.Log(auditActor(c), audit.Entry{
		Action:      audit.ActionStatusChanged,
		Module:      audit.ModuleUserManagement,
		EntityType:  "member",
		EntityID:    v.ID,
		EntityName:  displayName(v.Name, v.Email),
		Description: fmt.Sprintf("%s %s", stateWord, displayName(v.Name, v.Email)),
		Changes: map[string]audit.Change{
			"status": {Before: map[bool]string{true: "active", false: "inactive"}[!*body.Active], After: map[bool]string{true: "active", false: "inactive"}[*body.Active]},
		},
	})
	return utils.Ok(c, v, "Status updated")
}

// POST /api/v1/members/:id/resend
func (ctrl *MemberController) Resend(c *fiber.Ctx) error {
	url, err := ctrl.svc.ResendInvite(membershipActor(c), middlewares.GetCompanyID(c), c.Params("id"))
	if err != nil {
		return handleMembershipError(c, err)
	}
	return utils.Ok(c, fiber.Map{"sent": true, "invite_url": url}, "Undangan dikirim ulang")
}

// companyAccessDiff compares a member's per-company role before and after a sync, keyed by company
// name so the audit detail view reads the same way the "CHANGES" table in the UI mock does.
func companyAccessDiff(before, after *membership.MemberDetail) map[string]audit.Change {
	changes := map[string]audit.Change{}
	prev := map[string]string{}
	if before != nil {
		for _, co := range before.Companies {
			prev[co.CompanyName] = co.RoleName
		}
	}
	seen := map[string]bool{}
	if after != nil {
		for _, co := range after.Companies {
			seen[co.CompanyName] = true
			if b, ok := prev[co.CompanyName]; !ok {
				changes[co.CompanyName] = audit.Change{Before: "(no access)", After: co.RoleName}
			} else if b != co.RoleName {
				changes[co.CompanyName] = audit.Change{Before: b, After: co.RoleName}
			}
		}
	}
	for name, role := range prev {
		if !seen[name] {
			changes[name] = audit.Change{Before: role, After: "(removed)"}
		}
	}
	return changes
}
