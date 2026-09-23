// Package domain_documenttemplate — per-company default template for each printable document type.
// A default only seeds NEW documents; changing it never touches a document that already exists.
package domain_documenttemplate

import "duluin_invoice/app/model"

// SetDTO — the template to use by default.
type SetDTO struct {
	Template string `json:"template" validate:"required,oneof=template_1 template_2 template_3 template_4 template_5 template_6 template_7"`
}

// Item — one document type with its effective default (template_1 when the company never set one).
type Item struct {
	DocType  string `json:"doc_type"`
	Template string `json:"template"`
}

type IRepository interface {
	List(companyID string) ([]Item, error)
	Set(companyID, docType, template, actorID string) (*Item, error)
}

type IService interface {
	List(companyID string) ([]Item, error)
	Set(companyID, actorID, docType string, dto *SetDTO) (*Item, error)
}

type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }

var _ = model.DefaultSalesInvoiceTemplate
