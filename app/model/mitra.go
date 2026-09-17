package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"duluin_invoice/utils"
)

// MitraType — a mitra can act as customer, supplier, or both (PRD §8 / §17 ERD:
// MITRA sebagai_customer / sebagai_supplier).
type MitraType string

const (
	MitraTypeCustomer MitraType = "customer"
	MitraTypeSupplier MitraType = "supplier"
	MitraTypeBoth     MitraType = "both"
)

func IsValidMitraType(v string) bool {
	switch MitraType(v) {
	case MitraTypeCustomer, MitraTypeSupplier, MitraTypeBoth:
		return true
	default:
		return false
	}
}

// Mitra is the master-data record for a business relation (PRD §8). Multi-company
// via CompanyID (the local Company UUID, NFR §18). Monetary columns on future
// models MUST use numeric, never float (PRD §3) — this model has none.
type Mitra struct {
	ID          string        `gorm:"type:uuid;primaryKey"                         json:"id"`
	CompanyID   string        `gorm:"type:uuid;not null;index"                     json:"company_id"`
	Type        MitraType     `gorm:"type:varchar(20);not null;default:'customer'" json:"type"`
	Name        string        `gorm:"type:varchar(255);not null"                   json:"name"`
	ContactName string        `gorm:"type:varchar(255)"                            json:"contact_name,omitempty"`
	Email       string        `gorm:"type:varchar(150)"                            json:"email,omitempty"`
	Phone       string        `gorm:"type:varchar(50)"                             json:"phone,omitempty"`
	Npwp        string        `gorm:"type:varchar(50)"                             json:"npwp,omitempty"`
	Address     string        `gorm:"type:text"                                    json:"address,omitempty"`
	IsActive    utils.BoolInt `gorm:"type:smallint;not null;default:1"             json:"is_active"`

	CreatedAt time.Time      `gorm:"autoCreateTime"        json:"created_at"`
	CreatedBy string         `gorm:"type:varchar(64)"      json:"created_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"        json:"updated_at"`
	UpdatedBy string         `gorm:"type:varchar(64)"      json:"updated_by,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index"                 json:"-"`
}

func (Mitra) TableName() string { return "mitra" }

func (m *Mitra) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}
