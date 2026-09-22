// Package notification defines the transactional-email boundary. PRD §3 lists
// SendGrid / AWS SES / SMTP as options; this package stays provider-agnostic so
// the rest of the app depends only on Service.
package notification

import (
	"context"
	"fmt"
	"log"
)

// UserInvite is the payload for a team invitation email (PRD §4 Step 4).
type UserInvite struct {
	ToEmail     string
	ToName      string
	InviterName string
	TenantName  string
	Role        string
	AcceptURL   string
}

// Service is the transactional-notification port. Implementations: LogService
// (dev stub), and later a real SendGrid/SES sender.
type Service interface {
	SendUserInvite(ctx context.Context, msg UserInvite) (string, error)
}

// LogService is a no-op implementation that logs instead of sending. Used until
// a real provider is configured (PRD §3). It never errors, so onboarding is not
// blocked by mail delivery in dev.
type LogService struct{}

func NewLogService() *LogService { return &LogService{} }

func (s *LogService) SendUserInvite(_ context.Context, msg UserInvite) (string, error) {
	log.Printf("[notification] (stub) invite email → %s <%s> | tenant=%q role=%s accept=%s",
		msg.ToName, msg.ToEmail, msg.TenantName, msg.Role, msg.AcceptURL)
	return msg.AcceptURL, nil
}

// SSOInviter is the subset of *sso.Client this package depends on — kept as
// a plain-args interface (not sso.InviteRequest) so notification never
// imports app/sso's full surface.
type SSOInviter interface {
	InviteSimple(ctx context.Context, email, name, redirectURL, inviterName, companyName, from string) (string, error)
}

// SSOInviteService sends the invite email through SSO's own POST /invite,
// passing our own AcceptURL as redirect_url so SSO's email points at this
// product's existing accept-invite flow instead of SSO's own /accept-invite
// page. SSO creates/links the shadow user as a side effect; invoicing-be's
// own UserAccountSSO row (and its InviteToken) remains the source of truth.
type SSOInviteService struct {
	sso  SSOInviter
	from string
}

func NewSSOInviteService(sso SSOInviter, from string) *SSOInviteService {
	return &SSOInviteService{sso: sso, from: from}
}

func (s *SSOInviteService) SendUserInvite(ctx context.Context, msg UserInvite) (string, error) {
	url, err := s.sso.InviteSimple(ctx, msg.ToEmail, msg.ToName, msg.AcceptURL, msg.InviterName, msg.TenantName, s.from)
	if err != nil {
		return "", fmt.Errorf("send invite via sso: %w", err)
	}
	return url, nil
}
