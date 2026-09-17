package utils

import (
	"fmt"
	"math"
	"strings"
)

// LineInput is one line of any tax-aware line-item document (Sales Order,
// Sales Invoice, ...). Shared so the tax math (discount, exclusive/inclusive
// tax) is written and tested once, not once per document type.
type LineInput struct {
	ProductName   string
	Description   string
	Quantity      float64
	UnitPrice     float64
	DiscountType  string // "percent" | "amount" (default "percent")
	DiscountValue float64
	TaxIDs        []string
}

// LineResult — LineInput plus its computed amounts.
type LineResult struct {
	ProductName   string
	Description   string
	Quantity      float64
	UnitPrice     float64
	DiscountType  string
	DiscountValue float64
	TaxIDs        []string
	LineSubtotal  float64
	LineTaxAmount float64
	LineTotal     float64
}

// TaxRate is the two fields CalcLines needs from a referenced tax.
type TaxRate struct {
	Rate       float64
	CalcMethod string // "exclusive" | "inclusive"
}

type LinesCalc struct {
	Lines                    []LineResult
	Subtotal                 float64
	DiscountTotal            float64
	TaxTotal                 float64
	GrandTotal               float64
	AdditionalDiscountType   string
	AdditionalDiscountValue  float64
	AdditionalDiscountAmount float64
}

// AdditionalDiscount is a document-level discount applied on top of the
// line totals — Sales Order, Sales Invoice, Purchase Order and Purchase
// Invoice all support one, same percent/amount shape as a per-line discount
// but computed once against the document's Subtotal.
type AdditionalDiscount struct {
	Type  string // "percent" | "amount" (default "percent")
	Value float64
}

// CalcLines — per line: base = qty*unitPrice, discount is either a percent
// of base or a flat amount, lineSubtotal = base-discount. Every referenced
// tax is grouped by calc method: exclusive rates sum and add on top of
// lineSubtotal; inclusive rates sum and are backed out of it. With a single
// tax this reduces to the original one-tax formula exactly. An optional
// document-level additionalDiscount is then subtracted from the resulting
// GrandTotal (see subtraction note below).
func CalcLines(lines []LineInput, rates map[string]TaxRate, additionalDiscount AdditionalDiscount) (*LinesCalc, error) {
	if len(lines) < 1 {
		return nil, fmt.Errorf("a document must have at least 1 line")
	}

	calc := &LinesCalc{Lines: make([]LineResult, 0, len(lines))}
	for i, l := range lines {
		if strings.TrimSpace(l.ProductName) == "" {
			return nil, fmt.Errorf("line %d has no product name", i+1)
		}
		if l.Quantity <= 0 {
			return nil, fmt.Errorf("line %d quantity must be greater than 0", i+1)
		}
		if l.UnitPrice < 0 {
			return nil, fmt.Errorf("line %d price cannot be negative", i+1)
		}

		discountType := l.DiscountType
		if discountType == "" {
			discountType = "percent"
		}
		if discountType != "percent" && discountType != "amount" {
			return nil, fmt.Errorf("line %d has an invalid discount type", i+1)
		}

		base := l.Quantity * l.UnitPrice

		var discountAmt float64
		switch discountType {
		case "amount":
			if l.DiscountValue < 0 {
				return nil, fmt.Errorf("line %d discount cannot be negative", i+1)
			}
			if l.DiscountValue > base {
				return nil, fmt.Errorf("line %d discount cannot exceed the line amount", i+1)
			}
			discountAmt = l.DiscountValue
		default: // percent
			if l.DiscountValue < 0 || l.DiscountValue > 100 {
				return nil, fmt.Errorf("line %d discount must be between 0-100%%", i+1)
			}
			discountAmt = base * l.DiscountValue / 100
		}
		lineSubtotal := base - discountAmt

		var exclusiveRate, inclusiveRate float64
		for _, rawID := range l.TaxIDs {
			taxID := strings.TrimSpace(rawID)
			if taxID == "" {
				continue
			}
			info, ok := rates[taxID]
			if !ok {
				return nil, fmt.Errorf("line %d has a tax that was not found or is inactive", i+1)
			}
			if info.CalcMethod == "inclusive" {
				inclusiveRate += info.Rate
			} else {
				exclusiveRate += info.Rate
			}
		}
		exclusiveTax := lineSubtotal * exclusiveRate / 100
		var inclusiveTax float64
		if inclusiveRate > 0 {
			inclusiveTax = lineSubtotal - lineSubtotal/(1+inclusiveRate/100)
		}
		lineTax := exclusiveTax + inclusiveTax
		lineTotal := lineSubtotal + exclusiveTax

		calc.Lines = append(calc.Lines, LineResult{
			ProductName: strings.TrimSpace(l.ProductName), Description: strings.TrimSpace(l.Description),
			Quantity: l.Quantity, UnitPrice: l.UnitPrice, DiscountType: discountType, DiscountValue: l.DiscountValue,
			TaxIDs:       l.TaxIDs,
			LineSubtotal: round2(lineSubtotal), LineTaxAmount: round2(lineTax), LineTotal: round2(lineTotal),
		})
		calc.Subtotal += lineSubtotal
		calc.DiscountTotal += discountAmt
		calc.TaxTotal += lineTax
		calc.GrandTotal += lineTotal
	}
	calc.Subtotal = round2(calc.Subtotal)
	calc.DiscountTotal = round2(calc.DiscountTotal)
	calc.TaxTotal = round2(calc.TaxTotal)
	calc.GrandTotal = round2(calc.GrandTotal)

	discType := additionalDiscount.Type
	if discType == "" {
		discType = "percent"
	}
	if discType != "percent" && discType != "amount" {
		return nil, fmt.Errorf("invalid additional discount type")
	}

	var addDiscAmt float64
	switch discType {
	case "amount":
		if additionalDiscount.Value < 0 {
			return nil, fmt.Errorf("additional discount cannot be negative")
		}
		if additionalDiscount.Value > calc.Subtotal {
			return nil, fmt.Errorf("additional discount cannot exceed the subtotal")
		}
		addDiscAmt = additionalDiscount.Value
	default: // percent
		if additionalDiscount.Value < 0 || additionalDiscount.Value > 100 {
			return nil, fmt.Errorf("additional discount must be between 0-100%%")
		}
		addDiscAmt = calc.Subtotal * additionalDiscount.Value / 100
	}

	calc.AdditionalDiscountType = discType
	calc.AdditionalDiscountValue = additionalDiscount.Value
	calc.AdditionalDiscountAmount = round2(addDiscAmt)
	// Subtracted from the already-correct GrandTotal rather than
	// reconstructed from Subtotal+TaxTotal: inclusive taxes are backed out
	// of (already embedded in) each line's subtotal while TaxTotal still
	// reports their amount for display, so Subtotal+TaxTotal would
	// double-count them. Subtracting here sidesteps that entirely.
	calc.GrandTotal = round2(calc.GrandTotal - calc.AdditionalDiscountAmount)
	return calc, nil
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
