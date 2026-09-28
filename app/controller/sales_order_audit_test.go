package controller

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	audit "duluin_invoice/app/domain/audit"
	domain "duluin_invoice/app/domain/salesorder"
	"duluin_invoice/app/model"
)

type fakeSalesOrderSvc struct {
	domain.IService
	deleted []string
}

func (f *fakeSalesOrderSvc) Get(_, id string) (*model.SalesOrder, error) {
	return &model.SalesOrder{ID: id, Number: "SO/2026/0004"}, nil
}
func (f *fakeSalesOrderSvc) Delete(_, id string) error { f.deleted = append(f.deleted, id); return nil }

func TestSalesOrderDeleteIsAudited(t *testing.T) {
	l := &recLogger{}
	svc := &fakeSalesOrderSvc{}
	ctrl := NewSalesOrderController(svc, nil, l)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("companyID", "c1"); return c.Next() })
	app.Delete("/sales-orders/:id", ctrl.Delete)
	app.Post("/sales-orders/bulk-delete", ctrl.BulkDelete)

	id1, id2 := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	if _, err := app.Test(httptest.NewRequest("DELETE", "/sales-orders/"+id1, nil)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/sales-orders/bulk-delete", strings.NewReader(`{"ids":["`+id1+`","`+id2+`"]}`))
	req.Header.Set("Content-Type", "application/json")
	if _, err := app.Test(req); err != nil {
		t.Fatal(err)
	}
	if len(svc.deleted) != 3 {
		t.Fatalf("expected 3 deletes, got %v", svc.deleted)
	}
	if len(l.entries) != 3 {
		t.Fatalf("every delete must be audited, got %d entries: %+v", len(l.entries), l.entries)
	}
	for _, e := range l.entries {
		if e.Action != audit.ActionDeleted || e.Module != audit.ModuleSalesOrder || e.EntityName != "SO/2026/0004" {
			t.Errorf("bad entry %+v", e)
		}
	}
}
