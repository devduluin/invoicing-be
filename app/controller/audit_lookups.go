package controller

import (
	"github.com/gofiber/fiber/v2"

	contactdomain "duluin_invoice/app/domain/contactperson"
	membership "duluin_invoice/app/domain/membership"
)

// AuditLookup fetches the current stored record of the URL's id for AuditWrites (nil when unknown).
type AuditLookup = func(c *fiber.Ctx, companyID string, ids []string) any

// LookupBy adapts a service's Get(companyID, id) into an AuditLookup.
func LookupBy[T any](get func(companyID, id string) (*T, error)) AuditLookup {
	return func(_ *fiber.Ctx, companyID string, ids []string) any {
		if len(ids) == 0 {
			return nil
		}
		row, err := get(companyID, ids[len(ids)-1])
		if err != nil || row == nil {
			return nil
		}
		return row
	}
}

func RoleAuditLookup(svc membership.IRoleService) AuditLookup {
	return func(c *fiber.Ctx, companyID string, ids []string) any {
		if len(ids) == 0 {
			return nil
		}
		d, err := svc.Get(membershipActor(c), companyID, ids[len(ids)-1])
		if err != nil || d == nil {
			return nil
		}
		return d
	}
}

func MemberAuditLookup(svc membership.IMembershipService) AuditLookup {
	return func(c *fiber.Ctx, companyID string, ids []string) any {
		if len(ids) == 0 {
			return nil
		}
		d, err := svc.GetMember(membershipActor(c), companyID, ids[len(ids)-1])
		if err != nil || d == nil {
			return nil
		}
		return d
	}
}

// ContactAuditLookup reads /mitra/:id/contact-persons/:cid — the partner id first, then the contact's.
func ContactAuditLookup(svc contactdomain.IService) AuditLookup {
	return func(_ *fiber.Ctx, companyID string, ids []string) any {
		if len(ids) < 2 {
			return nil
		}
		row, err := svc.Get(companyID, ids[0], ids[1])
		if err != nil || row == nil {
			return nil
		}
		return row
	}
}
