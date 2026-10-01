package model

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestMitraContactPersonsIsHasMany(t *testing.T) {
	s, err := schema.Parse(&Mitra{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	if r := s.Relationships.Relations["ContactPersons"]; r == nil || r.Type != schema.HasMany {
		t.Fatalf("want ContactPersons has_many, got %v", r)
	}
}

func TestListRelationsAreBelongsTo(t *testing.T) {
	cases := []struct {
		model any
		rels  []string
	}{
		{&SalesOrder{}, []string{"MitraRel", "SalespersonRel"}},
		{&SalesInvoice{}, []string{"MitraRel", "SalesOrderRel", "LinkedInvoiceRel", "SalespersonRel"}},
		{&PurchaseOrder{}, []string{"MitraRel"}},
		{&PurchaseInvoice{}, []string{"MitraRel", "PurchaseOrderRel"}},
		{&SalesReceipt{}, []string{"MitraRel", "BankAccountRel"}},
		{&PurchaseReceipt{}, []string{"MitraRel", "BankAccountRel"}},
		{&DeliveryNote{}, []string{"MitraRel", "SalesOrderRel", "SalesInvoiceRel"}},
		{&GoodsReceipt{}, []string{"MitraRel", "PurchaseOrderRel"}},
		{&JournalEntry{}, []string{"JournalBookRel"}},
		{&JournalBook{}, []string{"DefaultAccountRel", "DefaultDebitAccountRel", "DefaultCreditAccountRel"}},
		{&Account{}, []string{"ParentRel"}},
		{&Tax{}, []string{"SalesAccountRel", "PurchaseAccountRel", "Component1Rel", "Component2Rel"}},
		{&Mitra{}, []string{"LinkedCompanyRel"}},
		{&SalesReceiptAllocation{}, []string{"SalesInvoiceRel"}},
		{&PurchaseReceiptAllocation{}, []string{"PurchaseInvoiceRel"}},
		{&JournalLine{}, []string{"AccountRel"}},
	}
	for _, c := range cases {
		s, err := schema.Parse(c.model, &sync.Map{}, schema.NamingStrategy{})
		if err != nil {
			t.Fatalf("%T: %v", c.model, err)
		}
		for _, name := range c.rels {
			r, ok := s.Relationships.Relations[name]
			if !ok {
				t.Fatalf("%T: no relation %s", c.model, name)
			}
			if r.Type != schema.BelongsTo {
				t.Errorf("%T.%s: want belongs_to, got %s", c.model, name, r.Type)
			}
		}
	}
}
