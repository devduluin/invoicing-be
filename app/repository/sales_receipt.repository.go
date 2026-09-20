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

	domain "duluin_invoice/app/domain/salesreceipt"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

var salesReceiptListColumns = []string{"number", "date", "mitra_id", "amount", "payment_method", "created_at"}

type SalesReceiptRepository struct{ db *gorm.DB }

func NewSalesReceiptRepository(db *gorm.DB) domain.IRepository {
	return &SalesReceiptRepository{db: db}
}

func (r *SalesReceiptRepository) Create(dto *domain.CreateDTO, actorID string) (*model.SalesReceipt, error) {
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
		if number, err = generateSalesReceiptNumber(r.db, dto.CompanyID, date); err != nil {
			return nil, err
		}
	}

	allocations, err := normalizeReceiptAllocations(dto.Allocations)
	if err != nil {
		return nil, err
	}

	receiptID := uuid.NewString()
	var total float64

	err = r.db.Transaction(func(tx *gorm.DB) error {
		var err error
		if total, err = applyReceiptAllocations(tx, dto.CompanyID, dto.MitraID, allocations); err != nil {
			return err
		}

		receipt := &model.SalesReceipt{
			ID:            receiptID,
			CompanyID:     dto.CompanyID,
			MitraID:       dto.MitraID,
			Number:        number,
			Date:          date,
			Amount:        total,
			PaymentMethod: model.SalesReceiptPaymentMethod(dto.PaymentMethod),
			BankAccountID: trimPtr(dto.BankAccountID),
			Notes:         utils.SanitizeRichText(dto.Notes),
			CreatedBy:     actorID,
			UpdatedBy:     actorID,
		}
		if err := tx.Create(receipt).Error; err != nil {
			return fmt.Errorf("create sales receipt: %w", err)
		}

		allocRows := make([]model.SalesReceiptAllocation, len(allocations))
		for i, a := range allocations {
			allocRows[i] = model.SalesReceiptAllocation{
				ID:             uuid.NewString(),
				SalesReceiptID: receiptID,
				SalesInvoiceID: a.SalesInvoiceID,
				CompanyID:      dto.CompanyID,
				Amount:         a.Amount,
			}
		}
		if err := tx.Create(&allocRows).Error; err != nil {
			return fmt.Errorf("create sales receipt allocations: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.FindByID(dto.CompanyID, receiptID)
}

func (r *SalesReceiptRepository) FindByID(companyID, id string) (*model.SalesReceipt, error) {
	var rec model.SalesReceipt
	err := r.db.Preload("Allocations").Where("id = ? AND company_id = ?", id, companyID).First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &domain.ErrNotFound{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("find sales receipt %s: %w", id, err)
	}
	return &rec, nil
}

func (r *SalesReceiptRepository) FindAll(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	q := r.db.Model(&model.SalesReceipt{}).Where("company_id = ?", f.CompanyID)
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where("lower(number) LIKE ? OR lower(notes) LIKE ?", like, like)
	}
	if f.MitraID != "" {
		q = q.Where("mitra_id = ?", f.MitraID)
	}
	if f.SalesInvoiceID != "" {
		q = q.Where("EXISTS (SELECT 1 FROM sales_receipt_allocations sra WHERE sra.sales_receipt_id = sales_receipts.id AND sra.sales_invoice_id = ?)", f.SalesInvoiceID)
	}

	sortCol := utils.NormalizeSort(f.Sort, salesReceiptListColumns, "date")
	order := f.Order
	if strings.TrimSpace(order) == "" {
		order = "DESC"
	}
	return utils.GetPaginatedDataOffset(q, utils.OffsetPaginationOptions{
		Model:                &model.SalesReceipt{},
		Page:                 f.Page,
		Limit:                f.PageSize,
		Sort:                 sortCol,
		Order:                order,
		Select:               f.Fields,
		ValidColumns:         salesReceiptListColumns,
		Preloads:             []string{"Allocations"},
		PreserveAssociations: true,
	})
}

func (r *SalesReceiptRepository) MitraExists(companyID, mitraID string) (bool, error) {
	var n int64
	if err := r.db.Model(&model.Mitra{}).Where("id = ? AND company_id = ?", mitraID, companyID).Count(&n).Error; err != nil {
		return false, fmt.Errorf("check mitra: %w", err)
	}
	return n > 0, nil
}

func (r *SalesReceiptRepository) numberExists(companyID, number string) (bool, error) {
	var n int64
	err := r.db.Model(&model.SalesReceipt{}).
		Where("company_id = ? AND lower(number) = lower(?)", companyID, number).Count(&n).Error
	if err != nil {
		return false, fmt.Errorf("check sales receipt number: %w", err)
	}
	return n > 0, nil
}

// generateSalesReceiptNumber — KW/<YYYY>/<NNNN>, same max-trailing-integer
// scan as generateSalesOrderNumber/generateSalesInvoiceNumber.
func generateSalesReceiptNumber(db *gorm.DB, companyID string, date time.Time) (string, error) {
	prefix := fmt.Sprintf("KW/%d/", date.Year())

	var numbers []string
	if err := db.Model(&model.SalesReceipt{}).
		Where("company_id = ? AND number LIKE ?", companyID, prefix+"%").
		Pluck("number", &numbers).Error; err != nil {
		return "", fmt.Errorf("scan sales receipt numbers: %w", err)
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
func (r *SalesReceiptRepository) PreviewNumber(companyID string) (string, error) {
	return generateSalesReceiptNumber(r.db, companyID, time.Now())
}

// normalizeReceiptAllocations sorts by invoice ID (a consistent lock order across
// concurrent receipts touching overlapping invoice sets avoids deadlocks) and
// rejects a duplicated invoice.
func normalizeReceiptAllocations(in []domain.AllocationDTO) ([]domain.AllocationDTO, error) {
	allocations := append([]domain.AllocationDTO(nil), in...)
	sort.Slice(allocations, func(i, j int) bool { return allocations[i].SalesInvoiceID < allocations[j].SalesInvoiceID })
	seen := make(map[string]bool, len(allocations))
	for _, a := range allocations {
		if seen[a.SalesInvoiceID] {
			return nil, &domain.ErrValidation{Message: "the same invoice can't be allocated twice in one receipt"}
		}
		seen[a.SalesInvoiceID] = true
	}
	return allocations, nil
}

// applyReceiptAllocations validates each invoice belongs to the partner and
// applies its amount to the invoice balance. Returns the receipt total.
func applyReceiptAllocations(tx *gorm.DB, companyID, mitraID string, allocations []domain.AllocationDTO) (float64, error) {
	var total float64
	for _, a := range allocations {
		var invoice model.SalesInvoice
		if err := tx.Where("id = ? AND company_id = ?", a.SalesInvoiceID, companyID).First(&invoice).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, &domain.ErrValidation{Message: "invoice not found"}
			}
			return 0, fmt.Errorf("find invoice %s: %w", a.SalesInvoiceID, err)
		}
		if invoice.MitraID != mitraID {
			return 0, &domain.ErrValidation{Message: "invoice does not belong to the selected partner"}
		}
		if err := applySalesInvoicePayment(tx, companyID, a.SalesInvoiceID, a.Amount); err != nil {
			var notConfirmed *ErrInvoiceNotConfirmed
			if errors.As(err, &notConfirmed) {
				return 0, &domain.ErrValidation{Message: notConfirmed.Error()}
			}
			var exceeds *ErrInvoiceExceedsBalance
			if errors.As(err, &exceeds) {
				return 0, &domain.ErrExceedsBalance{Message: exceeds.Error()}
			}
			return 0, err
		}
		total = round2(total + a.Amount)
	}
	return total, nil
}

// reverseReceiptAllocations gives every allocation of the receipt back to its
// invoice (sorted by invoice ID for a consistent lock order).
func reverseReceiptAllocations(tx *gorm.DB, companyID, receiptID string) error {
	var rows []model.SalesReceiptAllocation
	if err := tx.Where("sales_receipt_id = ?", receiptID).Order("sales_invoice_id").Find(&rows).Error; err != nil {
		return fmt.Errorf("load receipt allocations: %w", err)
	}
	for _, a := range rows {
		if err := reverseSalesInvoicePayment(tx, companyID, a.SalesInvoiceID, a.Amount); err != nil {
			return err
		}
	}
	return nil
}

// Update replaces the receipt: the previous allocations are reversed, the new
// ones validated and applied, all in one transaction. A blank Number keeps the
// current one.
func (r *SalesReceiptRepository) Update(companyID, id string, dto *domain.UpdateDTO, actorID string) (*model.SalesReceipt, error) {
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
		if err := r.db.Model(&model.SalesReceipt{}).
			Where("company_id = ? AND lower(number) = lower(?) AND id <> ?", companyID, number, id).Count(&n).Error; err != nil {
			return nil, fmt.Errorf("check sales receipt number: %w", err)
		}
		if n > 0 {
			return nil, &domain.ErrNumberExists{Number: number}
		}
	}
	allocations, err := normalizeReceiptAllocations(dto.Allocations)
	if err != nil {
		return nil, err
	}

	err = r.db.Transaction(func(tx *gorm.DB) error {
		if err := reverseReceiptAllocations(tx, companyID, id); err != nil {
			return err
		}
		total, err := applyReceiptAllocations(tx, companyID, dto.MitraID, allocations)
		if err != nil {
			return err
		}
		if err := tx.Where("sales_receipt_id = ?", id).Delete(&model.SalesReceiptAllocation{}).Error; err != nil {
			return fmt.Errorf("clear receipt allocations: %w", err)
		}
		rows := make([]model.SalesReceiptAllocation, len(allocations))
		for i, a := range allocations {
			rows[i] = model.SalesReceiptAllocation{
				ID: uuid.NewString(), SalesReceiptID: id, SalesInvoiceID: a.SalesInvoiceID, CompanyID: companyID, Amount: a.Amount,
			}
		}
		if err := tx.Create(&rows).Error; err != nil {
			return fmt.Errorf("create sales receipt allocations: %w", err)
		}
		return tx.Model(&model.SalesReceipt{}).Where("id = ? AND company_id = ?", id, companyID).Updates(map[string]any{
			"mitra_id":        dto.MitraID,
			"number":          number,
			"date":            date,
			"amount":          total,
			"payment_method":  dto.PaymentMethod,
			"bank_account_id": trimPtr(dto.BankAccountID),
			"notes":           utils.SanitizeRichText(dto.Notes),
			"updated_at":      time.Now(),
			"updated_by":      actorID,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return r.FindByID(companyID, id)
}

// Delete soft-deletes the receipt and gives its allocations back to the
// invoices (allocation rows are kept for the audit trail; every query that
// reads them joins on the non-deleted receipt).
func (r *SalesReceiptRepository) Delete(companyID, id string) error {
	if _, err := r.FindByID(companyID, id); err != nil {
		return err
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := reverseReceiptAllocations(tx, companyID, id); err != nil {
			return err
		}
		if err := tx.Where("id = ? AND company_id = ?", id, companyID).Delete(&model.SalesReceipt{}).Error; err != nil {
			return fmt.Errorf("delete sales receipt %s: %w", id, err)
		}
		return nil
	})
}
