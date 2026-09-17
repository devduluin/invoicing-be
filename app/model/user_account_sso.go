package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SSO role names for the 3 seeded global roles (DuluinInvoiceSeeder).
const (
	SSORoleOwner  = "Invoice Owner"
	SSORoleAdmin  = "Invoice Admin"
	SSORoleViewer = "Invoice Viewer"
)

// UserAccountSSO is the membership table — it maps an SSO user to a Company with
// a role. Same name + columns as acc-master-service (app/model/sso.go), so the
// two products stay recognisable. `RoleID` is an SSO role UUID (global or
// company-scoped custom); permissions are resolved from SSO per request.
//
// For invite-by-email (invitee not yet an SSO user) `UserID` is empty and
// `Email`/`Name` carry the pending identity until AcceptInvite fills `UserID`.
type UserAccountSSO struct {
	ID           string     `gorm:"type:uuid;primaryKey"     json:"id"`
	UserID       string     `gorm:"type:varchar(64);index"   json:"user_id"`
	CompanyID    string     `gorm:"type:uuid;not null;index" json:"company_id"`
	SecondaryID  *string    `gorm:"type:uuid;index"          json:"secondary_id,omitempty"`
	RoleID       *string    `gorm:"type:uuid;index"          json:"role_id,omitempty"`
	IsActivated  bool       `gorm:"not null;default:false"   json:"is_activated"`
	IsBanned     bool       `gorm:"not null;default:false"   json:"is_banned"`
	BannedReason string     `gorm:"type:varchar(255)"        json:"banned_reason,omitempty"`
	ActivatedAt  *time.Time `                               json:"activated_at,omitempty"`

	// Pending-invite identity (denormalized; acc-master creates a shadow users_sso
	// row instead — invoice keeps it light).
	Email       string     `gorm:"type:varchar(150);index" json:"email,omitempty"`
	Name        string     `gorm:"type:varchar(255)"       json:"name,omitempty"`
	InviteToken *string    `gorm:"type:varchar(64);index"  json:"-"`
	InvitedBy   *string    `gorm:"type:varchar(64);index"  json:"invited_by,omitempty"`
	InvitedAt   *time.Time `                              json:"invited_at,omitempty"`

	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index"          json:"-"`

	Company *Company `gorm:"foreignKey:CompanyID" json:"company,omitempty"`
}

func (UserAccountSSO) TableName() string { return "user_account_sso" }

func (u *UserAccountSSO) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}

// Pending reports an invite that has not been accepted yet.
func (u *UserAccountSSO) Pending() bool { return !u.IsActivated && u.InvitedAt != nil }
