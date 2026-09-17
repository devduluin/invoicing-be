package repository

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/salespayment"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

var salesPaymentListColumns = []string{"number", "date", "amount", "payment_method", "status", "created_at"}

type SalesPaymentRepository struct{ db *gorm.DB }

func NewSalesPaymentRepository(db *gorm.DB) domain.IRepository {
	return &SalesPaymentRepository{db: db}
}

func (r *SalesPaymentRepository) Create(dto *domain.CreateDTO, actorID string) (*model.SalesPayment, error) {
	date, err := time.Parse("2006-01-02", strings.TrimSpace(dto.Date))
	if err != nil {
		return nil, &domain.ErrValidation{Message: "invalid date (format YYYY-MM-DD)"}
	}

	invoice, err := r.InvoiceForPayment(dto.CompanyID, dto.SalesInvoiceID)
	if err != nil {
		return nil, err
	}
	remaining := round2(invoice.GrandTotal - invoice.PaidAmount)
	if dto.Amount > remaining {
		return nil, &domain.ErrExceedsBalance{Message: fmt.Sprintf("amount exceeds the remaining balance (%.2f)", remaining)}
	}

	number := strings.TrimSpace(dto.Number)
	if number != "" {
		if exists, err := r.numberExists(dto.CompanyID, number); err != nil {
			return nil, err
		} else if exists {
			return nil, &domain.ErrNumberExists{Number: number}
		}
	} else {
		if number, err = generateSalesPaymentNumber(r.db, dto.CompanyID, date); err != nil {
			return nil, err
		}
	}

	payment := &model.SalesPayment{
		ID:             uuid.NewString(),
		CompanyID:      dto.CompanyID,
		SalesInvoiceID: dto.SalesInvoiceID,
		MitraID:        dto.MitraID,
		Number:         number,
		Date:           date,
		Amount:         dto.Amount,
		PaymentMethod:  model.SalesReceiptPaymentMethod(dto.PaymentMethod),
		BankAccountID:  trimPtr(dto.BankAccountID),
		RefNo:          strings.TrimSpace(dto.RefNo),
		Notes:          strings.TrimSpace(dto.Notes),
		CreatedBy:      actorID,
		UpdatedBy:      actorID,
	}
	if err := r.db.Create(payment).Error; err != nil {
		return nil, fmt.Errorf("create sales payment: %w", err)
	}
	return r.FindByID(dto.CompanyID, payment.ID)
}

func (r *SalesPaymentRepository) FindByID(companyID, id string) (*model.SalesPayment, error) {
	var p model.SalesPayment
	err := r.db.Where("id = ? AND company_id = ?", id, companyID).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &domain.ErrNotFound{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("find sales payment %s: %w", id, err)
	}
	return &p, nil
}

func (r *SalesPaymentRepository) FindAll(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	q := r.db.Model(&model.SalesPayment{}).Where("company_id = ?", f.CompanyID)
	if f.SalesInvoiceID != "" {
		q = q.Where("sales_invoice_id = ?", f.SalesInvoiceID)
	}
	if f.MitraID != "" {
		q = q.Where("mitra_id = ?", f.MitraID)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where("lower(number) LIKE ? OR lower(notes) LIKE ?", like, like)
	}

	sortCol := utils.NormalizeSort(f.Sort, salesPaymentListColumns, "date")
	order := f.Order
	if strings.TrimSpace(order) == "" {
		order = "DESC"
	}
	return utils.GetPaginatedDataOffset(q, utils.OffsetPaginationOptions{
		Model:                &model.SalesPayment{},
		Page:                 f.Page,
		Limit:                f.PageSize,
		Sort:                 sortCol,
		Order:                order,
		Select:               f.Fields,
		ValidColumns:         salesPaymentListColumns,
		PreserveAssociations: true,
	})
}

// InvoiceForPayment loads the invoice a payment is (or will be) against,
// scoped to the company. Used both to validate a Create and to snapshot the
// invoice's MitraID onto the payment.
func (r *SalesPaymentRepository) InvoiceForPayment(companyID, invoiceID string) (*model.SalesInvoice, error) {
	var inv model.SalesInvoice
	err := r.db.Where("id = ? AND company_id = ?", invoiceID, companyID).First(&inv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &domain.ErrValidation{Message: "invoice not found"}
	}
	if err != nil {
		return nil, fmt.Errorf("find invoice %s: %w", invoiceID, err)
	}
	return &inv, nil
}

// Verify locks the parent invoice row so two concurrent verifies against
// the same invoice can't both pass the balance check, then marks the
// payment verified and recomputes the invoice's PaidAmount/PaymentStatus.
func (r *SalesPaymentRepository) Verify(companyID, actorID, id string) (*model.SalesPayment, error) {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var payment model.SalesPayment
		if err := tx.Where("id = ? AND company_id = ?", id, companyID).First(&payment).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return &domain.ErrNotFound{ID: id}
			}
			return fmt.Errorf("find sales payment %s: %w", id, err)
		}
		if payment.Status != model.SalesPaymentStatusPending {
			return &domain.ErrInvalidTransition{Message: "only a pending payment can be verified"}
		}

		if err := applySalesInvoicePayment(tx, companyID, payment.SalesInvoiceID, payment.Amount); err != nil {
			var notConfirmed *ErrInvoiceNotConfirmed
			if errors.As(err, &notConfirmed) {
				return &domain.ErrValidation{Message: notConfirmed.Error()}
			}
			var exceeds *ErrInvoiceExceedsBalance
			if errors.As(err, &exceeds) {
				return &domain.ErrExceedsBalance{Message: exceeds.Error()}
			}
			return err
		}

		now := time.Now()
		if err := tx.Model(&payment).Updates(map[string]interface{}{
			"status":      model.SalesPaymentStatusVerified,
			"verified_by": actorID,
			"verified_at": &now,
			"updated_by":  actorID,
		}).Error; err != nil {
			return fmt.Errorf("verify sales payment %s: %w", id, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.FindByID(companyID, id)
}

func (r *SalesPaymentRepository) numberExists(companyID, number string) (bool, error) {
	var n int64
	err := r.db.Model(&model.SalesPayment{}).
		Where("company_id = ? AND lower(number) = lower(?)", companyID, number).Count(&n).Error
	if err != nil {
		return false, fmt.Errorf("check sales payment number: %w", err)
	}
	return n > 0, nil
}

// generateSalesPaymentNumber — PAY/<YYYY>/<NNNN>, same max-trailing-integer
// scan as generateSalesReceiptNumber.
func generateSalesPaymentNumber(db *gorm.DB, companyID string, date time.Time) (string, error) {
	prefix := fmt.Sprintf("PAY/%d/", date.Year())

	var numbers []string
	if err := db.Model(&model.SalesPayment{}).
		Where("company_id = ? AND number LIKE ?", companyID, prefix+"%").
		Pluck("number", &numbers).Error; err != nil {
		return "", fmt.Errorf("scan sales payment numbers: %w", err)
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

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
