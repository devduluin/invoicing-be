package service

import (
	"regexp"

	domain "duluin_invoice/app/domain/documentconfig"
)

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

var (
	pageSizes    = map[string]bool{"": true, "a4": true, "a5": true, "letter": true}
	orientations = map[string]bool{"": true, "portrait": true, "landscape": true}
	fontKeys     = map[string]bool{"": true, "inter": true, "sans": true, "serif": true, "mono": true}
	aligns       = map[string]bool{"": true, "left": true, "center": true, "right": true}
	numberFmts   = map[string]bool{"": true, "id": true, "en": true, "space": true}
	currencies   = map[string]bool{"": true, "rp": true, "idr": true, "usd": true, "none": true}
	dateFmts     = map[string]bool{"": true, "dmy": true, "dmy-dash": true, "ymd": true, "long": true}
	taxFmts      = map[string]bool{"": true, "name": true, "rate": true}
	discountFmts = map[string]bool{"": true, "entered": true, "percent": true, "amount": true}
	textGroups   = map[string]bool{"title": true, "heading": true, "body": true, "tableHead": true, "tableBody": true, "total": true}
)

var styleKeys = map[string]bool{
	"default": true, "template_1": true, "template_2": true, "template_3": true, "template_4": true,
	"template_5": true, "template_6": true, "template_7": true,
}

// validateTemplateStyle checks one template's look. Only out-of-range or unknown values are
// rejected, so a crafted request can never store a colour, size or margin the renderer would choke on.
func validateTemplateStyle(st *domain.TemplateStyle) error {
	bad := func(msg string) error { return &domain.ErrValidation{Message: msg} }
	if c := st.Appearance.Color; c != "" && !hexColor.MatchString(c) {
		return bad("document color must be a #rrggbb value")
	}
	if !pageSizes[st.Page.Size] {
		return bad("unknown page size")
	}
	if !orientations[st.Page.Orientation] {
		return bad("unknown page orientation")
	}
	for _, m := range []*float64{st.Page.Margins.Top, st.Page.Margins.Bottom, st.Page.Margins.Left, st.Page.Margins.Right} {
		if m != nil && (*m < 0 || *m > 50) {
			return bad("margins must be between 0 and 50 mm")
		}
	}
	if len(st.TextStyles) > len(textGroups) {
		return bad("too many text styles")
	}
	for group, ts := range st.TextStyles {
		if !textGroups[group] {
			return bad("unknown text style")
		}
		if !fontKeys[ts.Font] {
			return bad("unknown font")
		}
		if ts.Size != nil && (*ts.Size < 6 || *ts.Size > 40) {
			return bad("font size must be between 6 and 40 pt")
		}
		if ts.Color != "" && !hexColor.MatchString(ts.Color) {
			return bad("text color must be a #rrggbb value")
		}
		if !aligns[ts.Align] {
			return bad("unknown text alignment")
		}
	}
	return nil
}

// validateStyle checks the per-template looks and the document-level number / date formats.
func validateStyle(cfg *domain.Config) error {
	bad := func(msg string) error { return &domain.ErrValidation{Message: msg} }
	if len(cfg.TemplateStyles) > len(styleKeys) {
		return bad("too many template styles")
	}
	for key, st := range cfg.TemplateStyles {
		if !styleKeys[key] {
			return bad("unknown template")
		}
		st := st
		if err := validateTemplateStyle(&st); err != nil {
			return err
		}
	}
	f := cfg.Formats
	if !numberFmts[f.Number] || !currencies[f.Currency] || !dateFmts[f.Date] || !taxFmts[f.Tax] || !discountFmts[f.Discount] {
		return bad("unknown number / currency / date format")
	}
	if f.Decimals != nil && (*f.Decimals < 0 || *f.Decimals > 4) {
		return bad("decimal places must be between 0 and 4")
	}
	return nil
}
