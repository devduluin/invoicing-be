package model

import "testing"

func TestOutstandingOf(t *testing.T) {
	cases := []struct{ total, paid, want float64 }{
		{10000000, 0, 10000000},
		{10000000, 4000000, 6000000},
		{10000000, 10000000, 0},
		{10000000, 12000000, 0}, // overpaid never goes negative
		{0, 0, 0},
		{100.005, 0.001, 100},
	}
	for _, c := range cases {
		if got := outstandingOf(c.total, c.paid); got != c.want {
			t.Errorf("outstandingOf(%v, %v) = %v, want %v", c.total, c.paid, got, c.want)
		}
	}
}

func TestSalesBalance(t *testing.T) {
	const m = 1_000_000.0
	cases := []struct {
		name                 string
		total, dp, paid, out float64
		status               SalesInvoicePaymentStatus
	}{
		{"nothing yet", 10 * m, 0, 0, 10 * m, SalesInvoicePaymentUnpaid},
		{"DP only", 10 * m, 3 * m, 0, 7 * m, SalesInvoicePaymentPartiallyPaid},
		{"DP + payment", 10 * m, 3 * m, 2 * m, 5 * m, SalesInvoicePaymentPartiallyPaid},
		{"payment only", 10 * m, 0, 4 * m, 6 * m, SalesInvoicePaymentPartiallyPaid},
		{"DP + rest paid", 10 * m, 3 * m, 7 * m, 0, SalesInvoicePaymentPaid},
		{"never negative", 10 * m, 6 * m, 6 * m, 0, SalesInvoicePaymentPaid},
		{"zero invoice", 0, 0, 0, 0, SalesInvoicePaymentUnpaid},
	}
	for _, c := range cases {
		out, st := SalesBalance(c.total, c.dp, c.paid)
		if out != c.out || st != c.status {
			t.Errorf("%s: got %v/%s, want %v/%s", c.name, out, st, c.out, c.status)
		}
	}
}

func TestInvoiceAfterFindSetsOutstanding(t *testing.T) {
	s := &SalesInvoice{GrandTotal: 10000000, PaidAmount: 4000000}
	_ = s.AfterFind(nil)
	if s.OutstandingAmount != 6000000 {
		t.Errorf("sales outstanding = %v", s.OutstandingAmount)
	}
	p := &PurchaseInvoice{GrandTotal: 5000000, PaidAmount: 5000000}
	_ = p.AfterFind(nil)
	if p.OutstandingAmount != 0 {
		t.Errorf("purchase outstanding = %v", p.OutstandingAmount)
	}
}
