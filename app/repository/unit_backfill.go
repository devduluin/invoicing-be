package repository

import (
	"errors"
	"fmt"
	"log"

	"gorm.io/gorm"

	"duluin_invoice/app/model"
)

// unitSeedBackfillKey guards BackfillUnitsOnce so it runs exactly once across
// the whole database. Bump the suffix if the seed data changes again and
// every company should re-check (existing rows are never touched — this only
// tops up companies that currently have zero units).
const unitSeedBackfillKey = "v1_unit_seed_20260918"

// BackfillUnitsOnce seeds Unit defaults for every company that doesn't have
// any yet. Unit's seeder only runs automatically on new-company onboarding
// (MasterDefaultsService) or lazily on first GET /units (UnitService's
// self-heal) — neither of which reaches a company created before Unit
// existed, or one whose users can't yet call /units because the
// invoice-unit-* permissions aren't granted. This one-time backfill (same
// guard pattern as acc-master-service's asset-category backfill) closes that
// gap without waiting on either.
func BackfillUnitsOnce(db *gorm.DB) error {
	var rec model.MigrationRecord
	err := db.Where("key = ?", unitSeedBackfillKey).First(&rec).Error
	if err == nil {
		return nil // already applied
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("check unit backfill record: %w", err)
	}

	var companyIDs []string
	if err := db.Model(&model.Company{}).Pluck("id", &companyIDs).Error; err != nil {
		return fmt.Errorf("list companies for unit backfill: %w", err)
	}

	repo := NewUnitRepository(db)
	seeded := 0
	for _, companyID := range companyIDs {
		if err := repo.SeedDefaults(companyID, "system"); err != nil {
			return fmt.Errorf("backfill units for company %s: %w", companyID, err)
		}
		seeded++
	}

	if err := db.Create(&model.MigrationRecord{Key: unitSeedBackfillKey}).Error; err != nil {
		return fmt.Errorf("record unit backfill: %w", err)
	}
	log.Printf("✅ Unit backfill applied: checked %d compan(y/ies)", seeded)
	return nil
}
