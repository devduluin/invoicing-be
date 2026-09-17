package controller

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	membership "duluin_invoice/app/domain/membership"
	domain "duluin_invoice/app/domain/onboarding"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

type OnboardingController struct {
	svc domain.IOnboardingService
}

func NewOnboardingController(svc domain.IOnboardingService) *OnboardingController {
	return &OnboardingController{svc: svc}
}

// POST /api/v1/onboarding — the single commit of the wizard draft.
func (ctrl *OnboardingController) Submit(c *fiber.Ctx) error {
	var dto domain.SubmitDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}

	actor := domain.Actor{
		SSOUserID:       middlewares.GetUserID(c),
		Email:           middlewares.GetEmail(c),
		Name:            middlewares.GetName(c),
		ActiveCompanyID: middlewares.GetCompanyID(c),
		Token:           c.Get("Authorization"),
	}

	result, err := ctrl.svc.Submit(actor, dto)
	if err != nil {
		return handleOnboardingError(c, err)
	}
	return utils.Created(c, result, "Company created")
}

func handleOnboardingError(c *fiber.Ctx, err error) error {
	var vErr *domain.ErrValidation
	if errors.As(err, &vErr) {
		return utils.ValidationFailed(c, []string{err.Error()})
	}
	var onboarded *domain.ErrAlreadyOnboarded
	if errors.As(err, &onboarded) {
		return conflict(c, err, "already_onboarded")
	}
	return handleMembershipError(c, err)
}

// handleMembershipError maps the membership-domain errors shared by the
// onboarding submit and the team endpoints.
func handleMembershipError(c *fiber.Ctx, err error) error {
	var invalidRole *membership.ErrInvalidRole
	if errors.As(err, &invalidRole) {
		return utils.ValidationFailed(c, []string{err.Error()})
	}
	var dup *membership.ErrDuplicateInvite
	if errors.As(err, &dup) {
		return conflict(c, err, "duplicate_invite")
	}
	var quota *membership.ErrInviteQuota
	if errors.As(err, &quota) {
		return conflict(c, err, "invite_quota_exceeded")
	}
	var lastOwner *membership.ErrLastOwner
	if errors.As(err, &lastOwner) {
		return conflict(c, err, "last_owner")
	}
	var roleInUse *membership.ErrRoleInUse
	if errors.As(err, &roleInUse) {
		return conflict(c, err, "role_in_use")
	}
	var notFound *membership.ErrMemberNotFound
	if errors.As(err, &notFound) {
		return utils.NotFound(c, []string{err.Error()})
	}
	var inviteInvalid *membership.ErrInviteInvalid
	if errors.As(err, &inviteInvalid) {
		return utils.NotFound(c, []string{err.Error()})
	}
	var noMembership *membership.ErrNoMembership
	if errors.As(err, &noMembership) {
		return utils.Forbidden(c, []string{err.Error()})
	}
	var companyNF *membership.ErrCompanyNotFound
	if errors.As(err, &companyNF) {
		return utils.NotFound(c, []string{err.Error()})
	}
	return utils.InternalError(c, err)
}

func conflict(c *fiber.Ctx, err error, code string) error {
	return c.Status(fiber.StatusConflict).JSON(fiber.Map{
		"success": false, "message": err.Error(), "error_code": code,
	})
}
