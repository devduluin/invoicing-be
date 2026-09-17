package middlewares

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	membership "duluin_invoice/app/domain/membership"
)

func TestRequireCompletedOnboarding(t *testing.T) {
	cases := []struct {
		name       string
		status     string
		wantStatus int
	}{
		{"no company", "", fiber.StatusForbidden},
		{"pending", "pending", fiber.StatusForbidden},
		{"active", "active", fiber.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			app.Use(func(c *fiber.Ctx) error {
				if tc.status != "" {
					c.Locals("onboardingStatus", tc.status)
				}
				return c.Next()
			})
			app.Get("/x", RequireCompletedOnboarding(), func(c *fiber.Ctx) error { return c.SendString("ok") })

			resp, _ := app.Test(httptest.NewRequest("GET", "/x", nil))
			defer resp.Body.Close()
			if resp.StatusCode != tc.wantStatus {
				b, _ := io.ReadAll(resp.Body)
				t.Fatalf("status = %d want %d (%s)", resp.StatusCode, tc.wantStatus, b)
			}
		})
	}
}

func TestRequireCompanyMembership(t *testing.T) {
	cases := []struct {
		name       string
		access     *membership.AccessResolution
		wantStatus int
	}{
		{"nil access passes through", nil, fiber.StatusOK},
		{"no access", &membership.AccessResolution{HasAccess: false}, fiber.StatusForbidden},
		{"banned", &membership.AccessResolution{HasAccess: true, IsBanned: true}, fiber.StatusForbidden},
		{"not activated", &membership.AccessResolution{HasAccess: true, IsActivated: false}, fiber.StatusForbidden},
		{"ok", &membership.AccessResolution{HasAccess: true, IsActivated: true}, fiber.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			app.Use(func(c *fiber.Ctx) error {
				if tc.access != nil {
					c.Locals("localAccess", tc.access)
				}
				return c.Next()
			})
			app.Get("/x", RequireCompanyMembership(), func(c *fiber.Ctx) error { return c.SendString("ok") })

			resp, _ := app.Test(httptest.NewRequest("GET", "/x", nil))
			defer resp.Body.Close()
			if resp.StatusCode != tc.wantStatus {
				b, _ := io.ReadAll(resp.Body)
				t.Fatalf("status = %d want %d (%s)", resp.StatusCode, tc.wantStatus, b)
			}
		})
	}
}
