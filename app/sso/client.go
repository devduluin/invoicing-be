// Package sso is the thin outbound client for the Duluin SSO service. It carries
// no business logic — callers decide what to sync and when. Role/permission
// definitions live in SSO; this client is the catalog + assignment sink, same as
// acc-master-service's utils/crossService.go.
package sso

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"duluin_invoice/utils"
)

type Client struct {
	baseURL     string
	accountType string
	apiKey      string
	http        *http.Client
}

func NewClient(baseURL, accountType, apiKey string) *Client {
	return &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		accountType: accountType,
		apiKey:      apiKey,
		http:        utils.NewOutboundHTTPClient(10 * time.Second),
	}
}

// ── assignment sink (public SSO endpoints, no auth) ─────────────────────────

// AssignRole sets the SSO role for the user on this product's account type
// (replace semantics). Coarse — SSO has no company dimension on model_has_roles;
// the authoritative per-company role is user_account_sso.role_id locally.
func (c *Client) AssignRole(ctx context.Context, ssoUserID, roleName string) error {
	if strings.TrimSpace(roleName) == "" {
		return fmt.Errorf("assign role: missing role name")
	}
	return c.postForm(ctx, "/users/register/assign_role", map[string]any{
		"user_id":   ssoUserID,
		"role_name": roleName,
	})
}

// SetSecondaryID mirrors the local company id onto user_accounts.secondary_id so
// any service resolves the company from a plain SSO token (acc-master pattern).
func (c *Client) SetSecondaryID(ctx context.Context, ssoUserID, secondaryID string) error {
	if strings.TrimSpace(secondaryID) == "" {
		return fmt.Errorf("set secondary id: missing secondary id")
	}
	return c.postForm(ctx, "/users/register/set_secondary_id", map[string]any{
		"user_id":      ssoUserID,
		"secondary_id": secondaryID,
	})
}

// Invite calls SSO's generic POST /invite, which creates a shadow SSO user
// for a brand-new email (or links an existing one) and sends its own
// transactional email. RedirectURL, when set, is honored by SSO as the
// literal accept-link the email points to (instead of SSO's own
// /accept-invite construction) — see the invite controller's redirect_url
// handling.
type InviteRequest struct {
	Email       string
	Name        string
	CompanyID   string
	RedirectURL string
	InviterName string
	CompanyName string
	From        string
}

type InviteResult struct {
	Email string `json:"email"`
	IsNew bool   `json:"is_new"`
	URL   string `json:"url"`
}

func (c *Client) Invite(ctx context.Context, req InviteRequest) (*InviteResult, error) {
	if strings.TrimSpace(req.Email) == "" {
		return nil, fmt.Errorf("sso invite: missing email")
	}
	payload := map[string]any{
		"email":        req.Email,
		"name":         req.Name,
		"account_type": c.accountType,
		"company_id":   req.CompanyID,
		"redirect_url": req.RedirectURL,
		"inviter_name": req.InviterName,
		"company_name": req.CompanyName,
	}
	if req.From != "" {
		payload["from"] = req.From
	}
	raw, err := c.do(ctx, http.MethodPost, "/invite", "", payload)
	if err != nil {
		return nil, err
	}
	var env struct {
		Data InviteResult `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("sso invite: decode response: %w", err)
	}
	return &env.Data, nil
}

// InviteSimple adapts Invite to notification.SSOInviter's plain-args shape.
func (c *Client) InviteSimple(ctx context.Context, email, name, redirectURL, inviterName, companyName, from string) error {
	_, err := c.Invite(ctx, InviteRequest{
		Email:       email,
		Name:        name,
		RedirectURL: redirectURL,
		InviterName: inviterName,
		CompanyName: companyName,
		From:        from,
	})
	return err
}

// UserExists calls SSO's POST /auth/user-validation to check whether an
// email already has an account for this product. A 404 means "not
// registered" — a normal outcome, not a failure.
func (c *Client) UserExists(ctx context.Context, email string) (bool, error) {
	_, err := c.do(ctx, http.MethodPost, "/auth/user-validation", "", map[string]any{"email": email})
	if err == nil {
		return true, nil
	}
	var ssoErr *Error
	if as(err, &ssoErr) && ssoErr.Status == http.StatusNotFound {
		return false, nil
	}
	return false, err
}

func (c *Client) postForm(ctx context.Context, path string, payload map[string]any) error {
	if s, _ := payload["user_id"].(string); strings.TrimSpace(s) == "" {
		return fmt.Errorf("sso %s: missing user id", path)
	}
	_, err := c.do(ctx, http.MethodPost, path, "", payload)
	return err
}

// do issues a JSON request. token = request bearer; empty → API_KEY fallback.
func (c *Client) do(ctx context.Context, method, path, token string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("sso %s: build request: %w", path, err)
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Account-Type", c.accountType)
	if auth := bearer(token, c.apiKey); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &Error{Path: path, Message: err.Error(), Temporary: true}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return raw, &Error{
			Path:      path,
			Status:    resp.StatusCode,
			Message:   strings.TrimSpace(string(raw)),
			Temporary: resp.StatusCode == 429 || resp.StatusCode >= 500,
		}
	}
	return raw, nil
}

func bearer(token, apiKey string) string {
	if t := strings.TrimSpace(token); t != "" {
		if strings.HasPrefix(t, "Bearer ") {
			return t
		}
		return "Bearer " + t
	}
	if apiKey != "" {
		return "Bearer " + apiKey
	}
	return ""
}

// Error is an SSO call failure; Temporary drives one-shot retry in the caller.
type Error struct {
	Path      string
	Status    int
	Message   string
	Temporary bool
}

func (e *Error) Error() string {
	return fmt.Sprintf("sso %s: status=%d %s", e.Path, e.Status, e.Message)
}

func IsTemporary(err error) bool {
	var e *Error
	if ok := as(err, &e); ok {
		return e.Temporary
	}
	return false
}

func as(err error, target **Error) bool {
	for err != nil {
		if e, ok := err.(*Error); ok {
			*target = e
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
