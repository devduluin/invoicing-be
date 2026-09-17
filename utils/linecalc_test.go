package utils

import (
	"math"
	"testing"
)

func lcLine(product string, qty, price, discountPct float64, taxIDs ...string) LineInput {
	return LineInput{
		ProductName: product, Quantity: qty, UnitPrice: price,
		DiscountType: "percent", DiscountValue: discountPct, TaxIDs: taxIDs,
	}
}

func lcLineAmount(product string, qty, price, discountAmt float64, taxIDs ...string) LineInput {
	return LineInput{
		ProductName: product, Quantity: qty, UnitPrice: price,
		DiscountType: "amount", DiscountValue: discountAmt, TaxIDs: taxIDs,
	}
}

func TestCalcLines(t *testing.T) {
	t.Run("no tax, no discount", func(t *testing.T) {
		calc, err := CalcLines([]LineInput{lcLine("Jasa A", 2, 100000, 0)}, nil, AdditionalDiscount{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calc.Subtotal != 200000 || calc.TaxTotal != 0 || calc.GrandTotal != 200000 {
			t.Errorf("unexpected totals: %+v", calc)
		}
	})

	t.Run("with percent discount", func(t *testing.T) {
		calc, err := CalcLines([]LineInput{lcLine("Jasa A", 1, 100000, 10)}, nil, AdditionalDiscount{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calc.DiscountTotal != 10000 || calc.Subtotal != 90000 || calc.GrandTotal != 90000 {
			t.Errorf("unexpected totals: %+v", calc)
		}
	})

	t.Run("with amount discount", func(t *testing.T) {
		calc, err := CalcLines([]LineInput{lcLineAmount("Jasa A", 1, 100000, 15000)}, nil, AdditionalDiscount{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calc.DiscountTotal != 15000 || calc.Subtotal != 85000 || calc.GrandTotal != 85000 {
			t.Errorf("unexpected totals: %+v", calc)
		}
	})

	t.Run("rejects amount discount larger than the line", func(t *testing.T) {
		if _, err := CalcLines([]LineInput{lcLineAmount("Jasa A", 1, 100000, 150000)}, nil, AdditionalDiscount{}); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("exclusive tax adds on top", func(t *testing.T) {
		taxID := "tax-ppn"
		rates := map[string]TaxRate{taxID: {Rate: 11, CalcMethod: "exclusive"}}
		calc, err := CalcLines([]LineInput{lcLine("Jasa A", 1, 100000, 0, taxID)}, rates, AdditionalDiscount{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calc.TaxTotal != 11000 || calc.GrandTotal != 111000 {
			t.Errorf("unexpected totals: %+v", calc)
		}
	})

	t.Run("inclusive tax is backed out", func(t *testing.T) {
		taxID := "tax-ppn-inc"
		rates := map[string]TaxRate{taxID: {Rate: 11, CalcMethod: "inclusive"}}
		calc, err := CalcLines([]LineInput{lcLine("Jasa A", 1, 111000, 0, taxID)}, rates, AdditionalDiscount{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// 111000 already includes 11% tax → DPP = 111000/1.11 = 100000, tax = 11000.
		if math.Abs(calc.TaxTotal-11000) > 0.01 {
			t.Errorf("expected tax ~11000, got %.2f", calc.TaxTotal)
		}
		if calc.GrandTotal != 111000 {
			t.Errorf("expected grand total to stay 111000 (already tax-inclusive), got %.2f", calc.GrandTotal)
		}
	})

	t.Run("two exclusive taxes on one line sum", func(t *testing.T) {
		vat := "tax-ppn"
		svc := "tax-svc"
		rates := map[string]TaxRate{
			vat: {Rate: 11, CalcMethod: "exclusive"},
			svc: {Rate: 2, CalcMethod: "exclusive"},
		}
		calc, err := CalcLines([]LineInput{lcLine("Jasa A", 1, 100000, 0, vat, svc)}, rates, AdditionalDiscount{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calc.TaxTotal != 13000 || calc.GrandTotal != 113000 {
			t.Errorf("unexpected totals: %+v", calc)
		}
	})

	t.Run("mixed exclusive and inclusive taxes on one line", func(t *testing.T) {
		exclusive := "tax-exc"
		inclusive := "tax-inc"
		rates := map[string]TaxRate{
			exclusive: {Rate: 10, CalcMethod: "exclusive"},
			inclusive: {Rate: 10, CalcMethod: "inclusive"},
		}
		calc, err := CalcLines([]LineInput{lcLine("Jasa A", 1, 100000, 0, exclusive, inclusive)}, rates, AdditionalDiscount{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// inclusive: 100000 - 100000/1.10 = 9090.91; exclusive: 100000*10% = 10000.
		if math.Abs(calc.TaxTotal-19090.91) > 0.01 {
			t.Errorf("expected tax ~19090.91, got %.2f", calc.TaxTotal)
		}
		// grand total only grows by the exclusive portion (inclusive stays embedded).
		if calc.GrandTotal != 110000 {
			t.Errorf("expected grand total 110000, got %.2f", calc.GrandTotal)
		}
	})

	t.Run("multiple lines sum correctly", func(t *testing.T) {
		taxID := "tax-ppn"
		rates := map[string]TaxRate{taxID: {Rate: 10, CalcMethod: "exclusive"}}
		calc, err := CalcLines([]LineInput{
			lcLine("Jasa A", 2, 50000, 0, taxID), // 100000 + 10% = 110000
			lcLine("Jasa B", 1, 20000, 0),        // 20000, no tax
		}, rates, AdditionalDiscount{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calc.Subtotal != 120000 || calc.TaxTotal != 10000 || calc.GrandTotal != 130000 {
			t.Errorf("unexpected totals: %+v", calc)
		}
	})

	t.Run("rejects empty product name", func(t *testing.T) {
		if _, err := CalcLines([]LineInput{lcLine("", 1, 1000, 0)}, nil, AdditionalDiscount{}); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("rejects zero quantity", func(t *testing.T) {
		if _, err := CalcLines([]LineInput{lcLine("Jasa A", 0, 1000, 0)}, nil, AdditionalDiscount{}); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("rejects discount over 100", func(t *testing.T) {
		if _, err := CalcLines([]LineInput{lcLine("Jasa A", 1, 1000, 150)}, nil, AdditionalDiscount{}); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("rejects unknown tax id", func(t *testing.T) {
		if _, err := CalcLines([]LineInput{lcLine("Jasa A", 1, 1000, 0, "missing")}, map[string]TaxRate{}, AdditionalDiscount{}); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("rejects zero lines", func(t *testing.T) {
		if _, err := CalcLines(nil, nil, AdditionalDiscount{}); err == nil {
			t.Fatal("expected an error")
		}
	})
}

func TestCalcLinesAdditionalDiscount(t *testing.T) {
	t.Run("percent additional discount reduces grand total", func(t *testing.T) {
		calc, err := CalcLines(
			[]LineInput{lcLine("Jasa A", 1, 100000, 0)}, nil,
			AdditionalDiscount{Type: "percent", Value: 10},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Subtotal 100000, 10% additional discount = 10000 off.
		if calc.AdditionalDiscountAmount != 10000 || calc.GrandTotal != 90000 {
			t.Errorf("unexpected totals: %+v", calc)
		}
	})

	t.Run("amount additional discount reduces grand total", func(t *testing.T) {
		calc, err := CalcLines(
			[]LineInput{lcLine("Jasa A", 1, 100000, 0)}, nil,
			AdditionalDiscount{Type: "amount", Value: 25000},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calc.AdditionalDiscountAmount != 25000 || calc.GrandTotal != 75000 {
			t.Errorf("unexpected totals: %+v", calc)
		}
	})

	t.Run("rejects amount additional discount exceeding subtotal", func(t *testing.T) {
		_, err := CalcLines(
			[]LineInput{lcLine("Jasa A", 1, 100000, 0)}, nil,
			AdditionalDiscount{Type: "amount", Value: 150000},
		)
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("rejects percent additional discount over 100", func(t *testing.T) {
		_, err := CalcLines(
			[]LineInput{lcLine("Jasa A", 1, 100000, 0)}, nil,
			AdditionalDiscount{Type: "percent", Value: 150},
		)
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("inclusive tax plus additional discount does not double-count", func(t *testing.T) {
		// Regression guard: GrandTotal must be derived by subtracting the
		// additional discount from the already-correct GrandTotal, not by
		// reconstructing Subtotal+TaxTotal (which would double-count the
		// inclusive tax already embedded in Subtotal).
		taxID := "tax-ppn-inc"
		rates := map[string]TaxRate{taxID: {Rate: 11, CalcMethod: "inclusive"}}
		calc, err := CalcLines(
			[]LineInput{lcLine("Jasa A", 1, 111000, 0, taxID)}, rates,
			AdditionalDiscount{Type: "amount", Value: 11000},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Without the discount, GrandTotal stays 111000 (already tax-inclusive).
		// With an 11000 flat discount subtracted from that: 100000.
		if calc.GrandTotal != 100000 {
			t.Errorf("expected grand total 100000, got %.2f", calc.GrandTotal)
		}
	})
}
