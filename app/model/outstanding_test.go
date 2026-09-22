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
