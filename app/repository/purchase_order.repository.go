package repository

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/purchaseorder"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// purchaseOrderListColumns — fields the MasterTable may show / sort by.
var purchaseOrderListColumns = []string{
	"number", "date", "status", "mitra_id", "grand_total", "created_at",
}

type PurchaseOrderRepository struct{ db *gorm.DB }

func NewPurchaseOrderRepository(db *gorm.DB) domain.IRepository {
	return &PurchaseOrderRepository{db: db}
}

func (r *PurchaseOrderRepository) Create(dto *domain.CreateDTO, calc *domain.OrderCalc, actorID string) (*model.PurchaseOrder, error) {
	date, err := parsePurchaseOrderDate(dto.Date)
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
			gen, err := generatePurchaseOrderNumber(tx, dto.CompanyID, date)
			if err != nil {
				return err
			}
			n = gen
		}

		order := &model.PurchaseOrder{
			ID:                       uuid.NewString(),
			CompanyID:                dto.CompanyID,
			MitraID:                  dto.MitraID,
			Number:                   n,
			Date:                     date,
			RefNo:                    strings.TrimSpace(dto.RefNo),
			Notes:                    utils.SanitizeRichText(dto.Notes),
			Subtotal:                 calc.Subtotal,
			DiscountTotal:            calc.DiscountTotal,
			TaxTotal:                 calc.TaxTotal,
			GrandTotal:               calc.GrandTotal,
			AdditionalDiscountType:   calc.AdditionalDiscountType,
			AdditionalDiscountValue:  calc.AdditionalDiscountValue,
			AdditionalDiscountAmount: calc.AdditionalDiscountAmount,
			ShipTo:                   strings.TrimSpace(dto.ShipTo),
			AttachmentData:           dto.AttachmentData,
			AttachmentName:           strings.TrimSpace(dto.AttachmentName),
			SignatureData:            dto.SignatureData,
			StampDuty:                dto.StampDuty,
			Template:                 templateForDoc(tx, dto.CompanyID, "purchase_order", dto.Template),
			CreatedBy:                actorID,
			UpdatedBy:                actorID,
		}
		snap, err := contactSnapshot(tx, dto.CompanyID, dto.MitraID, dto.ContactPersonID, "")
		if err != nil {
			if errors.Is(err, errContactInvalid) {
				return &domain.ErrValidation{Message: err.Error()}
			}
			return err
		}
		order.ContactPersonID, order.ContactName, order.ContactPosition, order.ContactPhone, order.ContactEmail = snap.ID, snap.Name, snap.Position, snap.Phone, snap.Email
		if err := tx.Create(order).Error; err != nil {
			return fmt.Errorf("create purchase order: %w", err)
		}
		lines := buildPurchaseOrderLines(calc.Lines, dto.CompanyID, order.ID)
		if err := tx.Create(&lines).Error; err != nil {
			return fmt.Errorf("create purchase order lines: %w", err)
		}
		lineTaxes := buildPurchaseOrderLineTaxes(lines, calc.Lines)
		if len(lineTaxes) > 0 {
			if err := tx.Create(&lineTaxes).Error; err != nil {
				return fmt.Errorf("create purchase order line taxes: %w", err)
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

func (r *PurchaseOrderRepository) Update(companyID, id string, dto *domain.UpdateDTO, calc *domain.OrderCalc, actorID string) (*model.PurchaseOrder, error) {
	existing, err := r.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}

	date, err := parsePurchaseOrderDate(dto.Date)
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
			"notes":                      utils.SanitizeRichText(dto.Notes),
			"subtotal":                   calc.Subtotal,
			"discount_total":             calc.DiscountTotal,
			"tax_total":                  calc.TaxTotal,
			"grand_total":                calc.GrandTotal,
			"additional_discount_type":   calc.AdditionalDiscountType,
			"additional_discount_value":  calc.AdditionalDiscountValue,
			"additional_discount_amount": calc.AdditionalDiscountAmount,
			"ship_to":                    strings.TrimSpace(dto.ShipTo),
			"attachment_data":            dto.AttachmentData,
			"attachment_name":            strings.TrimSpace(dto.AttachmentName),
			"signature_data":             dto.SignatureData,
			"stamp_duty":                 dto.StampDuty,
			"updated_at":                 time.Now(),
			"updated_by":                 actorID,
		}
		if t := strings.TrimSpace(dto.Template); t != "" {
			updates["template"] = t
		}
		snap, err := contactSnapshot(tx, companyID, dto.MitraID, dto.ContactPersonID, derefStr(existing.ContactPersonID))
		if err != nil {
			if errors.Is(err, errContactInvalid) {
				return &domain.ErrValidation{Message: err.Error()}
			}
			return err
		}
		if !snap.Keep {
			updates["contact_person_id"] = snap.ID
			updates["contact_name"] = snap.Name
			updates["contact_position"] = snap.Position
			updates["contact_phone"] = snap.Phone
			updates["contact_email"] = snap.Email
		}

		if err := tx.Model(&model.PurchaseOrder{}).
			Where("id = ? AND company_id = ?", id, companyID).Updates(updates).Error; err != nil {
			return fmt.Errorf("update purchase order %s: %w", id, err)
		}
		if err := deletePurchaseOrderLineTaxes(tx, id); err != nil {
			return err
		}
		if err := tx.Where("purchase_order_id = ?", id).Delete(&model.PurchaseOrderLine{}).Error; err != nil {
			return fmt.Errorf("clear purchase order lines %s: %w", id, err)
		}
		lines := buildPurchaseOrderLines(calc.Lines, companyID, id)
		if err := tx.Create(&lines).Error; err != nil {
			return fmt.Errorf("create purchase order lines %s: %w", id, err)
		}
		lineTaxes := buildPurchaseOrderLineTaxes(lines, calc.Lines)
		if len(lineTaxes) > 0 {
			if err := tx.Create(&lineTaxes).Error; err != nil {
				return fmt.Errorf("create purchase order line taxes %s: %w", id, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.FindByID(companyID, id)
}

func (r *PurchaseOrderRepository) FindByID(companyID, id string) (*model.PurchaseOrder, error) {
	var o model.PurchaseOrder
	err := r.db.
		Preload("Lines", func(db *gorm.DB) *gorm.DB { return db.Order("line_order ASC") }).
		Where("id = ? AND company_id = ?", id, companyID).First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &domain.ErrNotFound{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("find purchase order %s: %w", id, err)
	}
	if err := attachPurchaseOrderLineTaxes(r.db, o.Lines); err != nil {
		return nil, err
	}
	return &o, nil
}

// FindAll — the purchase order list, paginated + sortable like every other
// master list (lines aren't included; the FE reads GrandTotal directly).
func (r *PurchaseOrderRepository) FindAll(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	q := r.db.Model(&model.PurchaseOrder{}).Where("company_id = ?", f.CompanyID)
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

	sortCol := utils.NormalizeSort(f.Sort, purchaseOrderListColumns, "date")
	order := f.Order
	if strings.TrimSpace(order) == "" {
		order = "DESC"
	}
	return utils.GetPaginatedDataOffset(q, utils.OffsetPaginationOptions{
		Model:                &model.PurchaseOrder{},
		Page:                 f.Page,
		Limit:                f.PageSize,
		Sort:                 sortCol,
		Order:                order,
		Select:               f.Fields,
		ValidColumns:         purchaseOrderListColumns,
		Exclude:              listBlobExclude,
		PreserveAssociations: true,
	})
}

// Delete soft-deletes the document ONLY. Its lines (and line taxes) are kept on
// purpose: hard-deleting them would leave a "deleted" document that can never be
// audited or restored intact. Nothing reads lines except through a live parent.
func (r *PurchaseOrderRepository) Delete(companyID, id string) error {
	if _, err := r.FindByID(companyID, id); err != nil {
		return err
	}
	if err := r.db.Where("id = ? AND company_id = ?", id, companyID).Delete(&model.PurchaseOrder{}).Error; err != nil {
		return fmt.Errorf("delete purchase order %s: %w", id, err)
	}
	return nil
}

func (r *PurchaseOrderRepository) SetStatus(companyID, id, actorID string, status model.PurchaseOrderStatus) error {
	res := r.db.Model(&model.PurchaseOrder{}).Where("id = ? AND company_id = ?", id, companyID).
		Updates(map[string]any{"status": status, "updated_at": time.Now(), "updated_by": actorID})
	if res.Error != nil {
		return fmt.Errorf("set purchase order status %s: %w", id, res.Error)
	}
	if res.RowsAffected == 0 {
		return &domain.ErrNotFound{ID: id}
	}
	return nil
}

func (r *PurchaseOrderRepository) MitraExists(companyID, mitraID string) (bool, error) {
	var n int64
	if err := r.db.Model(&model.Mitra{}).Where("id = ? AND company_id = ?", mitraID, companyID).Count(&n).Error; err != nil {
		return false, fmt.Errorf("check mitra: %w", err)
	}
	return n > 0, nil
}

func (r *PurchaseOrderRepository) TaxRates(companyID string, taxIDs []string) (map[string]domain.TaxInfo, error) {
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

func (r *PurchaseOrderRepository) numberExists(companyID, number, exceptID string) (bool, error) {
	q := r.db.Model(&model.PurchaseOrder{}).Where("company_id = ? AND lower(number) = lower(?)", companyID, number)
	if exceptID != "" {
		q = q.Where("id <> ?", exceptID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return false, fmt.Errorf("check purchase order number: %w", err)
	}
	return n > 0, nil
}

// generatePurchaseOrderNumber — PO/<YYYY>/<NNNN>, NNNN = the highest existing
// trailing number for this company+year, plus one (same max-trailing-integer
// scan as generateSalesOrderNumber).
func generatePurchaseOrderNumber(tx *gorm.DB, companyID string, date time.Time) (string, error) {
	prefix := fmt.Sprintf("PO/%d/", date.Year())

	var numbers []string
	if err := tx.Model(&model.PurchaseOrder{}).
		Where("company_id = ? AND number LIKE ?", companyID, prefix+"%").
		Pluck("number", &numbers).Error; err != nil {
		return "", fmt.Errorf("scan purchase order numbers: %w", err)
	}
	return fmt.Sprintf("%s%04d", prefix, nextPurchaseOrderSequence(numbers)), nil
}

func nextPurchaseOrderSequence(refs []string) int {
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

func buildPurchaseOrderLines(calc []domain.LineCalc, companyID, orderID string) []model.PurchaseOrderLine {
	lines := make([]model.PurchaseOrderLine, 0, len(calc))
	for i, l := range calc {
		lines = append(lines, model.PurchaseOrderLine{
			ID:              uuid.NewString(),
			PurchaseOrderID: orderID,
			CompanyID:       companyID,
			ProductName:     l.ProductName,
			Description:     l.Description,
			Quantity:        l.Quantity,
			UnitPrice:       l.UnitPrice,
			DiscountType:    l.DiscountType,
			DiscountValue:   l.DiscountValue,
			LineSubtotal:    l.LineSubtotal,
			LineTaxAmount:   l.LineTaxAmount,
			LineTotal:       l.LineTotal,
			LineOrder:       i,
		})
	}
	return lines
}

// buildPurchaseOrderLineTaxes correlates freshly-inserted lines with the calc
// input they came from by index — the same correlation buildPurchaseOrderLines
// already relies on for LineOrder.
func buildPurchaseOrderLineTaxes(lines []model.PurchaseOrderLine, calc []domain.LineCalc) []model.PurchaseOrderLineTax {
	var out []model.PurchaseOrderLineTax
	for i, l := range calc {
		for _, taxID := range l.TaxIDs {
			taxID = strings.TrimSpace(taxID)
			if taxID == "" {
				continue
			}
			out = append(out, model.PurchaseOrderLineTax{
				ID: uuid.NewString(), PurchaseOrderLineID: lines[i].ID, TaxID: taxID,
			})
		}
	}
	return out
}

// deletePurchaseOrderLineTaxes removes every PurchaseOrderLineTax row
// belonging to the order's current lines — always called before the lines
// themselves are wiped (Update/Delete), since there's no FK cascade defined
// on this table.
func deletePurchaseOrderLineTaxes(tx *gorm.DB, orderID string) error {
	var lineIDs []string
	if err := tx.Model(&model.PurchaseOrderLine{}).Where("purchase_order_id = ?", orderID).
		Pluck("id", &lineIDs).Error; err != nil {
		return fmt.Errorf("list purchase order line ids %s: %w", orderID, err)
	}
	if len(lineIDs) == 0 {
		return nil
	}
	if err := tx.Where("purchase_order_line_id IN ?", lineIDs).Delete(&model.PurchaseOrderLineTax{}).Error; err != nil {
		return fmt.Errorf("delete purchase order line taxes %s: %w", orderID, err)
	}
	return nil
}

// attachPurchaseOrderLineTaxes bulk-loads every line's taxes in one query and
// assigns them back by PurchaseOrderLineID — no N+1 even with many lines.
func attachPurchaseOrderLineTaxes(db *gorm.DB, lines []model.PurchaseOrderLine) error {
	if len(lines) == 0 {
		return nil
	}
	lineIDs := make([]string, len(lines))
	for i, l := range lines {
		lineIDs[i] = l.ID
	}
	var rows []model.PurchaseOrderLineTax
	if err := db.Where("purchase_order_line_id IN ?", lineIDs).Find(&rows).Error; err != nil {
		return fmt.Errorf("find purchase order line taxes: %w", err)
	}
	byLine := make(map[string][]string, len(lines))
	for _, r := range rows {
		byLine[r.PurchaseOrderLineID] = append(byLine[r.PurchaseOrderLineID], r.TaxID)
	}
	for i := range lines {
		lines[i].TaxIDs = byLine[lines[i].ID]
	}
	return nil
}

func parsePurchaseOrderDate(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, &domain.ErrValidation{Message: "invalid date (format YYYY-MM-DD)"}
	}
	return t, nil
}

// PreviewNumber returns what the next auto-generated number would be right
// now — a preview for the Add page, not a reservation.
func (r *PurchaseOrderRepository) PreviewNumber(companyID string) (string, error) {
	return generatePurchaseOrderNumber(r.db, companyID, time.Now())
}

// SetTemplate changes ONLY the layout of an existing document. Allowed in any status: it is
// presentation, so it never touches lines, totals or the document lifecycle.
func (r *PurchaseOrderRepository) SetTemplate(companyID, id, actorID, template string) error {
	res := r.db.Model(&model.PurchaseOrder{}).Where("id = ? AND company_id = ?", id, companyID).
		Updates(map[string]any{"template": template, "updated_at": time.Now(), "updated_by": actorID})
	if res.Error != nil {
		return fmt.Errorf("set purchase order template %s: %w", id, res.Error)
	}
	if res.RowsAffected == 0 {
		return &domain.ErrNotFound{ID: id}
	}
	return nil
}

// CountCreatedSince — every purchase order created on or after `since`. Used for the Free-tier
// transactions/month limit.
func (r *PurchaseOrderRepository) CountCreatedSince(companyID string, since time.Time) (int64, error) {
	var n int64
	err := r.db.Model(&model.PurchaseOrder{}).Where("company_id = ? AND created_at >= ?", companyID, since).Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("count purchase orders since: %w", err)
	}
	return n, nil
}
