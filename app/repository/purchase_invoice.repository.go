package repository

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/purchaseinvoice"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// purchaseInvoiceListColumns — fields the MasterTable may show / sort by.
var purchaseInvoiceListColumns = []string{
	"number", "date", "due_date", "status", "mitra_id", "grand_total", "created_at",
}

type PurchaseInvoiceRepository struct{ db *gorm.DB }

func NewPurchaseInvoiceRepository(db *gorm.DB) domain.IRepository {
	return &PurchaseInvoiceRepository{db: db}
}

func (r *PurchaseInvoiceRepository) Create(dto *domain.CreateDTO, calc *utils.LinesCalc, actorID string) (*model.PurchaseInvoice, error) {
	date, err := parsePurchaseInvoiceDate(dto.Date)
	if err != nil {
		return nil, err
	}
	dueDate, err := parseOptionalPurchaseInvoiceDate(dto.DueDate)
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

	var invoiceID string
	err = r.db.Transaction(func(tx *gorm.DB) error {
		n := number
		if n == "" {
			gen, err := generatePurchaseInvoiceNumber(tx, dto.CompanyID, date)
			if err != nil {
				return err
			}
			n = gen
		}

		invoice := &model.PurchaseInvoice{
			ID:                       uuid.NewString(),
			CompanyID:                dto.CompanyID,
			PurchaseOrderID:          trimPtr(dto.PurchaseOrderID),
			MitraID:                  dto.MitraID,
			Number:                   n,
			Date:                     date,
			DueDate:                  dueDate,
			RefNo:                    strings.TrimSpace(dto.RefNo),
			Notes:                    strings.TrimSpace(dto.Notes),
			Subtotal:                 calc.Subtotal,
			DiscountTotal:            calc.DiscountTotal,
			TaxTotal:                 calc.TaxTotal,
			GrandTotal:               calc.GrandTotal,
			AdditionalDiscountType:   calc.AdditionalDiscountType,
			AdditionalDiscountValue:  calc.AdditionalDiscountValue,
			AdditionalDiscountAmount: calc.AdditionalDiscountAmount,
			ShippingCost:             dto.ShippingCost,
			ShipTo:                   strings.TrimSpace(dto.ShipTo),
			AttachmentData:           dto.AttachmentData,
			AttachmentName:           strings.TrimSpace(dto.AttachmentName),
			SignatureData:            dto.SignatureData,
			StampDuty:                dto.StampDuty,
			CreatedBy:                actorID,
			UpdatedBy:                actorID,
		}
		if err := tx.Create(invoice).Error; err != nil {
			return fmt.Errorf("create purchase invoice: %w", err)
		}
		lines := buildPurchaseInvoiceLines(calc.Lines, dto.CompanyID, invoice.ID)
		if err := tx.Create(&lines).Error; err != nil {
			return fmt.Errorf("create purchase invoice lines: %w", err)
		}
		lineTaxes := buildPurchaseInvoiceLineTaxes(lines, calc.Lines)
		if len(lineTaxes) > 0 {
			if err := tx.Create(&lineTaxes).Error; err != nil {
				return fmt.Errorf("create purchase invoice line taxes: %w", err)
			}
		}
		invoiceID = invoice.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.FindByID(dto.CompanyID, invoiceID)
}

func (r *PurchaseInvoiceRepository) Update(companyID, id string, dto *domain.UpdateDTO, calc *utils.LinesCalc, actorID string) (*model.PurchaseInvoice, error) {
	existing, err := r.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}

	date, err := parsePurchaseInvoiceDate(dto.Date)
	if err != nil {
		return nil, err
	}
	dueDate, err := parseOptionalPurchaseInvoiceDate(dto.DueDate)
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
			"purchase_order_id":          trimPtr(dto.PurchaseOrderID),
			"mitra_id":                   dto.MitraID,
			"number":                     number,
			"date":                       date,
			"due_date":                   dueDate,
			"ref_no":                     strings.TrimSpace(dto.RefNo),
			"notes":                      strings.TrimSpace(dto.Notes),
			"subtotal":                   calc.Subtotal,
			"discount_total":             calc.DiscountTotal,
			"tax_total":                  calc.TaxTotal,
			"grand_total":                calc.GrandTotal,
			"additional_discount_type":   calc.AdditionalDiscountType,
			"additional_discount_value":  calc.AdditionalDiscountValue,
			"additional_discount_amount": calc.AdditionalDiscountAmount,
			"shipping_cost":              dto.ShippingCost,
			"ship_to":                    strings.TrimSpace(dto.ShipTo),
			"attachment_data":            dto.AttachmentData,
			"attachment_name":            strings.TrimSpace(dto.AttachmentName),
			"signature_data":             dto.SignatureData,
			"stamp_duty":                 dto.StampDuty,
			"updated_at":                 time.Now(),
			"updated_by":                 actorID,
		}
		if err := tx.Model(&model.PurchaseInvoice{}).
			Where("id = ? AND company_id = ?", id, companyID).Updates(updates).Error; err != nil {
			return fmt.Errorf("update purchase invoice %s: %w", id, err)
		}
		if err := deletePurchaseInvoiceLineTaxes(tx, id); err != nil {
			return err
		}
		if err := tx.Where("purchase_invoice_id = ?", id).Delete(&model.PurchaseInvoiceLine{}).Error; err != nil {
			return fmt.Errorf("clear purchase invoice lines %s: %w", id, err)
		}
		lines := buildPurchaseInvoiceLines(calc.Lines, companyID, id)
		if err := tx.Create(&lines).Error; err != nil {
			return fmt.Errorf("create purchase invoice lines %s: %w", id, err)
		}
		lineTaxes := buildPurchaseInvoiceLineTaxes(lines, calc.Lines)
		if len(lineTaxes) > 0 {
			if err := tx.Create(&lineTaxes).Error; err != nil {
				return fmt.Errorf("create purchase invoice line taxes %s: %w", id, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.FindByID(companyID, id)
}

func (r *PurchaseInvoiceRepository) FindByID(companyID, id string) (*model.PurchaseInvoice, error) {
	var o model.PurchaseInvoice
	err := r.db.
		Preload("Lines", func(db *gorm.DB) *gorm.DB { return db.Order("line_order ASC") }).
		Where("id = ? AND company_id = ?", id, companyID).First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &domain.ErrNotFound{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("find purchase invoice %s: %w", id, err)
	}
	if err := attachPurchaseInvoiceLineTaxes(r.db, o.Lines); err != nil {
		return nil, err
	}
	return &o, nil
}

// FindAll — the invoice list, paginated + sortable like every other master
// list (lines aren't included; the FE reads GrandTotal directly).
func (r *PurchaseInvoiceRepository) FindAll(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	q := r.db.Model(&model.PurchaseInvoice{}).Where("company_id = ?", f.CompanyID)
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

	sortCol := utils.NormalizeSort(f.Sort, purchaseInvoiceListColumns, "date")
	order := f.Order
	if strings.TrimSpace(order) == "" {
		order = "DESC"
	}
	return utils.GetPaginatedDataOffset(q, utils.OffsetPaginationOptions{
		Model:                &model.PurchaseInvoice{},
		Page:                 f.Page,
		Limit:                f.PageSize,
		Sort:                 sortCol,
		Order:                order,
		Select:               f.Fields,
		ValidColumns:         purchaseInvoiceListColumns,
		PreserveAssociations: true,
	})
}

func (r *PurchaseInvoiceRepository) Delete(companyID, id string) error {
	if _, err := r.FindByID(companyID, id); err != nil {
		return err
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := deletePurchaseInvoiceLineTaxes(tx, id); err != nil {
			return err
		}
		if err := tx.Where("purchase_invoice_id = ?", id).Delete(&model.PurchaseInvoiceLine{}).Error; err != nil {
			return fmt.Errorf("delete purchase invoice lines %s: %w", id, err)
		}
		if err := tx.Where("id = ? AND company_id = ?", id, companyID).Delete(&model.PurchaseInvoice{}).Error; err != nil {
			return fmt.Errorf("delete purchase invoice %s: %w", id, err)
		}
		return nil
	})
}

func (r *PurchaseInvoiceRepository) SetStatus(companyID, id, actorID string, status model.PurchaseInvoiceStatus) error {
	res := r.db.Model(&model.PurchaseInvoice{}).Where("id = ? AND company_id = ?", id, companyID).
		Updates(map[string]any{"status": status, "updated_at": time.Now(), "updated_by": actorID})
	if res.Error != nil {
		return fmt.Errorf("set purchase invoice status %s: %w", id, res.Error)
	}
	if res.RowsAffected == 0 {
		return &domain.ErrNotFound{ID: id}
	}
	return nil
}

func (r *PurchaseInvoiceRepository) MitraExists(companyID, mitraID string) (bool, error) {
	var n int64
	if err := r.db.Model(&model.Mitra{}).Where("id = ? AND company_id = ?", mitraID, companyID).Count(&n).Error; err != nil {
		return false, fmt.Errorf("check mitra: %w", err)
	}
	return n > 0, nil
}

func (r *PurchaseInvoiceRepository) TaxRates(companyID string, taxIDs []string) (map[string]utils.TaxRate, error) {
	if len(taxIDs) == 0 {
		return map[string]utils.TaxRate{}, nil
	}
	var rows []model.Tax
	err := r.db.Where("company_id = ? AND id IN ? AND is_active = ?", companyID, taxIDs, utils.BoolInt(true).ToInt()).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("find taxes: %w", err)
	}
	out := make(map[string]utils.TaxRate, len(rows))
	for _, t := range rows {
		out[t.ID] = utils.TaxRate{Rate: t.Rate, CalcMethod: string(t.CalcMethod)}
	}
	return out, nil
}

func (r *PurchaseInvoiceRepository) numberExists(companyID, number, exceptID string) (bool, error) {
	q := r.db.Model(&model.PurchaseInvoice{}).Where("company_id = ? AND lower(number) = lower(?)", companyID, number)
	if exceptID != "" {
		q = q.Where("id <> ?", exceptID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return false, fmt.Errorf("check purchase invoice number: %w", err)
	}
	return n > 0, nil
}

// generatePurchaseInvoiceNumber — BILL/<YYYY>/<NNNN>, same max-trailing-integer
// scan as generateSalesOrderNumber.
func generatePurchaseInvoiceNumber(tx *gorm.DB, companyID string, date time.Time) (string, error) {
	prefix := fmt.Sprintf("BILL/%d/", date.Year())

	var numbers []string
	if err := tx.Model(&model.PurchaseInvoice{}).
		Where("company_id = ? AND number LIKE ?", companyID, prefix+"%").
		Pluck("number", &numbers).Error; err != nil {
		return "", fmt.Errorf("scan purchase invoice numbers: %w", err)
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

func buildPurchaseInvoiceLines(calc []utils.LineResult, companyID, invoiceID string) []model.PurchaseInvoiceLine {
	lines := make([]model.PurchaseInvoiceLine, 0, len(calc))
	for i, l := range calc {
		lines = append(lines, model.PurchaseInvoiceLine{
			ID:                uuid.NewString(),
			PurchaseInvoiceID: invoiceID,
			CompanyID:         companyID,
			ProductName:       l.ProductName,
			Description:       l.Description,
			Quantity:          l.Quantity,
			UnitPrice:         l.UnitPrice,
			DiscountType:      l.DiscountType,
			DiscountValue:     l.DiscountValue,
			LineSubtotal:      l.LineSubtotal,
			LineTaxAmount:     l.LineTaxAmount,
			LineTotal:         l.LineTotal,
			LineOrder:         i,
		})
	}
	return lines
}

// buildPurchaseInvoiceLineTaxes correlates freshly-inserted lines with the
// calc input they came from by index — the same correlation
// buildPurchaseInvoiceLines already relies on for LineOrder.
func buildPurchaseInvoiceLineTaxes(lines []model.PurchaseInvoiceLine, calc []utils.LineResult) []model.PurchaseInvoiceLineTax {
	var out []model.PurchaseInvoiceLineTax
	for i, l := range calc {
		for _, taxID := range l.TaxIDs {
			taxID = strings.TrimSpace(taxID)
			if taxID == "" {
				continue
			}
			out = append(out, model.PurchaseInvoiceLineTax{
				ID: uuid.NewString(), PurchaseInvoiceLineID: lines[i].ID, TaxID: taxID,
			})
		}
	}
	return out
}

// deletePurchaseInvoiceLineTaxes removes every PurchaseInvoiceLineTax row
// belonging to the invoice's current lines — always called before the lines
// themselves are wiped (Update/Delete), since there's no FK cascade defined
// on this table.
func deletePurchaseInvoiceLineTaxes(tx *gorm.DB, invoiceID string) error {
	var lineIDs []string
	if err := tx.Model(&model.PurchaseInvoiceLine{}).Where("purchase_invoice_id = ?", invoiceID).
		Pluck("id", &lineIDs).Error; err != nil {
		return fmt.Errorf("list purchase invoice line ids %s: %w", invoiceID, err)
	}
	if len(lineIDs) == 0 {
		return nil
	}
	if err := tx.Where("purchase_invoice_line_id IN ?", lineIDs).Delete(&model.PurchaseInvoiceLineTax{}).Error; err != nil {
		return fmt.Errorf("delete purchase invoice line taxes %s: %w", invoiceID, err)
	}
	return nil
}

// attachPurchaseInvoiceLineTaxes bulk-loads every line's taxes in one query
// and assigns them back by PurchaseInvoiceLineID — no N+1 even with many lines.
func attachPurchaseInvoiceLineTaxes(db *gorm.DB, lines []model.PurchaseInvoiceLine) error {
	if len(lines) == 0 {
		return nil
	}
	lineIDs := make([]string, len(lines))
	for i, l := range lines {
		lineIDs[i] = l.ID
	}
	var rows []model.PurchaseInvoiceLineTax
	if err := db.Where("purchase_invoice_line_id IN ?", lineIDs).Find(&rows).Error; err != nil {
		return fmt.Errorf("find purchase invoice line taxes: %w", err)
	}
	byLine := make(map[string][]string, len(lines))
	for _, r := range rows {
		byLine[r.PurchaseInvoiceLineID] = append(byLine[r.PurchaseInvoiceLineID], r.TaxID)
	}
	for i := range lines {
		lines[i].TaxIDs = byLine[lines[i].ID]
	}
	return nil
}

func parsePurchaseInvoiceDate(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, &domain.ErrValidation{Message: "invalid date (format YYYY-MM-DD)"}
	}
	return t, nil
}

func parseOptionalPurchaseInvoiceDate(s string) (*time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	t, err := parsePurchaseInvoiceDate(s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// PreviewNumber returns what the next auto-generated number would be right
// now — a preview for the Add page, not a reservation.
func (r *PurchaseInvoiceRepository) PreviewNumber(companyID string) (string, error) {
	return generatePurchaseInvoiceNumber(r.db, companyID, time.Now())
}
