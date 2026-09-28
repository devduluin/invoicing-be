package controller

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	audit "duluin_invoice/app/domain/audit"
)

type recLogger struct{ entries []audit.Entry }

func (r *recLogger) Log(_ audit.Actor, e audit.Entry) { r.entries = append(r.entries, e) }

type acct struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

func auditApp(l *recLogger) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("company_id", "c1"); return c.Next() })
	app.Use("/accounts", AuditWrites(l, AuditSpec{
		Module: audit.ModuleAccounting, Entity: "account", Label: "account",
		Lookup: func(_ *fiber.Ctx, _ string, ids []string) any { return &acct{ID: ids[len(ids)-1], Code: "1-1000", Name: "Kas"} },
	}))
	const id = "11111111-1111-4111-8111-111111111111"
	app.Post("/accounts", func(c *fiber.Ctx) error {
		return c.Status(201).JSON(fiber.Map{"success": true, "data": acct{ID: id, Code: "1-2000", Name: "Bank"}})
	})
	app.Put("/accounts/:id", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true, "data": acct{ID: id, Code: "1-1000", Name: "Kas Besar"}})
	})
	app.Delete("/accounts/:id", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"success": true}) })
	app.Post("/accounts/fail", func(c *fiber.Ctx) error { return c.Status(422).JSON(fiber.Map{"success": false}) })
	app.Get("/accounts", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"success": true}) })
	return app
}

func do(t *testing.T, app *fiber.App, method, path string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	if _, err := app.Test(req); err != nil {
		t.Fatal(err)
	}
}

func TestAuditWrites(t *testing.T) {
	l := &recLogger{}
	app := auditApp(l)
	const id = "11111111-1111-4111-8111-111111111111"

	do(t, app, "GET", "/accounts")
	do(t, app, "POST", "/accounts/fail")
	if len(l.entries) != 0 {
		t.Fatalf("reads and failed writes must not be recorded, got %+v", l.entries)
	}

	do(t, app, "POST", "/accounts")
	do(t, app, "PUT", "/accounts/"+id)
	do(t, app, "DELETE", "/accounts/"+id)
	if len(l.entries) != 3 {
		t.Fatalf("want 3 entries, got %d: %+v", len(l.entries), l.entries)
	}
	created, updated, deleted := l.entries[0], l.entries[1], l.entries[2]
	if created.Action != audit.ActionCreated || created.EntityName != "1-2000 Bank" || created.Description != "Created account 1-2000 Bank" || created.EntityID != id {
		t.Errorf("create entry wrong: %+v", created)
	}
	if updated.Action != audit.ActionUpdated || updated.EntityName != "1-1000 Kas Besar" {
		t.Errorf("update entry wrong: %+v", updated)
	}
	if ch, ok := updated.Changes["name"]; !ok || ch.Before != "Kas" || ch.After != "Kas Besar" {
		t.Errorf("update must show what changed: %+v", updated.Changes)
	}
	if _, ok := updated.Changes["code"]; ok {
		t.Error("unchanged field reported")
	}
	// a delete has no response body: the name comes from the record looked up beforehand
	if deleted.Action != audit.ActionDeleted || deleted.EntityName != "1-1000 Kas" || deleted.EntityID != id {
		t.Errorf("delete entry wrong: %+v", deleted)
	}
}

func TestAuditWritesMatch(t *testing.T) {
	l := &recLogger{}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("company_id", "c1"); return c.Next() })
	app.Use("/docs", AuditWrites(l, AuditSpec{
		Module: audit.ModuleSalesOrder, Entity: "sales_order", Label: "sales order",
		Match: func(_, p string) bool { return strings.HasSuffix(p, "/template") },
	}))
	app.Put("/docs/:id/template", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"id": "22222222-2222-4222-8222-222222222222", "number": "SO/1"}})
	})
	app.Put("/docs/:id", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"success": true}) })

	do(t, app, "PUT", "/docs/22222222-2222-4222-8222-222222222222")
	if len(l.entries) != 0 {
		t.Fatal("a route outside Match must not be recorded here (its controller records it)")
	}
	do(t, app, "PUT", "/docs/22222222-2222-4222-8222-222222222222/template")
	if len(l.entries) != 1 || l.entries[0].Description != "Changed the template of sales order SO/1" {
		t.Fatalf("template change not recorded: %+v", l.entries)
	}
}

func TestAuditWritesActionMapping(t *testing.T) {
	cases := []struct {
		method, path, action string
	}{
		{"POST", "/members/invite", audit.ActionInvitedUser},
		{"PATCH", "/members/x/role", audit.ActionRoleChanged},
		{"PATCH", "/members/x", audit.ActionUpdated},
		{"DELETE", "/members/x", audit.ActionDeleted},
		{"POST", "/members/x/resend", audit.ActionStatusChanged},
		{"POST", "/journal-entries/x/post", audit.ActionStatusChanged},
		{"POST", "/purchase-orders/x/cancel", audit.ActionCancelled},
		{"PUT", "/sales-orders/x/template", audit.ActionUpdated},
		{"POST", "/accounts", audit.ActionCreated},
	}
	for _, tc := range cases {
		got, _, ok := auditWritesAction(tc.method, tc.path)
		if !ok || got != tc.action {
			t.Errorf("%s %s: got %q ok=%v, want %q", tc.method, tc.path, got, ok, tc.action)
		}
	}
}
