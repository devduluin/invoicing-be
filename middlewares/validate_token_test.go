package middlewares

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

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
