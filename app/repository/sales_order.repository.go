package repository

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/salesorder"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// salesOrderListColumns — fields the MasterTable may show / sort by.
var salesOrderListColumns = []string{
	"number", "date", "status", "mitra_id", "grand_total", "created_at",
}

type SalesOrderRepository struct{ db *gorm.DB }

func NewSalesOrderRepository(db *gorm.DB) domain.IRepository { return &SalesOrderRepository{db: db} }

func (r *SalesOrderRepository) Create(dto *domain.CreateDTO, calc *domain.OrderCalc, actorID string) (*model.SalesOrder, error) {
	date, err := parseSalesOrderDate(dto.Date)
	if err != nil {
		return nil, err
	}

	number := strings.TrimSpace(dto.Number)
	if number != "" {
		if exists, err := r.numberExists(dto.CompanyID, number, ""); err != nil {
			return nil, err
		} else if exists {
			return nil, &domain.ErrNumberExists{Number: number}
		}
	}

	var orderID string
	err = r.db.Transaction(func(tx *gorm.DB) error {
		n := number
		if n == "" {
			gen, err := generateSalesOrderNumber(tx, dto.CompanyID, date)
			if err != nil {
				return err
			}
			n = gen
		}

		order := &model.SalesOrder{
			ID:                       uuid.NewString(),
			CompanyID:                dto.CompanyID,
			MitraID:                  dto.MitraID,
			Number:                   n,
			Date:                     date,
			RefNo:                    strings.TrimSpace(dto.RefNo),
			Notes:                    strings.TrimSpace(dto.Notes),
			Subtotal:                 calc.Subtotal,
			DiscountTotal:            calc.DiscountTotal,
			TaxTotal:                 calc.TaxTotal,
			GrandTotal:               calc.GrandTotal,
			AdditionalDiscountType:   calc.AdditionalDiscountType,
			AdditionalDiscountValue:  calc.AdditionalDiscountValue,
			AdditionalDiscountAmount: calc.AdditionalDiscountAmount,
			ShipFrom:                 strings.TrimSpace(dto.ShipFrom),
			Salesperson:              strings.TrimSpace(dto.Salesperson),
			AttachmentData:           dto.AttachmentData,
			AttachmentName:           strings.TrimSpace(dto.AttachmentName),
			SignatureData:            dto.SignatureData,
			StampDuty:                dto.StampDuty,
			CreatedBy:                actorID,
			UpdatedBy:                actorID,
		}
		if err := tx.Create(order).Error; err != nil {
			return fmt.Errorf("create sales order: %w", err)
		}
		lines := buildSalesOrderLines(calc.Lines, dto.CompanyID, order.ID)
		if err := tx.Create(&lines).Error; err != nil {
			return fmt.Errorf("create sales order lines: %w", err)
		}
		lineTaxes := buildSalesOrderLineTaxes(lines, calc.Lines)
		if len(lineTaxes) > 0 {
			if err := tx.Create(&lineTaxes).Error; err != nil {
				return fmt.Errorf("create sales order line taxes: %w", err)
			}
		}
		orderID = order.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.FindByID(dto.CompanyID, orderID)
}

func (r *SalesOrderRepository) Update(companyID, id string, dto *domain.UpdateDTO, calc *domain.OrderCalc, actorID string) (*model.SalesOrder, error) {
	existing, err := r.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}

	date, err := parseSalesOrderDate(dto.Date)
	if err != nil {
		return nil, err
	}

	number := strings.TrimSpace(dto.Number)
	if number == "" {
		number = existing.Number
	}
	if !strings.EqualFold(number, existing.Number) {
		if exists, err := r.numberExists(companyID, number, id); err != nil {
			return nil, err
		} else if exists {
			return nil, &domain.ErrNumberExists{Number: number}
		}
	}

	err = r.db.Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"mitra_id":                   dto.MitraID,
			"number":                     number,
			"date":                       date,
			"ref_no":                     strings.TrimSpace(dto.RefNo),
			"notes":                      strings.TrimSpace(dto.Notes),
			"subtotal":                   calc.Subtotal,
			"discount_total":             calc.DiscountTotal,
			"tax_total":                  calc.TaxTotal,
			"grand_total":                calc.GrandTotal,
			"additional_discount_type":   calc.AdditionalDiscountType,
			"additional_discount_value":  calc.AdditionalDiscountValue,
			"additional_discount_amount": calc.AdditionalDiscountAmount,
			"ship_from":                  strings.TrimSpace(dto.ShipFrom),
			"salesperson":                strings.TrimSpace(dto.Salesperson),
			"attachment_data":            dto.AttachmentData,
			"attachment_name":            strings.TrimSpace(dto.AttachmentName),
			"signature_data":             dto.SignatureData,
			"stamp_duty":                 dto.StampDuty,
			"updated_at":                 time.Now(),
			"updated_by":                 actorID,
		}
		if err := tx.Model(&model.SalesOrder{}).
			Where("id = ? AND company_id = ?", id, companyID).Updates(updates).Error; err != nil {
			return fmt.Errorf("update sales order %s: %w", id, err)
		}
		if err := deleteSalesOrderLineTaxes(tx, id); err != nil {
			return err
		}
		if err := tx.Where("sales_order_id = ?", id).Delete(&model.SalesOrderLine{}).Error; err != nil {
			return fmt.Errorf("clear sales order lines %s: %w", id, err)
		}
		lines := buildSalesOrderLines(calc.Lines, companyID, id)
		if err := tx.Create(&lines).Error; err != nil {
			return fmt.Errorf("create sales order lines %s: %w", id, err)
		}
		lineTaxes := buildSalesOrderLineTaxes(lines, calc.Lines)
		if len(lineTaxes) > 0 {
			if err := tx.Create(&lineTaxes).Error; err != nil {
				return fmt.Errorf("create sales order line taxes %s: %w", id, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.FindByID(companyID, id)
}

func (r *SalesOrderRepository) FindByID(companyID, id string) (*model.SalesOrder, error) {
	var o model.SalesOrder
	err := r.db.
		Preload("Lines", func(db *gorm.DB) *gorm.DB { return db.Order("line_order ASC") }).
		Where("id = ? AND company_id = ?", id, companyID).First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &domain.ErrNotFound{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("find sales order %s: %w", id, err)
	}
	if err := attachSalesOrderLineTaxes(r.db, o.Lines); err != nil {
		return nil, err
	}
	return &o, nil
}

// FindAll — the sales order list, paginated + sortable like every other
// master list (lines aren't included; the FE reads GrandTotal directly).
func (r *SalesOrderRepository) FindAll(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	q := r.db.Model(&model.SalesOrder{}).Where("company_id = ?", f.CompanyID)
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + strings.ToLower(s) + "%"
		q = q.Where("lower(number) LIKE ? OR lower(notes) LIKE ? OR lower(ref_no) LIKE ?", like, like, like)
	}
	if f.MitraID != "" {
		q = q.Where("mitra_id = ?", f.MitraID)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}

	sortCol := utils.NormalizeSort(f.Sort, salesOrderListColumns, "date")
	order := f.Order
	if strings.TrimSpace(order) == "" {
		order = "DESC"
	}
	return utils.GetPaginatedDataOffset(q, utils.OffsetPaginationOptions{
		Model:                &model.SalesOrder{},
		Page:                 f.Page,
		Limit:                f.PageSize,
		Sort:                 sortCol,
		Order:                order,
		Select:               f.Fields,
		ValidColumns:         salesOrderListColumns,
		PreserveAssociations: true,
	})
}

func (r *SalesOrderRepository) Delete(companyID, id string) error {
	if _, err := r.FindByID(companyID, id); err != nil {
		return err
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := deleteSalesOrderLineTaxes(tx, id); err != nil {
			return err
		}
		if err := tx.Where("sales_order_id = ?", id).Delete(&model.SalesOrderLine{}).Error; err != nil {
			return fmt.Errorf("delete sales order lines %s: %w", id, err)
		}
		if err := tx.Where("id = ? AND company_id = ?", id, companyID).Delete(&model.SalesOrder{}).Error; err != nil {
			return fmt.Errorf("delete sales order %s: %w", id, err)
		}
		return nil
	})
}

func (r *SalesOrderRepository) SetStatus(companyID, id, actorID string, status model.SalesOrderStatus) error {
	res := r.db.Model(&model.SalesOrder{}).Where("id = ? AND company_id = ?", id, companyID).
		Updates(map[string]any{"status": status, "updated_at": time.Now(), "updated_by": actorID})
	if res.Error != nil {
		return fmt.Errorf("set sales order status %s: %w", id, res.Error)
	}
	if res.RowsAffected == 0 {
		return &domain.ErrNotFound{ID: id}
	}
	return nil
}

func (r *SalesOrderRepository) MitraExists(companyID, mitraID string) (bool, error) {
	var n int64
	if err := r.db.Model(&model.Mitra{}).Where("id = ? AND company_id = ?", mitraID, companyID).Count(&n).Error; err != nil {
		return false, fmt.Errorf("check mitra: %w", err)
	}
	return n > 0, nil
}

func (r *SalesOrderRepository) TaxRates(companyID string, taxIDs []string) (map[string]domain.TaxInfo, error) {
	if len(taxIDs) == 0 {
		return map[string]domain.TaxInfo{}, nil
	}
	var rows []model.Tax
	err := r.db.Where("company_id = ? AND id IN ? AND is_active = ?", companyID, taxIDs, utils.BoolInt(true).ToInt()).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("find taxes: %w", err)
	}
	out := make(map[string]domain.TaxInfo, len(rows))
	for _, t := range rows {
		out[t.ID] = domain.TaxInfo{Rate: t.Rate, CalcMethod: string(t.CalcMethod)}
	}
	return out, nil
}

