package repository

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	domain "duluin_invoice/app/domain/contactperson"
	"duluin_invoice/app/model"
)

type ContactPersonRepository struct{ db *gorm.DB }

func NewContactPersonRepository(db *gorm.DB) domain.IRepository {
	return &ContactPersonRepository{db: db}
}

// lockMitra takes a row lock on the partner (company-scoped, must be active). Every change to a
// partner's contacts goes through it, so concurrent requests for the SAME partner run one after
// another and a double submit is caught by the duplicate index instead of racing past it.
func lockMitra(tx *gorm.DB, companyID, mitraID string) error {
	var id string
	err := tx.Model(&model.Mitra{}).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND company_id = ?", mitraID, companyID).Select("id").Take(&id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &domain.ErrNotFound{ID: mitraID}
	}
	if err != nil {
		return fmt.Errorf("lock partner %s: %w", mitraID, err)
	}
	return nil
}

func (r *ContactPersonRepository) List(companyID, mitraID, search string) ([]model.ContactPerson, error) {
	q := r.db.Where("company_id = ? AND mitra_id = ?", companyID, mitraID)
	if s := strings.TrimSpace(search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where("LOWER(name) LIKE ? OR LOWER(position) LIKE ? OR LOWER(phone) LIKE ? OR LOWER(email) LIKE ?", like, like, like, like)
	}
	var rows []model.ContactPerson
	if err := q.Order("LOWER(name) ASC, created_at ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list contact persons: %w", err)
	}
	return rows, nil
}

func (r *ContactPersonRepository) Get(companyID, mitraID, id string) (*model.ContactPerson, error) {
	var row model.ContactPerson
	err := r.db.Where("id = ? AND company_id = ? AND mitra_id = ?", id, companyID, mitraID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &domain.ErrNotFound{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("get contact person: %w", err)
	}
	return &row, nil
}

func apply(row *model.ContactPerson, name, position, phone, email string) {
	row.Name = strings.TrimSpace(name)
	row.Position = strings.TrimSpace(position)
	row.Phone = strings.TrimSpace(phone)
	row.Email = strings.TrimSpace(email)
}

func createContact(tx *gorm.DB, companyID, mitraID, actorID string, in domain.Input) (*model.ContactPerson, error) {
	row := model.ContactPerson{CompanyID: companyID, MitraID: mitraID, CreatedBy: actorID, UpdatedBy: actorID}
	apply(&row, in.Name, in.Position, in.Phone, in.Email)
	if row.Name == "" {
		return nil, &domain.ErrValidation{Message: "contact person name is required"}
	}
	if err := tx.Create(&row).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, &domain.ErrDuplicate{}
		}
		return nil, fmt.Errorf("create contact person: %w", err)
	}
	return &row, nil
}

func (r *ContactPersonRepository) Create(companyID, mitraID, actorID string, in *domain.Input) (*model.ContactPerson, error) {
	var created *model.ContactPerson
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockMitra(tx, companyID, mitraID); err != nil {
			return err
		}
		row, err := createContact(tx, companyID, mitraID, actorID, *in)
		created = row
		return err
	})
	return created, err
}

func (r *ContactPersonRepository) Update(companyID, mitraID, id, actorID string, in *domain.Input) (*model.ContactPerson, error) {
	var out model.ContactPerson
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockMitra(tx, companyID, mitraID); err != nil {
			return err
		}
		var row model.ContactPerson
		if err := tx.Where("id = ? AND company_id = ? AND mitra_id = ?", id, companyID, mitraID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return &domain.ErrNotFound{ID: id}
			}
			return fmt.Errorf("find contact person: %w", err)
		}
		apply(&row, in.Name, in.Position, in.Phone, in.Email)
		if row.Name == "" {
			return &domain.ErrValidation{Message: "contact person name is required"}
		}
		row.UpdatedBy = actorID
		if err := tx.Save(&row).Error; err != nil {
			if isUniqueViolation(err) {
				return &domain.ErrDuplicate{}
			}
			return fmt.Errorf("update contact person: %w", err)
		}
		out = row
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete is a soft delete.
func (r *ContactPersonRepository) Delete(companyID, mitraID, id string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockMitra(tx, companyID, mitraID); err != nil {
			return err
		}
		res := tx.Where("id = ? AND company_id = ? AND mitra_id = ?", id, companyID, mitraID).Delete(&model.ContactPerson{})
		if res.Error != nil {
			return fmt.Errorf("delete contact person: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return &domain.ErrNotFound{ID: id}
		}
		return nil
	})
}

func (r *ContactPersonRepository) Summaries(companyID string) ([]domain.Summary, error) {
	var rows []domain.Summary
	err := r.db.Raw(`
		SELECT mitra_id, COUNT(*)::int AS count
		FROM contact_persons
		WHERE company_id = ? AND deleted_at IS NULL
		GROUP BY mitra_id`, companyID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("contact summaries: %w", err)
	}
	return rows, nil
}

