package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"duluin_invoice/utils"
)

// Salesperson is a company's sales person master data, picked on Sales Orders and Sales Invoices.
// Not every salesperson is a user of the app (field sales, agents, people who left), so it is its
// own record; UserID optionally links it to a team member (the SSO user id of a user_account_sso
// row), one salesperson per member (uq_salespersons_company_user_active).
//
// Code is unique among the company's active salespersons (SLS-0001…, generated when left blank).
// Documents keep the salesperson's id and a snapshot of its name, so renaming or deactivating a
// salesperson never changes an issued document.
type Salesperson struct {
	ID        string        `gorm:"type:uuid;primaryKey"                 json:"id"`
	CompanyID string        `gorm:"type:uuid;not null;index"             json:"company_id"`
	Code      string        `gorm:"type:varchar(50);not null;default:''" json:"code"`
	Name      string        `gorm:"type:varchar(120);not null"           json:"name"`
	Email     string        `gorm:"type:varchar(150)"                    json:"email,omitempty"`
	Phone     string        `gorm:"type:varchar(50)"                     json:"phone,omitempty"`
	UserID    string        `gorm:"type:varchar(64);not null;default:''" json:"user_id,omitempty"`
	IsActive  utils.BoolInt `gorm:"type:smallint;not null;default:1"     json:"is_active"`

	CreatedAt time.Time      `gorm:"autoCreateTime"   json:"created_at"`
	CreatedBy string         `gorm:"type:varchar(64)" json:"created_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"   json:"updated_at"`
	UpdatedBy string         `gorm:"type:varchar(64)" json:"updated_by,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index"            json:"-"`
}

func (Salesperson) TableName() string { return "salespersons" }

func (s *Salesperson) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}
