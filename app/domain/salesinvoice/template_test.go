package domain_salesinvoice_test

import (
	"testing"

	domain "duluin_invoice/app/domain/salesinvoice"
	"duluin_invoice/app/validation"
)

func validDTO(template string) *domain.CreateDTO {
	return &domain.CreateDTO{
		Kind:     "invoice",
		MitraID:  "11111111-1111-4111-8111-111111111111",
		Date:     "2026-09-20",
		DueDate:  "2026-10-20",
		Template: template,
		Lines:    []domain.LineDTO{{ProductName: "A", Quantity: 1}},
	}
}

func TestCreateDTO_TemplateValidation(t *testing.T) {
	for _, tpl := range []string{"", "template_1", "template_2", "template_3", "template_4"} {
		if msgs := validation.Struct(validDTO(tpl)); msgs != nil {
			t.Errorf("template %q should be valid, got %v", tpl, msgs)
		}
	}
	for _, tpl := range []string{"template_5", "Template_1", "classic", "template_1; DROP"} {
		if msgs := validation.Struct(validDTO(tpl)); msgs == nil {
			t.Errorf("template %q should be rejected", tpl)
		}
	}
}
