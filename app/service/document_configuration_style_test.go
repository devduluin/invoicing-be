package service

import (
	"testing"

	domain "duluin_invoice/app/domain/documentconfig"
)

func ptrF(v float64) *float64 { return &v }
func ptrI(v int) *int         { return &v }

func one(key string, st domain.TemplateStyle) *domain.Config {
	return &domain.Config{TemplateStyles: map[string]domain.TemplateStyle{key: st}}
}

func TestValidateStyle(t *testing.T) {
	ok := &domain.Config{
		TemplateStyles: map[string]domain.TemplateStyle{
			"template_4": {
				Appearance: domain.Appearance{Color: "#4863E6"},
				Page:       domain.PageLayout{Size: "a4", Orientation: "landscape", Margins: domain.Margins{Top: ptrF(15)}},
				TextStyles: map[string]domain.TextStyle{"title": {Font: "serif", Size: ptrF(14), Color: "#000000", Align: "center"}},
			},
			"default": {Appearance: domain.Appearance{Color: "#16A34A"}},
		},
		Formats: domain.Formats{Number: "en", Decimals: ptrI(2), Currency: "usd", Date: "long", Tax: "rate", Discount: "amount"},
	}
	if err := validateStyle(ok); err != nil {
		t.Fatalf("valid style rejected: %v", err)
	}
	if err := validateStyle(&domain.Config{}); err != nil {
		t.Fatalf("empty (all defaults) rejected: %v", err)
	}

	cases := map[string]*domain.Config{
		"unknown template": one("template_9", domain.TemplateStyle{}),
		"bad color":        one("template_1", domain.TemplateStyle{Appearance: domain.Appearance{Color: "blue"}}),
		"css injection":    one("template_1", domain.TemplateStyle{Appearance: domain.Appearance{Color: "#fff;background:url(x)"}}),
		"bad size":         one("template_1", domain.TemplateStyle{Page: domain.PageLayout{Size: "a0"}}),
		"bad orientation":  one("template_1", domain.TemplateStyle{Page: domain.PageLayout{Orientation: "diagonal"}}),
		"huge margin":      one("template_1", domain.TemplateStyle{Page: domain.PageLayout{Margins: domain.Margins{Left: ptrF(500)}}}),
		"neg margin":       one("template_1", domain.TemplateStyle{Page: domain.PageLayout{Margins: domain.Margins{Top: ptrF(-1)}}}),
		"unknown group":    one("template_1", domain.TemplateStyle{TextStyles: map[string]domain.TextStyle{"footer": {}}}),
		"unknown font":     one("template_1", domain.TemplateStyle{TextStyles: map[string]domain.TextStyle{"title": {Font: "comic"}}}),
		"tiny font":        one("template_1", domain.TemplateStyle{TextStyles: map[string]domain.TextStyle{"title": {Size: ptrF(2)}}}),
		"bad align":        one("template_1", domain.TemplateStyle{TextStyles: map[string]domain.TextStyle{"title": {Align: "justify"}}}),
		"bad decimals":     {Formats: domain.Formats{Decimals: ptrI(9)}},
		"bad date":         {Formats: domain.Formats{Date: "mdy"}},
	}
	for name, cfg := range cases {
		if err := validateStyle(cfg); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}
}
