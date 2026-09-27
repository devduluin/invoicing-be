package repository

import (
	"os"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	purchaseinvoice "duluin_invoice/app/domain/purchaseinvoice"
	salesinvoice "duluin_invoice/app/domain/salesinvoice"
	salespayment "duluin_invoice/app/domain/salespayment"
	salesreceipt "duluin_invoice/app/domain/salesreceipt"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// End-to-end money flow against a real Postgres (the DP / payment / receipt rules are SQL-level:
// row locks, SUMs over linked documents). Skipped unless INVOICE_TEST_DSN points at a scratch
// database, e.g. host=localhost user=postgres password=... dbname=invoice_flow_test sslmode=disable.

type flow struct {
	t       *testing.T
	db      *gorm.DB
	company string
	mitra   string
	inv     *SalesInvoiceRepository
}

func newFlow(t *testing.T) *flow {
	dsn := os.Getenv("INVOICE_TEST_DSN")
	if dsn == "" {
		t.Skip("INVOICE_TEST_DSN not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrateAll(db); err != nil {
		t.Fatal(err)
	}
	return &flow{t: t, db: db, company: uuid.NewString(), mitra: uuid.NewString(), inv: &SalesInvoiceRepository{db: db}}
}

func (f *flow) salesOrder(total float64) string {
	so := &model.SalesOrder{ID: uuid.NewString(), CompanyID: f.company, MitraID: f.mitra, Number: "SO/" + uuid.NewString()[:6], GrandTotal: total, Status: model.SalesOrderStatusConfirmed}
	if err := f.db.Create(so).Error; err != nil {
		f.t.Fatal(err)
	}
	return so.ID
}

func (f *flow) doc(kind string, total float64, so, linked *string) (*model.SalesInvoice, error) {
	calc := &utils.LinesCalc{
		Lines:    []utils.LineResult{{ProductName: "x", Quantity: 1, UnitPrice: total, LineSubtotal: total, LineTotal: total}},
		Subtotal: total, GrandTotal: total,
	}
	return f.inv.Create(&salesinvoice.CreateDTO{
		CompanyID: f.company, Kind: kind, MitraID: f.mitra, Date: "2026-09-24", DueDate: "2026-10-24",
		SalesOrderID: so, LinkedInvoiceID: linked,
	}, calc, "u")
}

func (f *flow) mustDoc(kind string, total float64, so, linked *string) *model.SalesInvoice {
	d, err := f.doc(kind, total, so, linked)
	if err != nil {
		f.t.Fatalf("create %s %.0f: %v", kind, total, err)
	}
	return d
}

func (f *flow) setStatus(id string, s model.SalesInvoiceStatus) error {
	return f.inv.SetStatus(f.company, id, "u", s)
}

func (f *flow) mustConfirm(id string) {
	if err := f.setStatus(id, model.SalesInvoiceStatusConfirmed); err != nil {
		f.t.Fatalf("confirm: %v", err)
	}
}

func (f *flow) get(id string) *model.SalesInvoice {
	d, err := f.inv.FindByID(f.company, id)
	if err != nil {
		f.t.Fatal(err)
	}
	return d
}

func (f *flow) expect(id string, applied, paid, outstanding float64) {
	f.t.Helper()
	d := f.get(id)
	if d.AppliedDPAmount != applied || d.PaidAmount != paid || d.OutstandingAmount != outstanding {
		f.t.Fatalf("invoice %s: applied=%v paid=%v outstanding=%v, want %v/%v/%v",
			d.Number, d.AppliedDPAmount, d.PaidAmount, d.OutstandingAmount, applied, paid, outstanding)
	}
}

func (f *flow) receipt(invoiceID string, amount float64) (*model.SalesReceipt, error) {
	return (&SalesReceiptRepository{db: f.db}).Create(&salesreceipt.CreateDTO{
		CompanyID: f.company, MitraID: f.mitra, Date: "2026-09-24", PaymentMethod: "cash",
		Allocations: []salesreceipt.AllocationDTO{{SalesInvoiceID: invoiceID, Amount: amount}},
	}, "u")
}

const m = 1_000_000.0

func ptr(s string) *string { return &s }

// CASE 1 + 4: a plain invoice owes its total; a confirmed DP linked to it lowers only the outstanding.
func TestFlow_DownPaymentReducesOutstandingNotTotal(t *testing.T) {
	f := newFlow(t)
	inv := f.mustDoc("invoice", 10*m, nil, nil)
	f.mustConfirm(inv.ID)
	f.expect(inv.ID, 0, 0, 10*m)

	dp := f.mustDoc("down_payment", 3*m, nil, &inv.ID)
	f.expect(inv.ID, 0, 0, 10*m) // draft DP doesn't count
	f.mustConfirm(dp.ID)
	f.expect(inv.ID, 3*m, 0, 7*m)
	if g := f.get(inv.ID).GrandTotal; g != 10*m {
		t.Fatalf("invoice total changed to %v", g)
	}

	// CASE 5: payment 2jt on top → applied 3jt, paid 2jt, outstanding 5jt
	if _, err := f.receipt(inv.ID, 2*m); err != nil {
		t.Fatal(err)
	}
	f.expect(inv.ID, 3*m, 2*m, 5*m)
}

// CASE 9: cancelling / reverting / deleting the DP gives the money back; never 10-3-2-3.
func TestFlow_DownPaymentCancelDeleteRevert(t *testing.T) {
	f := newFlow(t)
	inv := f.mustDoc("invoice", 10*m, nil, nil)
	f.mustConfirm(inv.ID)
	dp := f.mustDoc("down_payment", 3*m, nil, &inv.ID)
	f.mustConfirm(dp.ID)
	f.expect(inv.ID, 3*m, 0, 7*m)

	if err := f.setStatus(dp.ID, model.SalesInvoiceStatusCancelled); err != nil {
		t.Fatal(err)
	}
	f.expect(inv.ID, 0, 0, 10*m)
	f.mustConfirm(dp.ID)
	f.expect(inv.ID, 3*m, 0, 7*m)
	if err := f.setStatus(dp.ID, model.SalesInvoiceStatusDraft); err != nil {
		t.Fatal(err)
	}
	f.expect(inv.ID, 0, 0, 10*m)
	f.mustConfirm(dp.ID)
	if err := f.inv.Delete(f.company, dp.ID); err != nil {
		t.Fatal(err)
	}
	f.expect(inv.ID, 0, 0, 10*m)
}

// CASE 6/7/8: payments against outstanding, exact settle, and over-payment rejected.
func TestFlow_PaymentLimits(t *testing.T) {
	f := newFlow(t)
	inv := f.mustDoc("invoice", 10*m, nil, nil)
	f.mustConfirm(inv.ID)
	if _, err := f.receipt(inv.ID, 4*m); err != nil {
		t.Fatal(err)
	}
	f.expect(inv.ID, 0, 4*m, 6*m)

	dp := f.mustDoc("down_payment", 3*m, nil, &inv.ID)
	f.mustConfirm(dp.ID)
	f.expect(inv.ID, 3*m, 4*m, 3*m)

	if _, err := f.receipt(inv.ID, 3*m+1); err == nil {
		t.Fatal("a payment above the outstanding must be rejected")
	}
	rc, err := f.receipt(inv.ID, 3*m)
	if err != nil {
		t.Fatal(err)
	}
	f.expect(inv.ID, 3*m, 7*m, 0)
	if got := f.get(inv.ID).PaymentStatus; got != model.SalesInvoicePaymentPaid {
		t.Fatalf("status %s, want paid", got)
	}

	// CASE 10: removing the payment brings the balance back (receipt = the payment record)
	if err := (&SalesReceiptRepository{db: f.db}).Delete(f.company, rc.ID); err != nil {
		t.Fatal(err)
	}
	f.expect(inv.ID, 3*m, 4*m, 3*m)
}

// CASE 8: DP 3jt then payment 8jt on a 10jt invoice is refused (only 7jt left); 7jt is fine.
func TestFlow_PaymentAboveOutstandingAfterDP(t *testing.T) {
	f := newFlow(t)
	inv := f.mustDoc("invoice", 10*m, nil, nil)
	f.mustConfirm(inv.ID)
	dp := f.mustDoc("down_payment", 3*m, nil, &inv.ID)
	f.mustConfirm(dp.ID)
	if _, err := f.receipt(inv.ID, 8*m); err == nil {
		t.Fatal("8jt against a 7jt outstanding must be rejected")
	}
	f.expect(inv.ID, 3*m, 0, 7*m)
	if _, err := f.receipt(inv.ID, 7*m); err != nil {
		t.Fatal(err)
	}
	f.expect(inv.ID, 3*m, 7*m, 0)
}

// The same rule holds for the direct payment API, which now applies in one step (no verify).
func TestFlow_RecordedPaymentAppliesOnce(t *testing.T) {
	f := newFlow(t)
	inv := f.mustDoc("invoice", 10*m, nil, nil)
	f.mustConfirm(inv.ID)
	dp := f.mustDoc("down_payment", 3*m, nil, &inv.ID)
	f.mustConfirm(dp.ID)
	pay := &SalesPaymentRepository{db: f.db}
	p, err := pay.Create(&salespayment.CreateDTO{CompanyID: f.company, MitraID: f.mitra, SalesInvoiceID: inv.ID, Date: "2026-09-24", Amount: 2 * m, PaymentMethod: "cash"}, "u")
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != model.SalesPaymentStatusVerified {
		t.Fatalf("payment status %s, want verified", p.Status)
	}
	f.expect(inv.ID, 3*m, 2*m, 5*m)
	if _, err := pay.Verify(f.company, "u", p.ID); err == nil {
		t.Fatal("an already-applied payment must not verify a second time")
	}
	f.expect(inv.ID, 3*m, 2*m, 5*m)
}

// CASE 2/3/11: SO → DP → Invoice. The order never shrinks; the DP made from the order is carried
// into the invoice made from the same order.
func TestFlow_SalesOrderDownPaymentInvoiceChain(t *testing.T) {
	f := newFlow(t)
	so := f.salesOrder(10 * m)
	dp := f.mustDoc("down_payment", 3*m, &so, nil)
	f.mustConfirm(dp.ID)

	inv := f.mustDoc("invoice", 10*m, &so, nil)
	f.mustConfirm(inv.ID)
	f.expect(inv.ID, 3*m, 0, 7*m)
	if got := f.get(inv.ID); got.GrandTotal != 10*m {
		t.Fatalf("invoice total %v", got.GrandTotal)
	}
	if l := f.get(dp.ID).LinkedInvoiceID; l == nil || *l != inv.ID {
		t.Fatal("the DP must now point at the invoice")
	}
	var soTotal float64
	f.db.Table("sales_orders").Select("grand_total").Where("id = ?", so).Scan(&soTotal)
	if soTotal != 10*m {
		t.Fatalf("sales order total changed to %v", soTotal)
	}

	rc, err := f.receipt(inv.ID, 2*m)
	if err != nil {
		t.Fatal(err)
	}
	if rc.Amount != 2*m {
		t.Fatalf("receipt amount %v", rc.Amount)
	}
	f.expect(inv.ID, 3*m, 2*m, 5*m)
}

// Validation: DP over the outstanding, invoice over the order, and DPs over the order are refused.
func TestFlow_Validation(t *testing.T) {
	f := newFlow(t)
	inv := f.mustDoc("invoice", 10*m, nil, nil)
	f.mustConfirm(inv.ID)
	if _, err := f.doc("down_payment", 10*m+1, nil, &inv.ID); err == nil {
		t.Fatal("DP above the invoice outstanding must be rejected")
	}

	// Two drafts each fit on their own (a draft reduces nothing)…
	a := f.mustDoc("down_payment", 6*m, nil, &inv.ID)
	b := f.mustDoc("down_payment", 6*m, nil, &inv.ID)
	f.mustConfirm(a.ID)
	f.expect(inv.ID, 6*m, 0, 4*m)
	// …but once one is confirmed, a second 6jt no longer fits, and can't be created or confirmed.
	if err := f.setStatus(b.ID, model.SalesInvoiceStatusConfirmed); err == nil {
		t.Fatal("confirming the second DP must fail: 6jt + 6jt > 10jt")
	}
	if _, err := f.doc("down_payment", 6*m, nil, &inv.ID); err == nil {
		t.Fatal("a new DP above the remaining outstanding must be rejected")
	}
	f.expect(inv.ID, 6*m, 0, 4*m)

	so := f.salesOrder(10 * m)
	if _, err := f.doc("invoice", 10*m+1, &so, nil); err == nil {
		t.Fatal("invoice above the order must be rejected")
	}
	first := f.mustDoc("invoice", 6*m, &so, nil)
	if _, err := f.doc("invoice", 5*m, &so, nil); err == nil {
		t.Fatal("second invoice must fit into what is left of the order")
	}
	_ = first
	if _, err := f.doc("down_payment", 10*m+1, ptr(so), nil); err == nil {
		t.Fatal("DP above the order must be rejected")
	}
}

// The summary uses the same formula as the detail.
func TestFlow_SummaryUsesSameFormula(t *testing.T) {
	f := newFlow(t)
	inv := f.mustDoc("invoice", 10*m, nil, nil)
	f.mustConfirm(inv.ID)
	dp := f.mustDoc("down_payment", 3*m, nil, &inv.ID)
	f.mustConfirm(dp.ID)
	if _, err := f.receipt(inv.ID, 2*m); err != nil {
		t.Fatal(err)
	}
	s, err := f.inv.Summary(f.company)
	if err != nil {
		t.Fatal(err)
	}
	if s.Outstanding.Amount != 5*m {
		t.Fatalf("summary outstanding %v, want %v", s.Outstanding.Amount, 5*m)
	}
}

// Scenario A: SO 10jt → Invoice 10jt. Nothing else exists, so the invoice owes all of it.
func TestFlow_A_SalesOrderToInvoice(t *testing.T) {
	f := newFlow(t)
	so := f.salesOrder(10 * m)
	inv := f.mustDoc("invoice", 10*m, &so, nil)
	f.mustConfirm(inv.ID)
	f.expect(inv.ID, 0, 0, 10*m)
}

// Scenario D: SO → Invoice → DP gives the SAME result as SO → DP → Invoice (scenario C).
func TestFlow_D_SalesOrderInvoiceThenDownPayment(t *testing.T) {
	f := newFlow(t)
	so := f.salesOrder(10 * m)
	inv := f.mustDoc("invoice", 10*m, &so, nil)
	f.mustConfirm(inv.ID)

	dp := f.mustDoc("down_payment", 3*m, &so, nil) // made from the order only
	if l := f.get(dp.ID).LinkedInvoiceID; l == nil || *l != inv.ID {
		t.Fatal("a DP from an order that already has an invoice must apply to that invoice")
	}
	f.expect(inv.ID, 0, 0, 10*m) // draft doesn't count
	f.mustConfirm(dp.ID)
	f.expect(inv.ID, 3*m, 0, 7*m)
	if g := f.get(inv.ID).GrandTotal; g != 10*m {
		t.Fatalf("invoice total changed to %v", g)
	}

	rc, err := f.receipt(inv.ID, 2*m)
	if err != nil {
		t.Fatal(err)
	}
	if rc.Amount != 2*m {
		t.Fatalf("receipt %v", rc.Amount)
	}
	f.expect(inv.ID, 3*m, 2*m, 5*m)
}

// "Create Invoice" from a confirmed DP's page: the invoice references the DP; that DP must apply.
func TestFlow_InvoiceMadeFromDownPayment(t *testing.T) {
	f := newFlow(t)
	dp := f.mustDoc("down_payment", 3*m, nil, nil)
	f.mustConfirm(dp.ID)
	inv := f.mustDoc("invoice", 10*m, nil, &dp.ID)
	f.mustConfirm(inv.ID)
	f.expect(inv.ID, 3*m, 0, 7*m)
	if l := f.get(dp.ID).LinkedInvoiceID; l == nil || *l != inv.ID {
		t.Fatal("the DP must now point at the invoice")
	}
	// a second invoice referencing the same, already-applied DP must not apply it again
	inv2 := f.mustDoc("invoice", 10*m, nil, &dp.ID)
	f.mustConfirm(inv2.ID)
	f.expect(inv2.ID, 0, 0, 10*m)
	f.expect(inv.ID, 3*m, 0, 7*m)
}

// Gap coverage: deleting a Sales Order that still has an invoice/DP/delivery note must be refused —
// otherwise those documents' "can't exceed the order total" cap silently turns off (the cap query
// finds no order row and used to skip the check instead of failing).
func TestFlow_SalesOrderDeleteGuardedWhileReferenced(t *testing.T) {
	f := newFlow(t)
	so := f.salesOrder(10 * m)
	inv := f.mustDoc("invoice", 6*m, &so, nil)
	f.mustConfirm(inv.ID)

	if err := (&SalesOrderRepository{db: f.db}).Delete(f.company, so); err == nil {
		t.Fatal("deleting a sales order with a live invoice must be refused")
	}

	// once refused, the order still exists and its cap still applies
	if _, err := f.doc("invoice", 5*m, &so, nil); err == nil {
		t.Fatal("order total 10jt already has 6jt invoiced; a second 5jt invoice must be rejected")
	}
}

func TestFlow_PurchaseInvoiceCappedByPurchaseOrder(t *testing.T) {
	f := newFlow(t)
	po := &model.PurchaseOrder{ID: uuid.NewString(), CompanyID: f.company, MitraID: f.mitra, Number: "PO/" + uuid.NewString()[:6], GrandTotal: 10 * m, Status: model.PurchaseOrderStatusConfirmed}
	if err := f.db.Create(po).Error; err != nil {
		t.Fatal(err)
	}
	repo := &PurchaseInvoiceRepository{db: f.db}
	mk := func(total float64) (*model.PurchaseInvoice, error) {
		calc := &utils.LinesCalc{Lines: []utils.LineResult{{ProductName: "x", Quantity: 1, UnitPrice: total, LineSubtotal: total, LineTotal: total}}, Subtotal: total, GrandTotal: total}
		return repo.Create(&purchaseinvoice.CreateDTO{CompanyID: f.company, PurchaseOrderID: &po.ID, MitraID: f.mitra, Date: "2026-09-24"}, calc, "u")
	}
	if _, err := mk(10*m + 1); err == nil {
		t.Fatal("a bill above the PO total must be rejected")
	}
	first, err := mk(6 * m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mk(5 * m); err == nil {
		t.Fatal("6jt + 5jt exceeds the 10jt PO; the second bill must be rejected")
	}
	_ = first

	if err := (&PurchaseOrderRepository{db: f.db}).Delete(f.company, po.ID); err == nil {
		t.Fatal("deleting a purchase order with a live bill must be refused")
	}
}
