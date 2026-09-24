package repository

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/goodsreceipt"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

var goodsReceiptListColumns = []string{"number", "date", "mitra_id", "created_at"}

type GoodsReceiptRepository struct{ db *gorm.DB }

func NewGoodsReceiptRepository(db *gorm.DB) domain.IRepository {
	return &GoodsReceiptRepository{db: db}
}

func (r *GoodsReceiptRepository) Create(dto *domain.CreateDTO, actorID string) (*model.GoodsReceipt, error) {
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
	}

	var receiptID string
	err = r.db.Transaction(func(tx *gorm.DB) error {
		n := number
		if n == "" {
			gen, err := generateGoodsReceiptNumber(tx, dto.CompanyID, date)
			if err != nil {
				return err
			}
			n = gen
		}

		receipt := &model.GoodsReceipt{
			ID:              uuid.NewString(),
			CompanyID:       dto.CompanyID,
			MitraID:         dto.MitraID,
			PurchaseOrderID: trimPtr(dto.PurchaseOrderID),
			Number:          n,
			Date:            date,
			Notes:           utils.SanitizeRichText(dto.Notes),
			ShippingMethod:  strings.TrimSpace(dto.ShippingMethod),
			TrackingNo:      strings.TrimSpace(dto.TrackingNo),
			VehicleNo:       strings.TrimSpace(dto.VehicleNo),
			DriverName:      strings.TrimSpace(dto.DriverName),
			TotalWeight:     dto.TotalWeight,
			AttachmentData:  dto.AttachmentData,
			AttachmentName:  strings.TrimSpace(dto.AttachmentName),
			CreatedBy:       actorID,
			UpdatedBy:       actorID,
		}
		if err := tx.Create(receipt).Error; err != nil {
			return fmt.Errorf("create goods receipt: %w", err)
		}
		lines := buildGoodsReceiptLines(dto.Lines, dto.CompanyID, receipt.ID)
		if err := tx.Create(&lines).Error; err != nil {
			return fmt.Errorf("create goods receipt lines: %w", err)
		}
		receiptID = receipt.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.FindByID(dto.CompanyID, receiptID)
}

func (r *GoodsReceiptRepository) FindByID(companyID, id string) (*model.GoodsReceipt, error) {
	var rec model.GoodsReceipt
	err := r.db.
		Preload("Lines", func(db *gorm.DB) *gorm.DB { return db.Order("line_order ASC") }).
		Where("id = ? AND company_id = ?", id, companyID).First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &domain.ErrNotFound{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("find goods receipt %s: %w", id, err)
	}
	return &rec, nil
}

func (r *GoodsReceiptRepository) FindAll(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	q := r.db.Model(&model.GoodsReceipt{}).Where("company_id = ?", f.CompanyID)
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where("lower(number) LIKE ? OR lower(notes) LIKE ?", like, like)
	}
	if f.MitraID != "" {
		q = q.Where("mitra_id = ?", f.MitraID)
	}

	sortCol := utils.NormalizeSort(f.Sort, goodsReceiptListColumns, "date")
	order := f.Order
	if strings.TrimSpace(order) == "" {
		order = "DESC"
	}
	return utils.GetPaginatedDataOffset(q, utils.OffsetPaginationOptions{
		Model:                &model.GoodsReceipt{},
		Page:                 f.Page,
		Limit:                f.PageSize,
		Sort:                 sortCol,
		Order:                order,
		Select:               f.Fields,
		ValidColumns:         goodsReceiptListColumns,
		Exclude:              listBlobExclude,
		PreserveAssociations: true,
	})
}

func (r *GoodsReceiptRepository) MitraExists(companyID, mitraID string) (bool, error) {
	var n int64
	if err := r.db.Model(&model.Mitra{}).Where("id = ? AND company_id = ?", mitraID, companyID).Count(&n).Error; err != nil {
		return false, fmt.Errorf("check mitra: %w", err)
	}
	return n > 0, nil
}

func (r *GoodsReceiptRepository) numberExists(companyID, number string) (bool, error) {
	var n int64
	err := r.db.Model(&model.GoodsReceipt{}).
		Where("company_id = ? AND lower(number) = lower(?)", companyID, number).Count(&n).Error
	if err != nil {
		return false, fmt.Errorf("check goods receipt number: %w", err)
	}
	return n > 0, nil
}

