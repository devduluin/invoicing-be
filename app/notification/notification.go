// Package notification defines the transactional-email boundary. PRD §3 lists
// SendGrid / AWS SES / SMTP as options; this package stays provider-agnostic so
// the rest of the app depends only on Service.
package notification

import (
	"context"
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
	SendUserInvite(ctx context.Context, msg UserInvite) error
}

// LogService is a no-op implementation that logs instead of sending. Used until
// a real provider is configured (PRD §3). It never errors, so onboarding is not
// blocked by mail delivery in dev.
type LogService struct{}

func NewLogService() *LogService { return &LogService{} }

func (s *LogService) SendUserInvite(_ context.Context, msg UserInvite) error {
	log.Printf("[notification] (stub) invite email → %s <%s> | tenant=%q role=%s accept=%s",
		msg.ToName, msg.ToEmail, msg.TenantName, msg.Role, msg.AcceptURL)
	return nil
}
