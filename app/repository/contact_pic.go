package repository

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/contactperson"
	"duluin_invoice/app/model"
)

// picName is the contact person's name: the PIC's name, or the partner's own name when none was typed.
func picName(contactName, partnerName string) string {
	if n := strings.TrimSpace(contactName); n != "" {
		return n
	}
	return strings.TrimSpace(partnerName)
}

// findEqualContact returns the active contact of this partner with exactly these details, if any
// (the same key as the duplicate index: name + email + phone, case-insensitive on name/email).
func findEqualContact(tx *gorm.DB, mitraID, name, email, phone string) (*model.ContactPerson, error) {
	var row model.ContactPerson
	err := tx.Where("mitra_id = ? AND lower(name) = lower(?) AND lower(coalesce(email, '')) = lower(?) AND coalesce(phone, '') = ?",
		mitraID, name, email, phone).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find contact person: %w", err)
	}
	return &row, nil
}

// createPICContact makes the partner's first contact person from its PIC details, flagged is_pic so
// later edits of the PIC update THIS row instead of adding another. If a contact with the same
// details already exists it is adopted (flagged) rather than duplicated. System-made, so it does not
// need the caller's contact permissions.
func createPICContact(tx *gorm.DB, companyID, mitraID, actorID, name, email, phone string) error {
	name, email, phone = strings.TrimSpace(name), strings.TrimSpace(email), strings.TrimSpace(phone)
	if name == "" {
		return nil
	}
	if existing, err := findEqualContact(tx, mitraID, name, email, phone); err != nil {
		return err
	} else if existing != nil {
		return tx.Model(&model.ContactPerson{}).Where("id = ?", existing.ID).Update("is_pic", true).Error
	}
	row, err := createContact(tx, companyID, mitraID, actorID, domain.Input{Name: name, Phone: phone, Email: email})
	if err != nil {
		return err
	}
	return tx.Model(&model.ContactPerson{}).Where("id = ?", row.ID).Update("is_pic", true).Error
}

// syncPICContact keeps the PIC contact in step when the partner's PIC name/email/phone CHANGED. It
// updates the is_pic row if there is one; otherwise it adopts an identical contact or creates one.
// A partner saved before this feature existed has a PIC but no is_pic contact at all — that gets
// backfilled here too, even when the PIC fields themselves are unchanged on this save; only a
// partner that ALREADY has its PIC contact skips the write when nothing changed, so a contact the
// user edited by hand is never overwritten by an unrelated partner save.
func syncPICContact(tx *gorm.DB, companyID, mitraID, actorID string, before *model.Mitra, name, email, phone string) error {
	newName, newEmail, newPhone := picName(name, before.Name), strings.TrimSpace(email), strings.TrimSpace(phone)
	oldName := picName(before.ContactName, before.Name)
	if newEmail == "" {
		newEmail = strings.TrimSpace(before.Email)
	}
	if newPhone == "" {
		newPhone = strings.TrimSpace(before.Phone)
	}
	if strings.TrimSpace(name) == "" {
		newName = oldName
	}

	var pic model.ContactPerson
	err := tx.Where("mitra_id = ? AND is_pic = ?", mitraID, true).First(&pic).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return createPICContact(tx, companyID, mitraID, actorID, newName, newEmail, newPhone)
	}
	if err != nil {
		return fmt.Errorf("find pic contact: %w", err)
	}
	if newName == oldName && newEmail == strings.TrimSpace(before.Email) && newPhone == strings.TrimSpace(before.Phone) {
		return nil
	}
	if other, err := findEqualContact(tx, mitraID, newName, newEmail, newPhone); err != nil {
		return err
	} else if other != nil && other.ID != pic.ID {
		return nil // another contact already carries these details; never create a duplicate
	}
	apply(&pic, newName, pic.Position, newPhone, newEmail)
	pic.UpdatedBy = actorID
	if err := tx.Save(&pic).Error; err != nil {
		if isUniqueViolation(err) {
			return nil
		}
		return fmt.Errorf("update pic contact: %w", err)
	}
	return nil
}
