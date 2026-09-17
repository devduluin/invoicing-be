package controller

import (
	"github.com/gofiber/fiber/v2"

	membership "duluin_invoice/app/domain/membership"
	"duluin_invoice/middlewares"
)

// membershipActor flattens the request context into a membership.Actor.
func membershipActor(c *fiber.Ctx) membership.Actor {
	return membership.Actor{
		UserID:          middlewares.GetUserID(c),
		Email:           middlewares.GetEmail(c),
		Name:            middlewares.GetName(c),
		ActiveCompanyID: middlewares.GetCompanyID(c),
		Token:           c.Get("Authorization"),
	}
}
