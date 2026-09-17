package controller

import (
	"github.com/gofiber/fiber/v2"

	membership "duluin_invoice/app/domain/membership"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type RoleController struct {
	svc membership.IRoleService
}

func NewRoleController(svc membership.IRoleService) *RoleController {
	return &RoleController{svc: svc}
}

// GET /api/v1/roles
func (ctrl *RoleController) List(c *fiber.Ctx) error {
	roles, err := ctrl.svc.List(membershipActor(c), middlewares.GetCompanyID(c))
	if err != nil {
		return handleMembershipError(c, err)
	}
	return utils.Ok(c, roles, "OK")
}

// GET /api/v1/roles/:id
func (ctrl *RoleController) Get(c *fiber.Ctx) error {
	detail, err := ctrl.svc.Get(membershipActor(c), middlewares.GetCompanyID(c), c.Params("id"))
	if err != nil {
		return handleMembershipError(c, err)
	}
	return utils.Ok(c, detail, "OK")
}

type roleWriteBody struct {
	Name        string   `json:"name" validate:"required,min=2,max=100"`
	Permissions []string `json:"permissions" validate:"omitempty,dive,max=100"`
}

// POST /api/v1/roles
func (ctrl *RoleController) Create(c *fiber.Ctx) error {
	var body roleWriteBody
	if err := c.BodyParser(&body); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&body); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	role, err := ctrl.svc.Create(membershipActor(c), middlewares.GetCompanyID(c), body.Name, body.Permissions)
	if err != nil {
		return handleMembershipError(c, err)
	}
	return utils.Created(c, role, "Role dibuat")
}

// PUT /api/v1/roles/:id
func (ctrl *RoleController) Update(c *fiber.Ctx) error {
	var body roleWriteBody
	if err := c.BodyParser(&body); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&body); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}
	role, err := ctrl.svc.Update(membershipActor(c), middlewares.GetCompanyID(c), c.Params("id"), body.Name, body.Permissions)
	if err != nil {
		return handleMembershipError(c, err)
	}
	return utils.Ok(c, role, "Role updated")
}

// DELETE /api/v1/roles/:id
func (ctrl *RoleController) Delete(c *fiber.Ctx) error {
	if err := ctrl.svc.Delete(membershipActor(c), middlewares.GetCompanyID(c), c.Params("id")); err != nil {
		return handleMembershipError(c, err)
	}
	return utils.Deleted(c, "Role deleted")
}
