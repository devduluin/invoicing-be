package middlewares

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"duluin_invoice/config"
)

func withSSO(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	prev := config.AppConfig.SSOURL
	config.AppConfig.SSOURL = srv.URL
	t.Cleanup(func() { config.AppConfig.SSOURL = prev })
}

const okBody = `{"success":true,"user":{"id":"u1","name":"A","email":"a@b.c","roles":[],"permissions":[]}}`

func TestValidateWithSSO_ClassifiesFailures(t *testing.T) {
	cases := []struct {
		name          string
		status        int
		body          string
		wantErr       bool
		wantTransient bool
	}{
		{"valid", http.StatusOK, okBody, false, false},
		{"sso says token dead", http.StatusUnauthorized, `{"message":"Invalid or expired token"}`, true, false},
		{"success false is a verdict", http.StatusOK, `{"success":false,"message":"nope"}`, true, false},
		{"sso 500", http.StatusInternalServerError, `oops`, true, true},
		{"sso 502", http.StatusBadGateway, ``, true, true},
		{"sso 429", http.StatusTooManyRequests, ``, true, true},
		{"unknown account type 404", http.StatusNotFound, ``, true, true},
		{"bad json", http.StatusOK, `<html>`, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withSSO(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})
			_, err := validateWithSSO("Bearer x", "", "duluin_invoice", "t")
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if err != nil && isTransientSSOError(err) != tc.wantTransient {
				t.Fatalf("transient=%v want %v (err=%v)", isTransientSSOError(err), tc.wantTransient, err)
			}
		})
	}
}

func TestValidateWithSSO_RetriesTransientOnce(t *testing.T) {
	var calls int32
	withSSO(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(okBody))
	})
	data, err := validateWithSSO("Bearer x", "", "duluin_invoice", "t")
	if err != nil || data == nil {
		t.Fatalf("expected success after one retry, got err=%v", err)
	}
	if calls != 2 {
		t.Fatalf("calls=%d want 2", calls)
	}
}

func TestValidateWithSSO_DoesNotRetryDeadToken(t *testing.T) {
	var calls int32
	withSSO(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusUnauthorized)
	})
	if _, err := validateWithSSO("Bearer x", "", "duluin_invoice", "t"); err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Fatalf("calls=%d want 1 (a 401 must not be retried)", calls)
	}
}

func TestValidateWithSSO_CapturesReissuedToken(t *testing.T) {
	withSSO(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"token":"NEW","token_reissued":true,"user":{"id":"u1"}}`))
	})
	data, err := validateWithSSO("Bearer old", "", "duluin_invoice", "t")
	if err != nil || data.ReissuedToken != "NEW" {
		t.Fatalf("data=%+v err=%v", data, err)
	}
}

// Regression for the "every API call takes ~20s" incident: a page fires several requests at once
// with the same token, and with Redis down each used to make its OWN SSO round-trip — one slow SSO
// reply stalled all of them (10s timeout x 2 attempts = ~20s). Now concurrent identical validations
// share one SSO call, and the verdict is cached process-locally for the same TTL as Redis.
func TestValidateToken_CoalescesConcurrentAndCachesWithoutRedis(t *testing.T) {
	var calls int32
	withSSO(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(150 * time.Millisecond) // a slow-ish SSO, long enough for the requests to overlap
		_, _ = w.Write([]byte(okBody))
	})
	app := fiber.New()
	app.Use(ValidateTokenForAccount("duluin_invoice"))
	app.Get("/x", func(c *fiber.Ctx) error { return c.SendStatus(http.StatusOK) })

	fire := func(n int) {
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req := httptest.NewRequest(http.MethodGet, "/x", nil)
				req.Header.Set("Authorization", "Bearer coalesce-test-token")
				resp, err := app.Test(req, 5000)
				if err != nil || resp.StatusCode != http.StatusOK {
					t.Errorf("request failed: err=%v", err)
				}
			}()
		}
		wg.Wait()
	}

	fire(8)
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("8 concurrent requests made %d SSO calls, want 1", got)
	}
	fire(8) // token now cached: no further SSO trips
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("cached follow-up requests made SSO calls: total %d, want 1", got)
	}
}
