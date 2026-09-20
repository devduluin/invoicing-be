package model

import "time"

// MigrationRecord guards a one-time data backfill (as opposed to a schema
// migration, which AutoMigrate already handles) — same pattern as
// acc-master-service's asset-category backfill guard. A backfill checks for
// its own key before running and inserts it after, so it applies exactly
// once across the whole database regardless of how many times the server
// restarts.
type MigrationRecord struct {
	Key       string    `gorm:"type:varchar(100);primaryKey" json:"key"`
	AppliedAt time.Time `gorm:"autoCreateTime"               json:"applied_at"`
}

func (MigrationRecord) TableName() string { return "migration_records" }
