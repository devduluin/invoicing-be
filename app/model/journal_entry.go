package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// JournalEntryStatus mirrors accounting-engine-service's draft/posted
// workflow (its "cancel" status isn't ported — no reversal/void flow here yet).
type JournalEntryStatus string

const (
	JournalEntryStatusDraft  JournalEntryStatus = "draft"
	JournalEntryStatusPosted JournalEntryStatus = "posted"
)

// JournalEntry is a manual double-entry journal ("Ayat Jurnal") — the first
// genuinely transactional record in invoice-service. TotalDebit/TotalCredit
// are computed server-side from Lines on every write (never trusted from the
// client) and cached here purely so the list view doesn't need to aggregate
// journal_lines per row.
type JournalEntry struct {
	ID             string             `gorm:"type:uuid;primaryKey"       json:"id"`
	CompanyID      string             `gorm:"type:uuid;not null;index"   json:"company_id"`
	JournalBookID  string             `gorm:"type:uuid;not null;index"   json:"journal_book_id"`
	Number         string             `gorm:"type:varchar(50);not null"  json:"number"`
	Description    string             `gorm:"type:varchar(255);not null" json:"description"`
	Date           time.Time          `gorm:"type:date;not null"         json:"date"`
	Status         JournalEntryStatus `gorm:"type:varchar(20);not null;default:'draft';index" json:"status"`
	IdempotencyKey string             `gorm:"type:varchar(100)"          json:"idempotency_key,omitempty"`
	TotalDebit     float64            `gorm:"type:numeric(18,2);not null;default:0" json:"total_debit"`
	TotalCredit    float64            `gorm:"type:numeric(18,2);not null;default:0" json:"total_credit"`

	Lines []JournalLine `gorm:"foreignKey:JournalEntryID" json:"lines,omitempty"`

	CreatedAt time.Time      `gorm:"autoCreateTime"   json:"created_at"`
	CreatedBy string         `gorm:"type:varchar(64)" json:"created_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"   json:"updated_at"`
	UpdatedBy string         `gorm:"type:varchar(64)" json:"updated_by,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index"            json:"-"`
}

func (JournalEntry) TableName() string { return "journal_entries" }

func (j *JournalEntry) BeforeCreate(tx *gorm.DB) error {
	if j.ID == "" {
		j.ID = uuid.New().String()
	}
	if j.Status == "" {
		j.Status = JournalEntryStatusDraft
	}
	return nil
}

// JournalLine is one debit/credit row of a JournalEntry. Exactly one of
// Debit/Credit is > 0 (enforced in the service layer, not the DB) per the
// double-entry rule the reference UI itself follows.
type JournalLine struct {
	ID             string  `gorm:"type:uuid;primaryKey"     json:"id"`
	JournalEntryID string  `gorm:"type:uuid;not null;index" json:"journal_entry_id"`
	CompanyID      string  `gorm:"type:uuid;not null;index" json:"company_id"`
	AccountID      string  `gorm:"type:uuid;not null;index" json:"account_id"`
	MitraID        *string `gorm:"type:uuid;index"          json:"mitra_id,omitempty"`
	Description    string  `gorm:"type:varchar(255)"        json:"description,omitempty"`
	Debit          float64 `gorm:"type:numeric(18,2);not null;default:0" json:"debit"`
	Credit         float64 `gorm:"type:numeric(18,2);not null;default:0" json:"credit"`
	LineOrder      int     `gorm:"not null;default:0"       json:"line_order"`
}

func (JournalLine) TableName() string { return "journal_lines" }

func (l *JournalLine) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	return nil
}
