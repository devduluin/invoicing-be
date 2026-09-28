package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func newBulkDeleteTestApp(fail map[string]string) *fiber.App {
	app := fiber.New()
	app.Post("/bulk-delete", func(c *fiber.Ctx) error {
		ids, ok := parseBulkIDs(c)
		if !ok {
			return nil
		}
		return runBulkDelete(c, ids, func(id string) error {
			if msg, bad := fail[id]; bad {
				return errors.New(msg)
			}
			return nil
		})
	})
	return app
}

func postJSON(t *testing.T, app *fiber.App, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/bulk-delete", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// One request deletes several documents; a row still referenced (or otherwise rejected) fails on
// its own without blocking the rest of the batch — matching the per-row guard every single-delete
// endpoint already enforces (this handler calls the exact same Delete, just once per id).
func TestBulkDelete_PartialFailureDoesNotBlockTheRest(t *testing.T) {
	idOK1 := uuid.NewString()
	idBad := uuid.NewString()
	idOK2 := uuid.NewString()
	app := newBulkDeleteTestApp(map[string]string{idBad: "this document is still referenced"})

	resp := postJSON(t, app, `{"ids":["`+idOK1+`","`+idBad+`","`+idOK2+`"]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var body struct {
		Data []BulkResult `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 3 {
		t.Fatalf("want 3 results, got %d", len(body.Data))
	}
	byID := map[string]BulkResult{}
	for _, r := range body.Data {
		byID[r.ID] = r
	}
	if !byID[idOK1].Success || !byID[idOK2].Success {
		t.Fatalf("both unaffected ids should succeed: %+v", body.Data)
	}
	if byID[idBad].Success || !strings.Contains(byID[idBad].Message, "still referenced") {
		t.Fatalf("the referenced id should fail with its reason: %+v", byID[idBad])
	}
}

func TestBulkDelete_RejectsEmptyOrOversizedRequests(t *testing.T) {
	app := newBulkDeleteTestApp(nil)

	if resp := postJSON(t, app, `{"ids":[]}`); resp.StatusCode == http.StatusOK {
		t.Fatal("an empty id list must be rejected")
	}

	ids := make([]string, 101)
	for i := range ids {
		ids[i] = uuid.NewString()
	}
	b, _ := json.Marshal(map[string][]string{"ids": ids})
	if resp := postJSON(t, app, string(b)); resp.StatusCode == http.StatusOK {
		t.Fatal("more than 100 ids in one request must be rejected")
	}
}
