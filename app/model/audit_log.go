package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditLog is one append-only record of a significant action taken in a company. Rows are never
// updated or deleted by the app: there is no repository method to do either (see
// app/domain/audit.IRepository), and this model deliberately has no gorm.DeletedAt — nothing here
// is ever soft-deleted either.
type AuditLog struct {
	ID          string `gorm:"type:uuid;primaryKey"          json:"id"`
	CompanyID   string `gorm:"type:uuid;not null;index"      json:"company_id"`
	ActorUserID string `gorm:"type:varchar(64);index"        json:"actor_user_id,omitempty"`
	ActorName   string `gorm:"type:varchar(255);not null"    json:"actor_name"`
	ActorEmail  string `gorm:"type:varchar(150)"             json:"actor_email,omitempty"`
	Action      string `gorm:"type:varchar(40);not null;index" json:"action"`
	Module      string `gorm:"type:varchar(40);not null;index" json:"module"`
	EntityType  string `gorm:"type:varchar(60)"              json:"entity_type,omitempty"`
	EntityID    string `gorm:"type:varchar(64);index"        json:"entity_id,omitempty"`
	EntityName  string `gorm:"type:varchar(255)"             json:"entity_name,omitempty"`
	Description string `gorm:"type:text;not null"            json:"description"`
	// Changes is a JSON object of {field: {before, after}} — see app/domain/audit.Change. Stored as
	// jsonb (Go string), same convention as DocumentConfiguration.Config.
	Changes   string    `gorm:"type:jsonb"              json:"-"`
	IPAddress string    `gorm:"type:varchar(64)"        json:"ip_address,omitempty"`
	UserAgent string    `gorm:"type:varchar(255)"       json:"user_agent,omitempty"`
	CreatedAt time.Time `gorm:"autoCreateTime;index"    json:"created_at"`
}

func (AuditLog) TableName() string { return "audit_logs" }

func (a *AuditLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}
