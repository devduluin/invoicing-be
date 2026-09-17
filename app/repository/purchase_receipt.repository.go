package repository

import (
	"errors"
	"fmt"
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

	receipt := &model.PurchaseReceipt{
		ID:                uuid.NewString(),
		CompanyID:         dto.CompanyID,
		MitraID:           dto.MitraID,
		PurchaseInvoiceID: trimPtr(dto.PurchaseInvoiceID),
		Number:            number,
		Date:              date,
		Amount:            dto.Amount,
		PaymentMethod:     model.PurchaseReceiptPaymentMethod(dto.PaymentMethod),
		BankAccountID:     trimPtr(dto.BankAccountID),
		Notes:             strings.TrimSpace(dto.Notes),
		CreatedBy:         actorID,
		UpdatedBy:         actorID,
	}
	if err := r.db.Create(receipt).Error; err != nil {
		return nil, fmt.Errorf("create purchase receipt: %w", err)
	}
	return r.FindByID(dto.CompanyID, receipt.ID)
}

func (r *PurchaseReceiptRepository) FindByID(companyID, id string) (*model.PurchaseReceipt, error) {
	var rec model.PurchaseReceipt
	err := r.db.Where("id = ? AND company_id = ?", id, companyID).First(&rec).Error
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
