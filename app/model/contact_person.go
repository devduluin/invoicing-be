package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ContactPerson is a person at a partner (Mitra): a partner has many. Soft-deleted (deleted_at) so a
// document that used the contact keeps working; documents also store a snapshot of the contact's
// details (see the contact_* columns on sales/purchase orders and invoices).
//
// The partner's own "Contact Name (PIC)" is a company field and is NOT a contact person.
//
// A partial unique index (ensurePartialIndexes) refuses a duplicate active contact under a partner
// (same name + email + phone).
type ContactPerson struct {
	ID        string `gorm:"type:uuid;primaryKey"       json:"id"`
	CompanyID string `gorm:"type:uuid;not null;index"   json:"company_id"`
	MitraID   string `gorm:"type:uuid;not null;index"   json:"mitra_id"`
	Name      string `gorm:"type:varchar(255);not null" json:"name"`
	Position  string `gorm:"type:varchar(150)"          json:"position,omitempty"`
	Phone     string `gorm:"type:varchar(50)"           json:"phone,omitempty"`
	Email     string `gorm:"type:varchar(150)"          json:"email,omitempty"`

	CreatedAt time.Time      `gorm:"autoCreateTime"   json:"created_at"`
	CreatedBy string         `gorm:"type:varchar(64)" json:"created_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"   json:"updated_at"`
	UpdatedBy string         `gorm:"type:varchar(64)" json:"updated_by,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index"            json:"-"`
}

func (ContactPerson) TableName() string { return "contact_persons" }

func (c *ContactPerson) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}
