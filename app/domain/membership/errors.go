package domain_membership

import "fmt"

// ErrNoMembership — the user has no membership row for the company (HTTP 403).
type ErrNoMembership struct{}

func (e *ErrNoMembership) Error() string { return "no access to this company" }

// ErrNotActivated — membership exists but is not activated yet (HTTP 403).
type ErrNotActivated struct{}

func (e *ErrNotActivated) Error() string { return "membership is not activated yet" }

// ErrBanned — membership is banned (HTTP 403).
type ErrBanned struct{ Reason string }

func (e *ErrBanned) Error() string {
	if e.Reason != "" {
		return "membership is banned: " + e.Reason
	}
	return "membership is banned"
}

// ErrLastOwner — the action would leave the company without an owner (HTTP 409).
type ErrLastOwner struct{}

func (e *ErrLastOwner) Error() string { return "can't demote/remove the last owner" }

// ErrInvalidRole — the role id is not a role of this company/account (HTTP 422).
type ErrInvalidRole struct{ Msg string }

func (e *ErrInvalidRole) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	return "invalid role"
}

// ErrDuplicateInvite — the email is already invited or a member (HTTP 409).
type ErrDuplicateInvite struct{ Email string }

func (e *ErrDuplicateInvite) Error() string {
	return fmt.Sprintf("%s is already invited or already a member", e.Email)
}

// ErrInviteQuota — the free invite quota is exhausted (HTTP 409).
type ErrInviteQuota struct{ Limit int }

func (e *ErrInviteQuota) Error() string {
	return fmt.Sprintf("the free invitation quota (%d people) is full", e.Limit)
}

// ErrMemberNotFound — target member/invite does not exist (HTTP 404).
type ErrMemberNotFound struct{ Ref string }

func (e *ErrMemberNotFound) Error() string { return fmt.Sprintf("member %s not found", e.Ref) }

// ErrInviteInvalid — invite token missing/expired/consumed (HTTP 404/409).
type ErrInviteInvalid struct{}

func (e *ErrInviteInvalid) Error() string { return "invitation is invalid or already used" }

// ErrCompanyNotFound — an action needs a company that does not exist (HTTP 404).
type ErrCompanyNotFound struct{}

func (e *ErrCompanyNotFound) Error() string { return "company not found" }

// ErrAccessUnresolved — RBAC could not be resolved; caller must return 503.
type ErrAccessUnresolved struct{ Reason string }

func (e *ErrAccessUnresolved) Error() string {
	return "access could not be verified: " + e.Reason
}

// ErrRoleInUse — a custom role still has active members (HTTP 409).
type ErrRoleInUse struct{ Count int }

func (e *ErrRoleInUse) Error() string {
	return fmt.Sprintf("role is still used by %d member(s)", e.Count)
}
