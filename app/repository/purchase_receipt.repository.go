package repository

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/purchasereceipt"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

var purchaseReceiptListColumns = []string{"number", "date", "mitra_id", "amount", "payment_method", "created_at"}

type PurchaseReceiptRepository struct{ db *gorm.DB }

func NewPurchaseReceiptRepository(db *gorm.DB) domain.IRepository {
	return &PurchaseReceiptRepository{db: db}
}

func (r *PurchaseReceiptRepository) Create(dto *domain.CreateDTO, actorID string) (*model.PurchaseReceipt, error) {
	date, err := time.Parse("2006-01-02", strings.TrimSpace(dto.Date))
	if err != nil {
		return nil, &domain.ErrValidation{Message: "invalid date (format YYYY-MM-DD)"}
	}

	number := strings.TrimSpace(dto.Number)
	if number != "" {
		if exists, err := r.numberExists(dto.CompanyID, number); err != nil {
			return nil, err
		} else if exists {
			return nil, &domain.ErrNumberExists{Number: number}
		}
	} else {
		if number, err = generatePurchaseReceiptNumber(r.db, dto.CompanyID, date); err != nil {
			return nil, err
		}
	}

	allocations, err := normalizePurchaseReceiptAllocations(dto.Allocations)
	if err != nil {
		return nil, err
	}

	receiptID := uuid.NewString()
	var total float64

	err = r.db.Transaction(func(tx *gorm.DB) error {
		var err error
		if total, err = applyPurchaseReceiptAllocations(tx, dto.CompanyID, dto.MitraID, allocations); err != nil {
			return err
		}

		receipt := &model.PurchaseReceipt{
			ID:             receiptID,
			CompanyID:      dto.CompanyID,
			MitraID:        dto.MitraID,
			Number:         number,
			Date:           date,
			Amount:         total,
			PaymentMethod:  model.PurchaseReceiptPaymentMethod(dto.PaymentMethod),
			BankAccountID:  trimPtr(dto.BankAccountID),
			Notes:          utils.SanitizeRichText(dto.Notes),
			AttachmentData: dto.AttachmentData,
			AttachmentName: strings.TrimSpace(dto.AttachmentName),
			SignatureData:  dto.SignatureData,
			CreatedBy:      actorID,
			UpdatedBy:      actorID,
		}
		if err := tx.Create(receipt).Error; err != nil {
			return fmt.Errorf("create purchase receipt: %w", err)
		}

		allocRows := make([]model.PurchaseReceiptAllocation, len(allocations))
		for i, a := range allocations {
			allocRows[i] = model.PurchaseReceiptAllocation{
				ID:                uuid.NewString(),
				PurchaseReceiptID: receiptID,
				PurchaseInvoiceID: a.PurchaseInvoiceID,
				CompanyID:         dto.CompanyID,
				Amount:            a.Amount,
			}
		}
		if err := tx.Create(&allocRows).Error; err != nil {
			return fmt.Errorf("create purchase receipt allocations: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.FindByID(dto.CompanyID, receiptID)
}

func (r *PurchaseReceiptRepository) FindByID(companyID, id string) (*model.PurchaseReceipt, error) {
	var rec model.PurchaseReceipt
	err := r.db.Preload("Allocations").Where("id = ? AND company_id = ?", id, companyID).First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &domain.ErrNotFound{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("find purchase receipt %s: %w", id, err)
	}
	return &rec, nil
}

func (r *PurchaseReceiptRepository) FindAll(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	q := r.db.Model(&model.PurchaseReceipt{}).Where("company_id = ?", f.CompanyID)
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where("lower(number) LIKE ? OR lower(notes) LIKE ?", like, like)
	}
	if f.MitraID != "" {
		q = q.Where("mitra_id = ?", f.MitraID)
	}
	if f.PurchaseInvoiceID != "" {
		q = q.Where("EXISTS (SELECT 1 FROM purchase_receipt_allocations pra WHERE pra.purchase_receipt_id = purchase_receipts.id AND pra.purchase_invoice_id = ?)", f.PurchaseInvoiceID)
	}

	sortCol := utils.NormalizeSort(f.Sort, purchaseReceiptListColumns, "date")
	order := f.Order
	if strings.TrimSpace(order) == "" {
		order = "DESC"
	}
	return utils.GetPaginatedDataOffset(q, utils.OffsetPaginationOptions{
		Model:                &model.PurchaseReceipt{},
		Page:                 f.Page,
		Limit:                f.PageSize,
		Sort:                 sortCol,
		Order:                order,
		Select:               f.Fields,
		ValidColumns:         purchaseReceiptListColumns,
		Preloads:             []string{"Allocations"},
		PreserveAssociations: true,
	})
}

func (r *PurchaseReceiptRepository) MitraExists(companyID, mitraID string) (bool, error) {
	var n int64
	if err := r.db.Model(&model.Mitra{}).Where("id = ? AND company_id = ?", mitraID, companyID).Count(&n).Error; err != nil {
		return false, fmt.Errorf("check mitra: %w", err)
	}
	return n > 0, nil
}

func (r *PurchaseReceiptRepository) numberExists(companyID, number string) (bool, error) {
	var n int64
	err := r.db.Model(&model.PurchaseReceipt{}).
		Where("company_id = ? AND lower(number) = lower(?)", companyID, number).Count(&n).Error
	if err != nil {
		return false, fmt.Errorf("check purchase receipt number: %w", err)
	}
	return n > 0, nil
}

// generatePurchaseReceiptNumber — PKW/<YYYY>/<NNNN>, same max-trailing-integer
// scan as generatePurchaseOrderNumber/generatePurchaseInvoiceNumber.
func generatePurchaseReceiptNumber(db *gorm.DB, companyID string, date time.Time) (string, error) {
	prefix := fmt.Sprintf("PKW/%d/", date.Year())

	var numbers []string
	if err := db.Model(&model.PurchaseReceipt{}).
		Where("company_id = ? AND number LIKE ?", companyID, prefix+"%").
		Pluck("number", &numbers).Error; err != nil {
		return "", fmt.Errorf("scan purchase receipt numbers: %w", err)
	}
	max := 0
	for _, ref := range numbers {
		parts := strings.Split(ref, "/")
		if len(parts) == 0 {
			continue
		}
		if seq, err := strconv.Atoi(parts[len(parts)-1]); err == nil && seq > max {
			max = seq
		}
	}
	return fmt.Sprintf("%s%04d", prefix, max+1), nil
}

// PreviewNumber returns what the next auto-generated number would be right
// now — a preview for the Add page, not a reservation.
func (r *PurchaseReceiptRepository) PreviewNumber(companyID string) (string, error) {
	return generatePurchaseReceiptNumber(r.db, companyID, time.Now())
}

// normalizePurchaseReceiptAllocations sorts by invoice ID (a consistent lock order across
// concurrent receipts touching overlapping invoice sets avoids deadlocks) and rejects a
// duplicated invoice.
func normalizePurchaseReceiptAllocations(in []domain.AllocationDTO) ([]domain.AllocationDTO, error) {
	allocations := append([]domain.AllocationDTO(nil), in...)
	sort.Slice(allocations, func(i, j int) bool { return allocations[i].PurchaseInvoiceID < allocations[j].PurchaseInvoiceID })
	seen := make(map[string]bool, len(allocations))
	for _, a := range allocations {
		if seen[a.PurchaseInvoiceID] {
			return nil, &domain.ErrValidation{Message: "the same invoice can't be allocated twice in one receipt"}
		}
		seen[a.PurchaseInvoiceID] = true
	}
	return allocations, nil
}

// applyPurchaseReceiptAllocations applies each allocation's amount to its invoice balance (partner
// match, confirmed status and outstanding-balance checks live in applyPurchaseInvoicePayment).
// Returns the receipt total.
func applyPurchaseReceiptAllocations(tx *gorm.DB, companyID, mitraID string, allocations []domain.AllocationDTO) (float64, error) {
	var total float64
	for _, a := range allocations {
		if err := applyPurchaseInvoicePayment(tx, companyID, a.PurchaseInvoiceID, mitraID, a.Amount); err != nil {
			return 0, mapPurchasePaymentErr(err)
		}
		total = round2(total + a.Amount)
	}
	return total, nil
}

// reversePurchaseReceiptAllocations gives every allocation of the receipt back to its invoice
// (sorted by invoice ID for a consistent lock order).
func reversePurchaseReceiptAllocations(tx *gorm.DB, companyID, receiptID string) error {
	var rows []model.PurchaseReceiptAllocation
	if err := tx.Where("purchase_receipt_id = ?", receiptID).Order("purchase_invoice_id").Find(&rows).Error; err != nil {
		return fmt.Errorf("load receipt allocations: %w", err)
	}
	for _, a := range rows {
		if err := reversePurchaseInvoicePayment(tx, companyID, a.PurchaseInvoiceID, a.Amount); err != nil {
			return err
		}
	}
	return nil
}

// Update replaces the receipt: the previous allocations are reversed, the new ones validated and
// applied, all in one transaction. A blank Number keeps the current one.
func (r *PurchaseReceiptRepository) Update(companyID, id string, dto *domain.UpdateDTO, actorID string) (*model.PurchaseReceipt, error) {
	existing, err := r.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}
	date, err := time.Parse("2006-01-02", strings.TrimSpace(dto.Date))
	if err != nil {
		return nil, &domain.ErrValidation{Message: "invalid date (format YYYY-MM-DD)"}
	}
	number := strings.TrimSpace(dto.Number)
	if number == "" {
		number = existing.Number
	}
	if !strings.EqualFold(number, existing.Number) {
		var n int64
		if err := r.db.Model(&model.PurchaseReceipt{}).
			Where("company_id = ? AND lower(number) = lower(?) AND id <> ?", companyID, number, id).Count(&n).Error; err != nil {
			return nil, fmt.Errorf("check purchase receipt number: %w", err)
		}
		if n > 0 {
			return nil, &domain.ErrNumberExists{Number: number}
		}
	}
	allocations, err := normalizePurchaseReceiptAllocations(dto.Allocations)
	if err != nil {
		return nil, err
	}

	err = r.db.Transaction(func(tx *gorm.DB) error {
		if err := reversePurchaseReceiptAllocations(tx, companyID, id); err != nil {
			return err
		}
		total, err := applyPurchaseReceiptAllocations(tx, companyID, dto.MitraID, allocations)
		if err != nil {
			return err
		}
		if err := tx.Where("purchase_receipt_id = ?", id).Delete(&model.PurchaseReceiptAllocation{}).Error; err != nil {
			return fmt.Errorf("clear receipt allocations: %w", err)
		}
		rows := make([]model.PurchaseReceiptAllocation, len(allocations))
		for i, a := range allocations {
			rows[i] = model.PurchaseReceiptAllocation{
				ID: uuid.NewString(), PurchaseReceiptID: id, PurchaseInvoiceID: a.PurchaseInvoiceID, CompanyID: companyID, Amount: a.Amount,
			}
		}
		if err := tx.Create(&rows).Error; err != nil {
			return fmt.Errorf("create purchase receipt allocations: %w", err)
		}
		return tx.Model(&model.PurchaseReceipt{}).Where("id = ? AND company_id = ?", id, companyID).Updates(map[string]any{
			"mitra_id":        dto.MitraID,
			"number":          number,
			"date":            date,
			"amount":          total,
			"payment_method":  dto.PaymentMethod,
			"bank_account_id": trimPtr(dto.BankAccountID),
			"notes":           utils.SanitizeRichText(dto.Notes),
			"attachment_data": dto.AttachmentData,
			"attachment_name": strings.TrimSpace(dto.AttachmentName),
			"signature_data":  dto.SignatureData,
			"updated_at":      time.Now(),
			"updated_by":      actorID,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return r.FindByID(companyID, id)
}

// Delete soft-deletes the receipt and gives its allocations back to the invoices (allocation rows
// are kept for the audit trail; every query that reads them joins on the non-deleted receipt).
func (r *PurchaseReceiptRepository) Delete(companyID, id string) error {
	if _, err := r.FindByID(companyID, id); err != nil {
		return err
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := reversePurchaseReceiptAllocations(tx, companyID, id); err != nil {
			return err
		}
		if err := tx.Where("id = ? AND company_id = ?", id, companyID).Delete(&model.PurchaseReceipt{}).Error; err != nil {
			return fmt.Errorf("delete purchase receipt %s: %w", id, err)
		}
		return nil
	})
}

// mapPurchasePaymentErr turns balance errors into the receipt domain's own errors.
func mapPurchasePaymentErr(err error) error {
	var notConfirmed *ErrPurchaseInvoiceNotConfirmed
	if errors.As(err, &notConfirmed) {
		return &domain.ErrValidation{Message: notConfirmed.Error()}
	}
	var mismatch *ErrPurchaseInvoiceMismatch
	if errors.As(err, &mismatch) {
		return &domain.ErrValidation{Message: mismatch.Error()}
	}
	var exceeds *ErrPurchaseInvoiceExceedsBalance
	if errors.As(err, &exceeds) {
		return &domain.ErrExceedsBalance{Message: exceeds.Error()}
	}
	return err
}
