package repository

import (
	"errors"
	"fmt"
	"log"

	"gorm.io/gorm"

	"duluin_invoice/app/model"
)

const mitraCodeBackfillKey = "mitra_code_v1"

// BackfillMitraCodesOnce gives every partner created before partner codes existed a code: MTR-0001,
// MTR-0002, … per company in creation order (soft-deleted ones included, so a code is never reused),
// continuing after any code the company already has. One transaction, recorded so it runs once.
func BackfillMitraCodesOnce(db *gorm.DB) error {
	var rec model.MigrationRecord
	err := db.Where("key = ?", mitraCodeBackfillKey).First(&rec).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("check partner code backfill record: %w", err)
	}

	filled := 0
	err = db.Transaction(func(tx *gorm.DB) error {
		var rows []model.Mitra
		if err := tx.Unscoped().Select("id", "company_id", "code").
			Where("code = '' OR code IS NULL").
			Order("company_id, created_at, id").
			Find(&rows).Error; err != nil {
			return fmt.Errorf("list partners without a code: %w", err)
		}
		next := map[string]int{}
		for _, m := range rows {
			if _, ok := next[m.CompanyID]; !ok {
				var codes []string
				if err := tx.Unscoped().Model(&model.Mitra{}).
					Where("company_id = ? AND upper(code) LIKE ?", m.CompanyID, mitraCodePrefix+"%").
					Pluck("code", &codes).Error; err != nil {
					return fmt.Errorf("scan partner codes: %w", err)
				}
				next[m.CompanyID] = nextMitraSequence(codes)
			}
			code := fmt.Sprintf("%s%04d", mitraCodePrefix, next[m.CompanyID])
			next[m.CompanyID]++
			if err := tx.Unscoped().Model(&model.Mitra{}).Where("id = ?", m.ID).
				UpdateColumn("code", code).Error; err != nil {
				return fmt.Errorf("set partner code: %w", err)
			}
			filled++
		}
		return tx.Create(&model.MigrationRecord{Key: mitraCodeBackfillKey}).Error
	})
	if err != nil {
		return err
	}
	log.Printf("✅ Partner code backfill applied: %d partner(s)", filled)
	return nil
}
