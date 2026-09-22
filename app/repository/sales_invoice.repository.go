package repository

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	domain "duluin_invoice/app/domain/salesinvoice"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// salesInvoiceListColumns — fields the MasterTable may show / sort by.
var salesInvoiceListColumns = []string{
	"number", "date", "due_date", "status", "payment_status", "kind", "mitra_id", "grand_total", "created_at",
}

type SalesInvoiceRepository struct{ db *gorm.DB }

func NewSalesInvoiceRepository(db *gorm.DB) domain.IRepository {
	return &SalesInvoiceRepository{db: db}
}

func (r *SalesInvoiceRepository) Create(dto *domain.CreateDTO, calc *utils.LinesCalc, actorID string) (*model.SalesInvoice, error) {
	date, err := parseSalesInvoiceDate(dto.Date)
	if err != nil {
		return nil, err
	}
	dueDate, err := parseOptionalSalesInvoiceDate(dto.DueDate)
	if err != nil {
		return nil, err
	}
	if dueDate != nil && dueDate.Before(date) {
		return nil, &domain.ErrValidation{Message: "due date can't be earlier than the invoice date"}
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
			gen, err := generateSalesInvoiceNumber(tx, dto.CompanyID, model.SalesInvoiceKind(dto.Kind), date)
			if err != nil {
				return err
			}
			n = gen
		}

		invoice := &model.SalesInvoice{
			ID:                       uuid.NewString(),
			CompanyID:                dto.CompanyID,
			SalesOrderID:             trimPtr(dto.SalesOrderID),
			LinkedInvoiceID:          trimPtr(dto.LinkedInvoiceID),
			MitraID:                  dto.MitraID,
			Kind:                     model.SalesInvoiceKind(dto.Kind),
			Number:                   n,
			Date:                     date,
			DueDate:                  dueDate,
			RefNo:                    strings.TrimSpace(dto.RefNo),
			Notes:                    utils.SanitizeRichText(dto.Notes),
			Terms:                    utils.SanitizeRichText(dto.Terms),
			Subtotal:                 calc.Subtotal,
			DiscountTotal:            calc.DiscountTotal,
			TaxTotal:                 calc.TaxTotal,
			GrandTotal:               calc.GrandTotal,
			AdditionalDiscountType:   calc.AdditionalDiscountType,
			AdditionalDiscountValue:  calc.AdditionalDiscountValue,
			AdditionalDiscountAmount: calc.AdditionalDiscountAmount,
			ShippingCost:             dto.ShippingCost,
			ShipFrom:                 strings.TrimSpace(dto.ShipFrom),
			Salesperson:              strings.TrimSpace(dto.Salesperson),
			AttachmentData:           dto.AttachmentData,
			AttachmentName:           strings.TrimSpace(dto.AttachmentName),
			SignatureData:            dto.SignatureData,
			StampDuty:                dto.StampDuty,
			Template:                 templateFor(tx, dto),
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
		invoice.ContactPersonID, invoice.ContactName, invoice.ContactPosition, invoice.ContactPhone, invoice.ContactEmail = snap.ID, snap.Name, snap.Position, snap.Phone, snap.Email
		if err := tx.Create(invoice).Error; err != nil {
			return fmt.Errorf("create sales invoice: %w", err)
		}
		lines := buildSalesInvoiceLines(calc.Lines, dto.CompanyID, invoice.ID)
		if err := tx.Create(&lines).Error; err != nil {
			return fmt.Errorf("create sales invoice lines: %w", err)
		}
		lineTaxes := buildSalesInvoiceLineTaxes(lines, calc.Lines)
		if len(lineTaxes) > 0 {
			if err := tx.Create(&lineTaxes).Error; err != nil {
				return fmt.Errorf("create sales invoice line taxes: %w", err)
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

func (r *SalesInvoiceRepository) Update(companyID, id string, dto *domain.UpdateDTO, calc *utils.LinesCalc, actorID string) (*model.SalesInvoice, error) {
	existing, err := r.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}

	date, err := parseSalesInvoiceDate(dto.Date)
	if err != nil {
		return nil, err
	}
	dueDate, err := parseOptionalSalesInvoiceDate(dto.DueDate)
	if err != nil {
		return nil, err
	}
	if dueDate != nil && dueDate.Before(date) {
		return nil, &domain.ErrValidation{Message: "due date can't be earlier than the invoice date"}
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
			"sales_order_id":             trimPtr(dto.SalesOrderID),
			"linked_invoice_id":          trimPtr(dto.LinkedInvoiceID),
			"mitra_id":                   dto.MitraID,
			"number":                     number,
			"date":                       date,
			"due_date":                   dueDate,
			"ref_no":                     strings.TrimSpace(dto.RefNo),
			"notes":                      utils.SanitizeRichText(dto.Notes),
			"terms":                      utils.SanitizeRichText(dto.Terms),
			"subtotal":                   calc.Subtotal,
			"discount_total":             calc.DiscountTotal,
			"tax_total":                  calc.TaxTotal,
			"grand_total":                calc.GrandTotal,
			"additional_discount_type":   calc.AdditionalDiscountType,
			"additional_discount_value":  calc.AdditionalDiscountValue,
			"additional_discount_amount": calc.AdditionalDiscountAmount,
			"shipping_cost":              dto.ShippingCost,
			"ship_from":                  strings.TrimSpace(dto.ShipFrom),
			"salesperson":                strings.TrimSpace(dto.Salesperson),
			"attachment_data":            dto.AttachmentData,
			"attachment_name":            strings.TrimSpace(dto.AttachmentName),
			"signature_data":             dto.SignatureData,
			"stamp_duty":                 dto.StampDuty,
			"updated_at":                 time.Now(),
			"updated_by":                 actorID,
		}
		// A client that doesn't send `template` must not reset the saved choice.
		if strings.TrimSpace(dto.Template) != "" {
			updates["template"] = strings.TrimSpace(dto.Template)
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
		if err := tx.Model(&model.SalesInvoice{}).
			Where("id = ? AND company_id = ?", id, companyID).Updates(updates).Error; err != nil {
			return fmt.Errorf("update sales invoice %s: %w", id, err)
		}
		// The total may have changed: paid stays as recorded, so re-derive the payment status
		// (outstanding = max(total - paid, 0) is derived on read).
		if err := refreshSalesInvoicePaymentStatus(tx, companyID, id); err != nil {
			return err
		}
		if err := deleteSalesInvoiceLineTaxes(tx, id); err != nil {
			return err
		}
		if err := tx.Where("sales_invoice_id = ?", id).Delete(&model.SalesInvoiceLine{}).Error; err != nil {
			return fmt.Errorf("clear sales invoice lines %s: %w", id, err)
		}
		lines := buildSalesInvoiceLines(calc.Lines, companyID, id)
		if err := tx.Create(&lines).Error; err != nil {
			return fmt.Errorf("create sales invoice lines %s: %w", id, err)
		}
		lineTaxes := buildSalesInvoiceLineTaxes(lines, calc.Lines)
		if len(lineTaxes) > 0 {
			if err := tx.Create(&lineTaxes).Error; err != nil {
				return fmt.Errorf("create sales invoice line taxes %s: %w", id, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.FindByID(companyID, id)
}

func (r *SalesInvoiceRepository) FindByID(companyID, id string) (*model.SalesInvoice, error) {
	var o model.SalesInvoice
	err := r.db.
		Preload("Lines", func(db *gorm.DB) *gorm.DB { return db.Order("line_order ASC") }).
		Where("id = ? AND company_id = ?", id, companyID).First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &domain.ErrNotFound{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("find sales invoice %s: %w", id, err)
	}
	if err := attachSalesInvoiceLineTaxes(r.db, o.Lines); err != nil {
		return nil, err
	}
	return &o, nil
}

// FindAll — the invoice list, paginated + sortable, scoped by Kind so
// "Invoice Penjualan" and "Invoice Uang Muka" each see only their own rows.
func (r *SalesInvoiceRepository) FindAll(f *domain.Filter) (*utils.OffsetPaginationResult, error) {
	q := r.db.Model(&model.SalesInvoice{}).Where("company_id = ?", f.CompanyID)
	if f.Kind != "" {
		q = q.Where("kind = ?", f.Kind)
	}
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
	if f.PaymentStatus != "" {
		// comma-separated: "unpaid,partially_paid" = everything still owing
		q = q.Where("payment_status IN ?", strings.Split(f.PaymentStatus, ","))
	}
	if f.Overdue {
		q = q.Where("status = ? AND payment_status <> ? AND due_date IS NOT NULL AND due_date < ?",
			model.SalesInvoiceStatusConfirmed, model.SalesInvoicePaymentPaid, time.Now().Format("2006-01-02"))
	}

	sortCol := utils.NormalizeSort(f.Sort, salesInvoiceListColumns, "date")
	order := f.Order
	if strings.TrimSpace(order) == "" {
		order = "DESC"
	}
	return utils.GetPaginatedDataOffset(q, utils.OffsetPaginationOptions{
		Model:                &model.SalesInvoice{},
		Page:                 f.Page,
		Limit:                f.PageSize,
		Sort:                 sortCol,
		Order:                order,
		Select:               f.Fields,
		ValidColumns:         salesInvoiceListColumns,
		PreserveAssociations: true,
	})
}

// Delete soft-deletes the document ONLY. Its lines (and line taxes) are kept on
// purpose: hard-deleting them would leave a "deleted" document that can never be
// audited or restored intact. Nothing reads lines except through a live parent.
func (r *SalesInvoiceRepository) Delete(companyID, id string) error {
	if _, err := r.FindByID(companyID, id); err != nil {
		return err
	}
	// Soft delete (deleted_at). Receipts / payments that were applied keep their own records; the
	// invoice just leaves the active lists, summaries and outstanding figures.
	if err := r.db.Where("id = ? AND company_id = ?", id, companyID).Delete(&model.SalesInvoice{}).Error; err != nil {
		return fmt.Errorf("delete sales invoice %s: %w", id, err)
	}
	return nil
}

func (r *SalesInvoiceRepository) SetStatus(companyID, id, actorID string, status model.SalesInvoiceStatus) error {
	// Reverting to draft reopens the invoice for editing/deletion — never while
	// payments or receipt allocations are applied to it.
	if status == model.SalesInvoiceStatusDraft {
		if err := checkInvoiceHasNoPayments(r.db, companyID, id); err != nil {
			return err
		}
	}
	res := r.db.Model(&model.SalesInvoice{}).Where("id = ? AND company_id = ?", id, companyID).
		Updates(map[string]any{"status": status, "updated_at": time.Now(), "updated_by": actorID})
	if res.Error != nil {
		return fmt.Errorf("set sales invoice status %s: %w", id, res.Error)
	}
	if res.RowsAffected == 0 {
		return &domain.ErrNotFound{ID: id}
	}
	return nil
}

func (r *SalesInvoiceRepository) MitraExists(companyID, mitraID string) (bool, error) {
	var n int64
	if err := r.db.Model(&model.Mitra{}).Where("id = ? AND company_id = ?", mitraID, companyID).Count(&n).Error; err != nil {
		return false, fmt.Errorf("check mitra: %w", err)
	}
	return n > 0, nil
}

func (r *SalesInvoiceRepository) TaxRates(companyID string, taxIDs []string) (map[string]utils.TaxRate, error) {
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

func (r *SalesInvoiceRepository) numberExists(companyID, number, exceptID string) (bool, error) {
	q := r.db.Model(&model.SalesInvoice{}).Where("company_id = ? AND lower(number) = lower(?)", companyID, number)
	if exceptID != "" {
		q = q.Where("id <> ?", exceptID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return false, fmt.Errorf("check sales invoice number: %w", err)
	}
	return n > 0, nil
}

// generateSalesInvoiceNumber — DP/<YYYY>/<NNNN> or INV/<YYYY>/<NNNN> depending
// on Kind, same max-trailing-integer scan as generateSalesOrderNumber.
func generateSalesInvoiceNumber(tx *gorm.DB, companyID string, kind model.SalesInvoiceKind, date time.Time) (string, error) {
	label := "INV"
	if kind == model.SalesInvoiceKindDownPayment {
		label = "DP"
	}
	prefix := fmt.Sprintf("%s/%d/", label, date.Year())

	var numbers []string
	if err := tx.Model(&model.SalesInvoice{}).
		Where("company_id = ? AND kind = ? AND number LIKE ?", companyID, kind, prefix+"%").
		Pluck("number", &numbers).Error; err != nil {
		return "", fmt.Errorf("scan sales invoice numbers: %w", err)
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

func buildSalesInvoiceLines(calc []utils.LineResult, companyID, invoiceID string) []model.SalesInvoiceLine {
	lines := make([]model.SalesInvoiceLine, 0, len(calc))
	for i, l := range calc {
		lines = append(lines, model.SalesInvoiceLine{
			ID:             uuid.NewString(),
			SalesInvoiceID: invoiceID,
			CompanyID:      companyID,
			ProductName:    l.ProductName,
			Description:    l.Description,
			Quantity:       l.Quantity,
			UnitPrice:      l.UnitPrice,
			DiscountType:   l.DiscountType,
			DiscountValue:  l.DiscountValue,
			LineSubtotal:   l.LineSubtotal,
			LineTaxAmount:  l.LineTaxAmount,
			LineTotal:      l.LineTotal,
			LineOrder:      i,
		})
	}
	return lines
}

// buildSalesInvoiceLineTaxes correlates freshly-inserted lines with the calc
// input they came from by index — the same correlation buildSalesInvoiceLines
// already relies on for LineOrder.
func buildSalesInvoiceLineTaxes(lines []model.SalesInvoiceLine, calc []utils.LineResult) []model.SalesInvoiceLineTax {
	var out []model.SalesInvoiceLineTax
	for i, l := range calc {
		for _, taxID := range l.TaxIDs {
			taxID = strings.TrimSpace(taxID)
			if taxID == "" {
				continue
			}
			out = append(out, model.SalesInvoiceLineTax{
				ID: uuid.NewString(), SalesInvoiceLineID: lines[i].ID, TaxID: taxID,
			})
		}
	}
	return out
}

// deleteSalesInvoiceLineTaxes removes every SalesInvoiceLineTax row
// belonging to the invoice's current lines — always called before the lines
// themselves are wiped (Update/Delete), since there's no FK cascade defined
// on this table.
func deleteSalesInvoiceLineTaxes(tx *gorm.DB, invoiceID string) error {
	var lineIDs []string
	if err := tx.Model(&model.SalesInvoiceLine{}).Where("sales_invoice_id = ?", invoiceID).
		Pluck("id", &lineIDs).Error; err != nil {
		return fmt.Errorf("list sales invoice line ids %s: %w", invoiceID, err)
	}
	if len(lineIDs) == 0 {
		return nil
	}
	if err := tx.Where("sales_invoice_line_id IN ?", lineIDs).Delete(&model.SalesInvoiceLineTax{}).Error; err != nil {
		return fmt.Errorf("delete sales invoice line taxes %s: %w", invoiceID, err)
	}
	return nil
}

// attachSalesInvoiceLineTaxes bulk-loads every line's taxes in one query and
// assigns them back by SalesInvoiceLineID — no N+1 even with many lines.
func attachSalesInvoiceLineTaxes(db *gorm.DB, lines []model.SalesInvoiceLine) error {
	if len(lines) == 0 {
		return nil
	}
	lineIDs := make([]string, len(lines))
	for i, l := range lines {
		lineIDs[i] = l.ID
	}
	var rows []model.SalesInvoiceLineTax
	if err := db.Where("sales_invoice_line_id IN ?", lineIDs).Find(&rows).Error; err != nil {
		return fmt.Errorf("find sales invoice line taxes: %w", err)
	}
	byLine := make(map[string][]string, len(lines))
	for _, r := range rows {
		byLine[r.SalesInvoiceLineID] = append(byLine[r.SalesInvoiceLineID], r.TaxID)
	}
	for i := range lines {
		lines[i].TaxIDs = byLine[lines[i].ID]
	}
	return nil
}

func parseSalesInvoiceDate(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, &domain.ErrValidation{Message: "invalid date (format YYYY-MM-DD)"}
	}
	return t, nil
}

func parseOptionalSalesInvoiceDate(s string) (*time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	t, err := parseSalesInvoiceDate(s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// PreviewNumber returns what the next auto-generated number would be for
// this kind right now — a preview for the Add page, not a reservation.
func (r *SalesInvoiceRepository) PreviewNumber(companyID, kind string) (string, error) {
	return generateSalesInvoiceNumber(r.db, companyID, model.SalesInvoiceKind(kind), time.Now())
}

// templateFor: an explicit choice wins; otherwise the company's default for this document type.
func templateFor(db *gorm.DB, dto *domain.CreateDTO) string {
	if v := strings.TrimSpace(dto.Template); v != "" {
		return v
	}
	docType := model.DocTypeSalesInvoice
	if dto.Kind == string(model.SalesInvoiceKindDownPayment) {
		docType = model.DocTypeDownPayment
	}
	return documentTemplateFor(db, dto.CompanyID, docType)
}

// SetTemplate changes ONLY the layout of an existing invoice. Allowed in any status: it is
// presentation, so it never touches lines, totals or the document lifecycle.
func (r *SalesInvoiceRepository) SetTemplate(companyID, id, actorID, template string) error {
	res := r.db.Model(&model.SalesInvoice{}).Where("id = ? AND company_id = ?", id, companyID).
		Updates(map[string]any{"template": template, "updated_at": time.Now(), "updated_by": actorID})
	if res.Error != nil {
		return fmt.Errorf("set sales invoice template %s: %w", id, res.Error)
	}
	if res.RowsAffected == 0 {
		return &domain.ErrNotFound{ID: id}
	}
	return nil
}

// Summary aggregates the dashboard figures for regular (non down-payment) invoices.
// Only confirmed invoices count toward money owed / billed; drafts are counted apart.
func (r *SalesInvoiceRepository) Summary(companyID string) (*domain.Summary, error) {
	now := time.Now()
	today := now.Format("2006-01-02")
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")

	base := func() *gorm.DB {
		return r.db.Model(&model.SalesInvoice{}).Where("company_id = ? AND kind = ?", companyID, "invoice")
	}
	figure := func(q *gorm.DB, expr string) (domain.SummaryFigure, error) {
		var row struct {
			Amount float64
			Count  int64
		}
		if err := q.Select("COALESCE(SUM(" + expr + "), 0) AS amount, COUNT(*) AS count").Scan(&row).Error; err != nil {
			return domain.SummaryFigure{}, err
		}
		return domain.SummaryFigure{Amount: round2(row.Amount), Count: row.Count}, nil
	}

	owed := func() *gorm.DB {
		return base().Where("status = ? AND payment_status <> ?", model.SalesInvoiceStatusConfirmed, model.SalesInvoicePaymentPaid)
	}
	out := &domain.Summary{}
	var err error
	if out.Outstanding, err = figure(owed(), "grand_total - paid_amount"); err != nil {
		return nil, fmt.Errorf("summary outstanding: %w", err)
	}
	if out.Overdue, err = figure(owed().Where("due_date IS NOT NULL AND due_date < ?", today), "grand_total - paid_amount"); err != nil {
		return nil, fmt.Errorf("summary overdue: %w", err)
	}
	if out.ThisMonth, err = figure(base().Where("status = ? AND date >= ?", model.SalesInvoiceStatusConfirmed, monthStart), "grand_total"); err != nil {
		return nil, fmt.Errorf("summary this month: %w", err)
	}
	if err := base().Where("status = ?", model.SalesInvoiceStatusDraft).Count(&out.Drafts).Error; err != nil {
		return nil, fmt.Errorf("summary drafts: %w", err)
	}
	return out, nil
}

// CountByKind — how many sales invoices of this Kind ("invoice" or "down_payment") the company has
// created, ever. Used by the activation milestone ("1 Invoice" — regular invoices only).
func (r *SalesInvoiceRepository) CountByKind(companyID, kind string) (int64, error) {
	var n int64
	err := r.db.Model(&model.SalesInvoice{}).Where("company_id = ? AND kind = ?", companyID, kind).Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("count sales invoices: %w", err)
	}
	return n, nil
}

// CountCreatedSince — every sales invoice (both kinds) created on or after `since`. Used for the
// Free-tier transactions/month limit.
func (r *SalesInvoiceRepository) CountCreatedSince(companyID string, since time.Time) (int64, error) {
	var n int64
	err := r.db.Model(&model.SalesInvoice{}).Where("company_id = ? AND created_at >= ?", companyID, since).Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("count sales invoices since: %w", err)
	}
	return n, nil
}