func (r *SalesOrderRepository) numberExists(companyID, number, exceptID string) (bool, error) {
	q := r.db.Model(&model.SalesOrder{}).Where("company_id = ? AND lower(number) = lower(?)", companyID, number)
	if exceptID != "" {
		q = q.Where("id <> ?", exceptID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return false, fmt.Errorf("check sales order number: %w", err)
	}
	return n > 0, nil
}

// generateSalesOrderNumber — SO/<YYYY>/<NNNN>, NNNN = the highest existing
// trailing number for this company+year, plus one (same max-trailing-integer
// scan as generateJournalEntryNumber in journal.repository.go).
func generateSalesOrderNumber(tx *gorm.DB, companyID string, date time.Time) (string, error) {
	prefix := fmt.Sprintf("SO/%d/", date.Year())

	var numbers []string
	if err := tx.Model(&model.SalesOrder{}).
		Where("company_id = ? AND number LIKE ?", companyID, prefix+"%").
		Pluck("number", &numbers).Error; err != nil {
		return "", fmt.Errorf("scan sales order numbers: %w", err)
	}
	return fmt.Sprintf("%s%04d", prefix, nextSalesOrderSequence(numbers)), nil
}

func nextSalesOrderSequence(refs []string) int {
	max := 0
	for _, ref := range refs {
		parts := strings.Split(ref, "/")
		if len(parts) == 0 {
			continue
		}
		if seq, err := strconv.Atoi(parts[len(parts)-1]); err == nil && seq > max {
			max = seq
		}
	}
	return max + 1
}

func buildSalesOrderLines(calc []domain.LineCalc, companyID, orderID string) []model.SalesOrderLine {
	lines := make([]model.SalesOrderLine, 0, len(calc))
	for i, l := range calc {
		lines = append(lines, model.SalesOrderLine{
			ID:            uuid.NewString(),
			SalesOrderID:  orderID,
			CompanyID:     companyID,
			ProductName:   l.ProductName,
			Description:   l.Description,
			Quantity:      l.Quantity,
			UnitPrice:     l.UnitPrice,
			DiscountType:  l.DiscountType,
			DiscountValue: l.DiscountValue,
			LineSubtotal:  l.LineSubtotal,
			LineTaxAmount: l.LineTaxAmount,
			LineTotal:     l.LineTotal,
			LineOrder:     i,
		})
	}
	return lines
}

// buildSalesOrderLineTaxes correlates freshly-inserted lines with the calc
// input they came from by index — the same correlation buildSalesOrderLines
// already relies on for LineOrder.
func buildSalesOrderLineTaxes(lines []model.SalesOrderLine, calc []domain.LineCalc) []model.SalesOrderLineTax {
	var out []model.SalesOrderLineTax
	for i, l := range calc {
		for _, taxID := range l.TaxIDs {
			taxID = strings.TrimSpace(taxID)
			if taxID == "" {
				continue
			}
			out = append(out, model.SalesOrderLineTax{
				ID: uuid.NewString(), SalesOrderLineID: lines[i].ID, TaxID: taxID,
			})
		}
	}
	return out
}

// deleteSalesOrderLineTaxes removes every SalesOrderLineTax row belonging to
// the order's current lines — always called before the lines themselves are
// wiped (Update/Delete), since there's no FK cascade defined on this table.
func deleteSalesOrderLineTaxes(tx *gorm.DB, orderID string) error {
	var lineIDs []string
	if err := tx.Model(&model.SalesOrderLine{}).Where("sales_order_id = ?", orderID).
		Pluck("id", &lineIDs).Error; err != nil {
		return fmt.Errorf("list sales order line ids %s: %w", orderID, err)
	}
	if len(lineIDs) == 0 {
		return nil
	}
	if err := tx.Where("sales_order_line_id IN ?", lineIDs).Delete(&model.SalesOrderLineTax{}).Error; err != nil {
		return fmt.Errorf("delete sales order line taxes %s: %w", orderID, err)
	}
	return nil
}

// attachSalesOrderLineTaxes bulk-loads every line's taxes in one query and
// assigns them back by SalesOrderLineID — no N+1 even with many lines.
func attachSalesOrderLineTaxes(db *gorm.DB, lines []model.SalesOrderLine) error {
	if len(lines) == 0 {
		return nil
	}
	lineIDs := make([]string, len(lines))
	for i, l := range lines {
		lineIDs[i] = l.ID
	}
	var rows []model.SalesOrderLineTax
	if err := db.Where("sales_order_line_id IN ?", lineIDs).Find(&rows).Error; err != nil {
		return fmt.Errorf("find sales order line taxes: %w", err)
	}
	byLine := make(map[string][]string, len(lines))
	for _, r := range rows {
		byLine[r.SalesOrderLineID] = append(byLine[r.SalesOrderLineID], r.TaxID)
	}
	for i := range lines {
		lines[i].TaxIDs = byLine[lines[i].ID]
	}
	return nil
}

func parseSalesOrderDate(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, &domain.ErrValidation{Message: "invalid date (format YYYY-MM-DD)"}
	}
	return t, nil
}

// PreviewNumber returns what the next auto-generated number would be right
// now — a preview for the Add page, not a reservation.
func (r *SalesOrderRepository) PreviewNumber(companyID string) (string, error) {
	return generateSalesOrderNumber(r.db, companyID, time.Now())
}
