package repository

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"duluin_invoice/app/model"
)

// Document types a connected-documents lookup understands. A down payment is a sales_invoices row
// with kind = down_payment, so it is reported as its own type.
const (
	ConnSalesOrder      = "sales_order"
	ConnDownPayment     = "down_payment"
	ConnSalesInvoice    = "sales_invoice"
	ConnSalesReceipt    = "sales_receipt"
	ConnDeliveryNote    = "delivery_note"
	ConnPurchaseOrder   = "purchase_order"
	ConnPurchaseInvoice = "purchase_invoice"
	ConnPurchaseReceipt = "purchase_receipt"
	ConnGoodsReceipt    = "goods_receipt"
)

// ConnectedDocument is a real, stored relationship of one document — never derived from a guess.
type ConnectedDocument struct {
	Type          string    `json:"type"`
	ID            string    `json:"id"`
	Number        string    `json:"number"`
	Date          time.Time `json:"date"`
	Status        string    `json:"status,omitempty"`
	PaymentStatus string    `json:"payment_status,omitempty"`
	Amount        *float64  `json:"amount,omitempty"`
}

// ErrConnectedNotFound — the document does not exist in this company.
var ErrConnectedNotFound = errors.New("document not found")

type ConnectedDocumentRepository struct{ db *gorm.DB }

func NewConnectedDocumentRepository(db *gorm.DB) *ConnectedDocumentRepository {
	return &ConnectedDocumentRepository{db: db}
}

type connRow struct {
	ID            string
	Number        string
	Date          time.Time
	Status        string
	PaymentStatus string
	Amount        float64
	Kind          string
	// links
	SalesOrderID      *string
	LinkedInvoiceID   *string
	SalesInvoiceID    *string
	PurchaseOrderID   *string
	PurchaseInvoiceID *string
}

// collector keeps first-seen order and drops duplicates and the document itself.
type collector struct {
	selfType, selfID string
	seen             map[string]bool
	out              []ConnectedDocument
}

func (c *collector) add(typ string, r connRow, withAmount, withStatus, withPay bool) {
	if typ == c.selfType && r.ID == c.selfID {
		return
	}
	k := typ + ":" + r.ID
	if c.seen[k] {
		return
	}
	c.seen[k] = true
	d := ConnectedDocument{Type: typ, ID: r.ID, Number: r.Number, Date: r.Date}
	if withStatus {
		d.Status = r.Status
	}
	if withPay {
		d.PaymentStatus = r.PaymentStatus
	}
	if withAmount {
		a := r.Amount
		d.Amount = &a
	}
	c.out = append(c.out, d)
}

// Every query below is filtered by company_id: a document of another company can never be
// reached, even by a forged id.
func (r *ConnectedDocumentRepository) rows(model any, cols string, companyID, where string, args ...any) ([]connRow, error) {
	var rows []connRow
	q := r.db.Model(model).Select(cols).Where("company_id = ?", companyID)
	if where != "" {
		q = q.Where(where, args...)
	}
	if err := q.Order("date, created_at").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("connected documents: %w", err)
	}
	return rows, nil
}

const (
	colsInvoice  = "id, number, date, status, payment_status, grand_total as amount, kind, sales_order_id, linked_invoice_id"
	colsPInvoice = "id, number, date, status, payment_status, grand_total as amount, purchase_order_id"
)

func (r *ConnectedDocumentRepository) salesInvoicesWhere(company, where string, args ...any) ([]connRow, error) {
	return r.rows(&model.SalesInvoice{}, colsInvoice, company, where, args...)
}

func invoiceType(row connRow) string {
	if row.Kind == string(model.SalesInvoiceKindDownPayment) {
		return ConnDownPayment
	}
	return ConnSalesInvoice
}

func (r *ConnectedDocumentRepository) addSalesInvoices(c *collector, rows []connRow) {
	for _, row := range rows {
		c.add(invoiceType(row), row, true, true, true)
	}
}