// generateGoodsReceiptNumber — GR/<YYYY>/<NNNN>, same max-trailing-integer
// scan as generatePurchaseOrderNumber/generatePurchaseInvoiceNumber.
func generateGoodsReceiptNumber(tx *gorm.DB, companyID string, date time.Time) (string, error) {
	prefix := fmt.Sprintf("GR/%d/", date.Year())

	var numbers []string
	if err := tx.Model(&model.GoodsReceipt{}).
		Where("company_id = ? AND number LIKE ?", companyID, prefix+"%").
		Pluck("number", &numbers).Error; err != nil {
		return "", fmt.Errorf("scan goods receipt numbers: %w", err)
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

func buildGoodsReceiptLines(lines []domain.LineDTO, companyID, receiptID string) []model.GoodsReceiptLine {
	out := make([]model.GoodsReceiptLine, 0, len(lines))
	for i, l := range lines {
		out = append(out, model.GoodsReceiptLine{
			ID:             uuid.NewString(),
			GoodsReceiptID: receiptID,
			CompanyID:      companyID,
			ProductName:    strings.TrimSpace(l.ProductName),
			Description:    strings.TrimSpace(l.Description),
			Quantity:       l.Quantity,
			Unit:           strings.TrimSpace(l.Unit),
			LineOrder:      i,
		})
	}
	return out
}

func (r *GoodsReceiptRepository) numberExistsExcept(companyID, number, exceptID string) (bool, error) {
	var n int64
	err := r.db.Model(&model.GoodsReceipt{}).
		Where("company_id = ? AND lower(number) = lower(?) AND id <> ?", companyID, number, exceptID).Count(&n).Error
	if err != nil {
		return false, fmt.Errorf("check goods receipt number: %w", err)
	}
	return n > 0, nil
}

// Update replaces the header and the lines. A blank Number keeps the current one.
func (r *GoodsReceiptRepository) Update(companyID, id string, dto *domain.UpdateDTO, actorID string) (*model.GoodsReceipt, error) {
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
		if taken, err := r.numberExistsExcept(companyID, number, id); err != nil {
			return nil, err
		} else if taken {
			return nil, &domain.ErrNumberExists{Number: number}
		}
	}

	err = r.db.Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"mitra_id":          dto.MitraID,
			"purchase_order_id": trimPtr(dto.PurchaseOrderID),
			"number":            number,
			"date":              date,
			"notes":             utils.SanitizeRichText(dto.Notes),
			"shipping_method":   strings.TrimSpace(dto.ShippingMethod),
			"tracking_no":       strings.TrimSpace(dto.TrackingNo),
			"vehicle_no":        strings.TrimSpace(dto.VehicleNo),
			"driver_name":       strings.TrimSpace(dto.DriverName),
			"total_weight":      dto.TotalWeight,
			"attachment_data":   dto.AttachmentData,
			"attachment_name":   strings.TrimSpace(dto.AttachmentName),
			"updated_at":        time.Now(),
			"updated_by":        actorID,
		}
		if err := tx.Model(&model.GoodsReceipt{}).Where("id = ? AND company_id = ?", id, companyID).Updates(updates).Error; err != nil {
			return fmt.Errorf("update goods receipt %s: %w", id, err)
		}
		if err := tx.Where("goods_receipt_id = ?", id).Delete(&model.GoodsReceiptLine{}).Error; err != nil {
			return fmt.Errorf("replace goods receipt lines %s: %w", id, err)
		}
		lines := buildGoodsReceiptLines(dto.Lines, companyID, id)
		if err := tx.Create(&lines).Error; err != nil {
			return fmt.Errorf("create goods receipt lines: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.FindByID(companyID, id)
}

// Delete soft-deletes the record only; its lines are kept so it stays auditable.
func (r *GoodsReceiptRepository) Delete(companyID, id string) error {
	if _, err := r.FindByID(companyID, id); err != nil {
		return err
	}
	if err := r.db.Where("id = ? AND company_id = ?", id, companyID).Delete(&model.GoodsReceipt{}).Error; err != nil {
		return fmt.Errorf("delete goods receipt %s: %w", id, err)
	}
	return nil
}