// syncContacts makes a partner's active contacts equal to `inputs`, INSIDE the caller's
// transaction (the partner row must already be locked or just created): rows with an id are updated,
// rows without one are created, and active contacts that are not in the list are soft-deleted.
// Each kind of change needs its own permission; a change the caller may not make fails the whole save.
func syncContacts(tx *gorm.DB, companyID, mitraID, actorID string, inputs []domain.SyncInput, perms domain.Perms) error {
	var existing []model.ContactPerson
	if err := tx.Where("company_id = ? AND mitra_id = ?", companyID, mitraID).Find(&existing).Error; err != nil {
		return fmt.Errorf("load contact persons: %w", err)
	}
	byID := make(map[string]model.ContactPerson, len(existing))
	for _, c := range existing {
		byID[c.ID] = c
	}
	keep := make(map[string]bool, len(inputs))
	for _, in := range inputs {
		if in.ID != "" {
			if _, ok := byID[in.ID]; !ok {
				return &domain.ErrValidation{Message: "a contact person no longer exists — reload and try again"}
			}
			if keep[in.ID] {
				return &domain.ErrValidation{Message: "the same contact person was sent twice"}
			}
			keep[in.ID] = true
		}
	}

	// 1. deletions first, so a renamed/replaced contact never trips the duplicate index
	for _, c := range existing {
		if keep[c.ID] {
			continue
		}
		if !perms.Delete {
			return &domain.ErrForbidden{Action: "delete"}
		}
		if err := tx.Delete(&model.ContactPerson{}, "id = ?", c.ID).Error; err != nil {
			return fmt.Errorf("delete contact person: %w", err)
		}
	}
	// 2. updates (only those that actually changed) and creations
	for _, in := range inputs {
		if in.ID != "" {
			row := byID[in.ID]
			before := row
			apply(&row, in.Name, in.Position, in.Phone, in.Email)
			if row.Name == "" {
				return &domain.ErrValidation{Message: "contact person name is required"}
			}
			if row.Name == before.Name && row.Position == before.Position && row.Phone == before.Phone && row.Email == before.Email {
				continue
			}
			if !perms.Update {
				return &domain.ErrForbidden{Action: "edit"}
			}
			row.UpdatedBy = actorID
			if err := tx.Save(&row).Error; err != nil {
				if isUniqueViolation(err) {
					return &domain.ErrDuplicate{}
				}
				return fmt.Errorf("update contact person: %w", err)
			}
			continue
		}
		if !perms.Create {
			return &domain.ErrForbidden{Action: "add"}
		}
		if _, err := createContact(tx, companyID, mitraID, actorID, domain.Input{Name: in.Name, Position: in.Position, Phone: in.Phone, Email: in.Email}); err != nil {
			return err
		}
	}
	return nil
}

// ── document snapshot ─────────────────────────────────────────────────────────────────────────────

var errContactInvalid = errors.New("contact person does not belong to this partner")

// contactSnap is what a document stores about its contact: the reference AND a copy of the details,
// so the document keeps printing the same person even if the contact is edited or deleted later.
type contactSnap struct {
	ID       *string
	Name     string
	Position string
	Phone    string
	Email    string
	// Keep: the document already references this very contact; leave its stored snapshot untouched.
	Keep bool
}

// contactSnapshot resolves the contact a document is saved with.
//   - no id                → the document has no contact (fields cleared)
//   - id == the document's current contact → Keep (never re-read: it may have been deleted or edited)
//   - otherwise            → must be an ACTIVE contact of THIS partner in THIS company; its details are copied
func contactSnapshot(db *gorm.DB, companyID, mitraID string, contactID *string, currentID string) (*contactSnap, error) {
	if contactID == nil || strings.TrimSpace(*contactID) == "" {
		return &contactSnap{}, nil
	}
	id := strings.TrimSpace(*contactID)
	if currentID != "" && id == currentID {
		return &contactSnap{ID: &id, Keep: true}, nil
	}
	var c model.ContactPerson
	err := db.Where("id = ? AND company_id = ? AND mitra_id = ?", id, companyID, mitraID).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errContactInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("find contact person: %w", err)
	}
	return &contactSnap{ID: &c.ID, Name: c.Name, Position: c.Position, Phone: c.Phone, Email: c.Email}, nil
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// RemovePICBackfilledContacts — one-time, idempotent cleanup. An earlier version copied each
// partner's "Contact Name (PIC)" into a contact person. The PIC is a company field, not a contact
// person, so those machine-made rows (created by "backfill" and never edited) are removed. Rows a
// user created or changed are untouched, and the PIC field itself is never modified.
func RemovePICBackfilledContacts(db *gorm.DB) error {
	return db.Exec(`
		DELETE FROM contact_persons
		WHERE created_by = 'backfill' AND updated_by = 'backfill'
		  AND id NOT IN (SELECT contact_person_id FROM sales_orders WHERE contact_person_id IS NOT NULL
		                 UNION SELECT contact_person_id FROM sales_invoices WHERE contact_person_id IS NOT NULL
		                 UNION SELECT contact_person_id FROM purchase_orders WHERE contact_person_id IS NOT NULL
		                 UNION SELECT contact_person_id FROM purchase_invoices WHERE contact_person_id IS NOT NULL)`).Error
}