func (r *ConnectedDocumentRepository) salesOrder(company, id string) (*connRow, error) {
	rows, err := r.rows(&model.SalesOrder{}, "id, number, date, status, grand_total as amount", company, "id = ?", id)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

func (r *ConnectedDocumentRepository) purchaseOrder(company, id string) (*connRow, error) {
	rows, err := r.rows(&model.PurchaseOrder{}, "id, number, date, status, grand_total as amount", company, "id = ?", id)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

// receiptsForInvoices — sales receipts that allocate money to any of the given invoices.
func (r *ConnectedDocumentRepository) receiptsForInvoices(company string, invoiceIDs []string) ([]connRow, error) {
	if len(invoiceIDs) == 0 {
		return nil, nil
	}
	return r.rows(&model.SalesReceipt{}, "id, number, date, amount", company,
		"id IN (SELECT sales_receipt_id FROM sales_receipt_allocations WHERE company_id = ? AND sales_invoice_id IN ?)", company, invoiceIDs)
}

func ids(rows []connRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

// List returns the documents genuinely linked to (docType, id) in this company.
func (r *ConnectedDocumentRepository) List(companyID, docType, id string) ([]ConnectedDocument, error) {
	c := &collector{selfType: docType, selfID: id, seen: map[string]bool{}}

	switch docType {
	case ConnSalesOrder:
		so, err := r.salesOrder(companyID, id)
		if err != nil {
			return nil, err
		}
		if so == nil {
			return nil, ErrConnectedNotFound
		}
		invs, err := r.salesInvoicesWhere(companyID, "sales_order_id = ?", id)
		if err != nil {
			return nil, err
		}
		r.addSalesInvoices(c, invs)
		// Delivery notes made straight from the order, or from one of its invoices.
		dns, err := r.rows(&model.DeliveryNote{}, "id, number, date", companyID, "sales_order_id = ? OR sales_invoice_id IN ?", id, append(ids(invs), "00000000-0000-0000-0000-000000000000"))
		if err != nil {
			return nil, err
		}
		for _, d := range dns {
			c.add(ConnDeliveryNote, d, false, false, false)
		}
		rcs, err := r.receiptsForInvoices(companyID, ids(invs))
		if err != nil {
			return nil, err
		}
		for _, rc := range rcs {
			c.add(ConnSalesReceipt, rc, true, false, false)
		}

	case ConnSalesInvoice, ConnDownPayment:
		rows, err := r.salesInvoicesWhere(companyID, "id = ? AND kind = ?", id, map[string]string{ConnSalesInvoice: string(model.SalesInvoiceKindInvoice), ConnDownPayment: string(model.SalesInvoiceKindDownPayment)}[docType])
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, ErrConnectedNotFound
		}
		inv := rows[0]
		if inv.SalesOrderID != nil {
			if so, err := r.salesOrder(companyID, *inv.SalesOrderID); err != nil {
				return nil, err
			} else if so != nil {
				c.add(ConnSalesOrder, *so, true, true, false)
			}
		}
		// A down payment and its invoice point at each other through one field, in either direction.
		linked, err := r.salesInvoicesWhere(companyID, "id = ? OR linked_invoice_id = ?", derefOr(inv.LinkedInvoiceID, id), id)
		if err != nil {
			return nil, err
		}
		r.addSalesInvoices(c, linked)
		dns, err := r.rows(&model.DeliveryNote{}, "id, number, date", companyID, "sales_invoice_id = ?", id)
		if err != nil {
			return nil, err
		}
		for _, d := range dns {
			c.add(ConnDeliveryNote, d, false, false, false)
		}
		rcs, err := r.receiptsForInvoices(companyID, []string{id})
		if err != nil {
			return nil, err
		}
		for _, rc := range rcs {
			c.add(ConnSalesReceipt, rc, true, false, false)
		}

	case ConnSalesReceipt:
		rcs, err := r.rows(&model.SalesReceipt{}, "id, number, date, amount", companyID, "id = ?", id)
		if err != nil {
			return nil, err
		}
		if len(rcs) == 0 {
			return nil, ErrConnectedNotFound
		}
		invs, err := r.salesInvoicesWhere(companyID, "id IN (SELECT sales_invoice_id FROM sales_receipt_allocations WHERE company_id = ? AND sales_receipt_id = ?)", companyID, id)
		if err != nil {
			return nil, err
		}
		r.addSalesInvoices(c, invs)
		// The down payment / invoice each allocated invoice is linked to, and their sales orders.
		if len(invs) > 0 {
			more, err := r.salesInvoicesWhere(companyID, "id IN ? OR linked_invoice_id IN ?", linkedIDs(invs), ids(invs))
			if err != nil {
				return nil, err
			}
			r.addSalesInvoices(c, more)
			for _, row := range append(invs, more...) {
				if row.SalesOrderID != nil {
					if so, err := r.salesOrder(companyID, *row.SalesOrderID); err != nil {
						return nil, err
					} else if so != nil {
						c.add(ConnSalesOrder, *so, true, true, false)
					}
				}
			}
		}

	case ConnDeliveryNote:
		dns, err := r.rows(&model.DeliveryNote{}, "id, number, date, sales_order_id, sales_invoice_id", companyID, "id = ?", id)
		if err != nil {
			return nil, err
		}
		if len(dns) == 0 {
			return nil, ErrConnectedNotFound
		}
		if dns[0].SalesOrderID != nil {
			if so, err := r.salesOrder(companyID, *dns[0].SalesOrderID); err != nil {
				return nil, err
			} else if so != nil {
				c.add(ConnSalesOrder, *so, true, true, false)
			}
		}
		if dns[0].SalesInvoiceID != nil {
			invs, err := r.salesInvoicesWhere(companyID, "id = ?", *dns[0].SalesInvoiceID)
			if err != nil {
				return nil, err
			}
			r.addSalesInvoices(c, invs)
		}

	case ConnPurchaseOrder:
		po, err := r.purchaseOrder(companyID, id)
		if err != nil {
			return nil, err
		}
		if po == nil {
			return nil, ErrConnectedNotFound
		}
		if err := r.purchaseChildren(c, companyID, id); err != nil {
			return nil, err
		}

	case ConnPurchaseInvoice:
		rows, err := r.rows(&model.PurchaseInvoice{}, colsPInvoice, companyID, "id = ?", id)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, ErrConnectedNotFound
		}
		if rows[0].PurchaseOrderID != nil {
			if po, err := r.purchaseOrder(companyID, *rows[0].PurchaseOrderID); err != nil {
				return nil, err
			} else if po != nil {
				c.add(ConnPurchaseOrder, *po, true, true, false)
			}
			// Goods received against the same order.
			if err := r.purchaseGoods(c, companyID, *rows[0].PurchaseOrderID); err != nil {
				return nil, err
			}
		}
		if err := r.purchasePayments(c, companyID, []string{id}); err != nil {
			return nil, err
		}

	case ConnPurchaseReceipt:
		rcs, err := r.rows(&model.PurchaseReceipt{}, "id, number, date, amount, purchase_invoice_id", companyID, "id = ?", id)
		if err != nil {
			return nil, err
		}
		if len(rcs) == 0 {
			return nil, ErrConnectedNotFound
		}
		if rcs[0].PurchaseInvoiceID != nil {
			pis, err := r.rows(&model.PurchaseInvoice{}, colsPInvoice, companyID, "id = ?", *rcs[0].PurchaseInvoiceID)
			if err != nil {
				return nil, err
			}
			for _, pi := range pis {
				c.add(ConnPurchaseInvoice, pi, true, true, true)
				if pi.PurchaseOrderID != nil {
					if po, err := r.purchaseOrder(companyID, *pi.PurchaseOrderID); err != nil {
						return nil, err
					} else if po != nil {
						c.add(ConnPurchaseOrder, *po, true, true, false)
					}
				}
			}
		}

	case ConnGoodsReceipt:
		grs, err := r.rows(&model.GoodsReceipt{}, "id, number, date, purchase_order_id", companyID, "id = ?", id)
		if err != nil {
			return nil, err
		}
		if len(grs) == 0 {
			return nil, ErrConnectedNotFound
		}
		if grs[0].PurchaseOrderID != nil {
			if po, err := r.purchaseOrder(companyID, *grs[0].PurchaseOrderID); err != nil {
				return nil, err
			} else if po != nil {
				c.add(ConnPurchaseOrder, *po, true, true, false)
			}
			pis, err := r.rows(&model.PurchaseInvoice{}, colsPInvoice, companyID, "purchase_order_id = ?", *grs[0].PurchaseOrderID)
			if err != nil {
				return nil, err
			}
			for _, pi := range pis {
				c.add(ConnPurchaseInvoice, pi, true, true, true)
			}
		}

	default:
		return nil, fmt.Errorf("unknown document type %q", docType)
	}
	return c.out, nil
}

func (r *ConnectedDocumentRepository) purchaseChildren(c *collector, companyID, poID string) error {
	pis, err := r.rows(&model.PurchaseInvoice{}, colsPInvoice, companyID, "purchase_order_id = ?", poID)
	if err != nil {
		return err
	}
	for _, pi := range pis {
		c.add(ConnPurchaseInvoice, pi, true, true, true)
	}
	if err := r.purchaseGoods(c, companyID, poID); err != nil {
		return err
	}
	return r.purchasePayments(c, companyID, ids(pis))
}

func (r *ConnectedDocumentRepository) purchaseGoods(c *collector, companyID, poID string) error {
	grs, err := r.rows(&model.GoodsReceipt{}, "id, number, date", companyID, "purchase_order_id = ?", poID)
	if err != nil {
		return err
	}
	for _, g := range grs {
		c.add(ConnGoodsReceipt, g, false, false, false)
	}
	return nil
}

func (r *ConnectedDocumentRepository) purchasePayments(c *collector, companyID string, invoiceIDs []string) error {
	if len(invoiceIDs) == 0 {
		return nil
	}
	rcs, err := r.rows(&model.PurchaseReceipt{}, "id, number, date, amount", companyID, "purchase_invoice_id IN ?", invoiceIDs)
	if err != nil {
		return err
	}
	for _, rc := range rcs {
		c.add(ConnPurchaseReceipt, rc, true, false, false)
	}
	return nil
}

func derefOr(p *string, fallback string) string {
	if p != nil {
		return *p
	}
	return fallback
}

func linkedIDs(rows []connRow) []string {
	out := []string{}
	for _, r := range rows {
		if r.LinkedInvoiceID != nil {
			out = append(out, *r.LinkedInvoiceID)
		}
	}
	if len(out) == 0 {
		return []string{"00000000-0000-0000-0000-000000000000"}
	}
	return out
}
