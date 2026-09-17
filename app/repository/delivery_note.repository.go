package repository

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/deliverynote"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

var deliveryNoteListColumns = []string{"number", "date", "mitra_id", "created_at"}

type DeliveryNoteRepository struct{ db *gorm.DB }

func NewDeliveryNoteRepository(db *gorm.DB) domain.IRepository {
	return &DeliveryNoteRepository{db: db}
}

func (r *DeliveryNoteRepository) Create(dto *domain.CreateDTO, actorID string) (*model.DeliveryNote, error) {
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

	var noteID string
	err = r.db.Transaction(func(tx *gorm.DB) error {
		n := number
		if n == "" {
			gen, err := generateDeliveryNoteNumber(tx, dto.CompanyID, date)
			if err != nil {
				return err
			}
			n = gen
		}

		note := &model.DeliveryNote{
			ID:             uuid.NewString(),
			CompanyID:      dto.CompanyID,
			MitraID:        dto.MitraID,
			SalesOrderID:   trimPtr(dto.SalesOrderID),
			Number:         n,
			Date:           date,
			Notes:          strings.TrimSpace(dto.Notes),
			ShippingMethod: strings.TrimSpace(dto.ShippingMethod),
			TrackingNo:     strings.TrimSpace(dto.TrackingNo),
			VehicleNo:      strings.TrimSpace(dto.VehicleNo),
			DriverName:     strings.TrimSpace(dto.DriverName),
			TotalWeight:    dto.TotalWeight,
			AttachmentData: dto.AttachmentData,
			AttachmentName: strings.TrimSpace(dto.AttachmentName),
			CreatedBy:      actorID,
			UpdatedBy:      actorID,
		}
		if err := tx.Create(note).Error; err != nil {
			return fmt.Errorf("create delivery note: %w", err)
		}
		lines := buildDeliveryNoteLines(dto.Lines, dto.CompanyID, note.ID)
		if err := tx.Create(&lines).Error; err != nil {
			return fmt.Errorf("create delivery note lines: %w", err)
		}
		noteID = note.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.FindByID(dto.CompanyID, noteID)
}

func (r *DeliveryNoteRepository) FindByID(companyID, id string) (*model.DeliveryNote, error) {
	var n model.DeliveryNote
	err := r.db.
		Preload("Lines", func(db *gorm.DB) *gorm.DB { return db.Order("line_order ASC") }).
		Where("id = ? AND company_id = ?", id, companyID).First(&n).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &domain.ErrNotFound{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("find delivery note %s: %w", id, err)
	}
	return &n, nil
}

func (r *DeliveryNoteRepository) FindAll(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	q := r.db.Model(&model.DeliveryNote{}).Where("company_id = ?", f.CompanyID)
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where("lower(number) LIKE ? OR lower(notes) LIKE ?", like, like)
	}
	if f.MitraID != "" {
		q = q.Where("mitra_id = ?", f.MitraID)
	}
	if f.SalesOrderID != "" {
		q = q.Where("sales_order_id = ?", f.SalesOrderID)
	}

	sortCol := utils.NormalizeSort(f.Sort, deliveryNoteListColumns, "date")
	order := f.Order
	if strings.TrimSpace(order) == "" {
		order = "DESC"
	}
	return utils.GetPaginatedDataOffset(q, utils.OffsetPaginationOptions{
		Model:                &model.DeliveryNote{},
		Page:                 f.Page,
		Limit:                f.PageSize,
		Sort:                 sortCol,
		Order:                order,
		Select:               f.Fields,
		ValidColumns:         deliveryNoteListColumns,
		PreserveAssociations: true,
	})
}

func (r *DeliveryNoteRepository) MitraExists(companyID, mitraID string) (bool, error) {
	var n int64
	if err := r.db.Model(&model.Mitra{}).Where("id = ? AND company_id = ?", mitraID, companyID).Count(&n).Error; err != nil {
		return false, fmt.Errorf("check mitra: %w", err)
	}
	return n > 0, nil
}

func (r *DeliveryNoteRepository) numberExists(companyID, number string) (bool, error) {
	var n int64
	err := r.db.Model(&model.DeliveryNote{}).
		Where("company_id = ? AND lower(number) = lower(?)", companyID, number).Count(&n).Error
	if err != nil {
		return false, fmt.Errorf("check delivery note number: %w", err)
	}
	return n > 0, nil
}

// generateDeliveryNoteNumber — DN/<YYYY>/<NNNN>, same max-trailing-integer
// scan as generateSalesOrderNumber/generateSalesReceiptNumber.
func generateDeliveryNoteNumber(tx *gorm.DB, companyID string, date time.Time) (string, error) {
	prefix := fmt.Sprintf("DN/%d/", date.Year())

	var numbers []string
	if err := tx.Model(&model.DeliveryNote{}).
		Where("company_id = ? AND number LIKE ?", companyID, prefix+"%").
		Pluck("number", &numbers).Error; err != nil {
		return "", fmt.Errorf("scan delivery note numbers: %w", err)
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

func buildDeliveryNoteLines(lines []domain.LineDTO, companyID, noteID string) []model.DeliveryNoteLine {
	out := make([]model.DeliveryNoteLine, 0, len(lines))
	for i, l := range lines {
		out = append(out, model.DeliveryNoteLine{
			ID:             uuid.NewString(),
			DeliveryNoteID: noteID,
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
