package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"duluin_invoice/utils"
)

// BankAccount is a company's receiving bank account, shown to buyers on the
// public invoice link for manual transfer (PRD §11). Simplified vs acc-master —
// no COA/journal binding in Phase 1.
type BankAccount struct {
	ID            string        `gorm:"type:uuid;primaryKey"             json:"id"`
	CompanyID     string        `gorm:"type:uuid;not null;index"         json:"company_id"`
	BankName      string        `gorm:"type:varchar(120);not null"       json:"bank_name"`
	BankCode      string        `gorm:"type:varchar(10)"                 json:"bank_code,omitempty"` // from the SSO bank directory (/api/meta/bank)
	AccountNumber string        `gorm:"type:varchar(60);not null"        json:"account_number"`
	AccountHolder string        `gorm:"type:varchar(255);not null"       json:"account_holder"`
	Branch        string        `gorm:"type:varchar(120)"                json:"branch,omitempty"`
	IsPrimary     utils.BoolInt `gorm:"type:smallint;not null;default:0" json:"is_primary"`
	IsActive      utils.BoolInt `gorm:"type:smallint;not null;default:1" json:"is_active"`

	CreatedAt time.Time      `gorm:"autoCreateTime"   json:"created_at"`
	CreatedBy string         `gorm:"type:varchar(64)" json:"created_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"   json:"updated_at"`
	UpdatedBy string         `gorm:"type:varchar(64)" json:"updated_by,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index"            json:"-"`
}

func (BankAccount) TableName() string { return "bank_accounts" }

func (b *BankAccount) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return nil
}
