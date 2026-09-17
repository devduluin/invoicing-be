package utils

import (
	"net/http"
	"time"
)

const defaultOutboundHTTPTimeout = 10 * time.Second

// NewOutboundHTTPClient returns an HTTP client for calling upstream services (SSO).
// HTTP/2 stays enabled — SSO negotiates h2 via ALPN.
func NewOutboundHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = defaultOutboundHTTPTimeout
	}
	return &http.Client{Timeout: timeout}
}
