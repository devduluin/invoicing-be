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
//
// Code is the partner's own unique code within the company (MTR-0001…), generated when left blank.
// Names may repeat (branches, namesakes); the code is what tells two partners apart, e.g. in
// imports. Unique among active partners (uq_mitra_company_code_active).
//
// LinkedCompanyID / LinkedCompanyCode link the partner to its own Duluin Invoice company (the
// shareable company code, e.g. K7NQ4P) when it has one — the basis for sending documents straight to
// the partner's account later. Optional; one partner per linked company (uq_mitra_company_linked_active).
// The code is a snapshot (a company's code never changes) so lists and imports need no join.
type Mitra struct {
	ID        string `gorm:"type:uuid;primaryKey"                         json:"id"`
	CompanyID string `gorm:"type:uuid;not null;index"                     json:"company_id"`
	Code      string `gorm:"type:varchar(50);not null;default:''"          json:"code"`

	LinkedCompanyID   *string `gorm:"type:uuid"                           json:"linked_company_id,omitempty"`
	LinkedCompanyCode string  `gorm:"type:varchar(20);not null;default:''" json:"linked_company_code,omitempty"`

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
