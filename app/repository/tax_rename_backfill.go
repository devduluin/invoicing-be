package repository

import (
	"errors"
	"fmt"
	"log"

	"gorm.io/gorm"

	"duluin_invoice/app/model"
)

const taxIndonesianNamesBackfillKey = "tax_indonesian_names_v1"

// legacyTaxNames maps the English names DefaultTaxes() used to seed to the Indonesian names
// (same as acc-master-service) it seeds now. Seeding only ever runs for a NEW company, so
// companies created before the rename still carry the English rows — this fixes those in place.
var legacyTaxNames = map[string]string{
	"VAT 11%":                                           "PPN 11%",
	"VAT 0% Export":                                     "PPN 0% Ekspor",
	"WHT Art. 23 Services (2%)":                         "PPh 23 Jasa (2%)",
	"WHT Art. 23 Rent (2%)":                             "PPh 23 Sewa (2%)",
	"WHT Art. 23 Royalty (15%)":                         "PPh 23 Royalti (15%)",
	"WHT Art. 23 Dividend (15%)":                        "PPh 23 Dividen (15%)",
	"WHT Art. 23 Interest (15%)":                        "PPh 23 Bunga (15%)",
	"WHT Art. 22 Import (API) (2.5%)":                   "PPh 22 Impor API (2,5%)",
	"WHT Art. 22 Import (Non-API) (7.5%)":               "PPh 22 Impor Non-API (7,5%)",
	"WHT Art. 26 Dividend (20%)":                        "PPh 26 Dividen (20%)",
	"WHT Art. 26 Interest (20%)":                        "PPh 26 Bunga (20%)",
	"WHT Art. 26 Royalty (20%)":                         "PPh 26 Royalti (20%)",
	"WHT Art. 26 Services (20%)":                        "PPh 26 Jasa (20%)",
	"Final WHT Land & Building Rent (10%)":              "PPh Final Sewa Tanah & Bangunan (10%)",
	"Final WHT Deposit Interest (20%)":                  "PPh Final Bunga Deposito (20%)",
	"Final WHT Small Construction Services (2%)":        "PPh Final Jasa Konstruksi Kecil (2%)",
	"Final WHT Medium/Large Construction Services (3%)": "PPh Final Jasa Konstruksi Menengah/Besar (3%)",
	"Final WHT Land & Building Rights Transfer (2.5%)":  "PPh Final Pengalihan Hak Tanah & Bangunan (2,5%)",
}

// BackfillTaxIndonesianNamesOnce renames the seeded (is_system = 1) English default taxes of
// existing companies to their Indonesian names, once. Only system rows are touched, so a custom
// tax that happens to share an old name is left alone, and a company that already has a row with
// the new name is skipped rather than ending up with a duplicate. Rows keep their ids, so
// invoices/orders that reference them are unaffected.
func BackfillTaxIndonesianNamesOnce(db *gorm.DB) error {
	var rec model.MigrationRecord
	err := db.Where("key = ?", taxIndonesianNamesBackfillKey).First(&rec).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("check tax rename record: %w", err)
	}

	renamed := int64(0)
	if err := db.Transaction(func(tx *gorm.DB) error {
		for oldName, newName := range legacyTaxNames {
			res := tx.Exec(`UPDATE taxes t SET name = ?
				WHERE t.name = ? AND t.is_system = 1 AND t.deleted_at IS NULL
				AND NOT EXISTS (SELECT 1 FROM taxes x WHERE x.company_id = t.company_id AND x.name = ? AND x.deleted_at IS NULL)`,
				newName, oldName, newName)
			if res.Error != nil {
				return fmt.Errorf("rename tax %q: %w", oldName, res.Error)
			}
			renamed += res.RowsAffected
		}
		return tx.Create(&model.MigrationRecord{Key: taxIndonesianNamesBackfillKey}).Error
	}); err != nil {
		return err
	}
	log.Printf("✅ Tax rename backfill applied: %d row(s) renamed to Indonesian names", renamed)
	return nil
}
