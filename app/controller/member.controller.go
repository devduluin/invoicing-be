package controller

import (
	"github.com/gofiber/fiber/v2"

	membership "duluin_invoice/app/domain/membership"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type MemberController struct {
	svc membership.IMembershipService
}

func NewMemberController(svc membership.IMembershipService) *MemberController {
	return &MemberController{svc: svc}
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
	return utils.Ok(c, m, "Undangan diterima")
}
