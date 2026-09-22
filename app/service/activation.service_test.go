package service

import (
	"errors"
	"testing"
	"time"

	domain "duluin_invoice/app/domain/activation"
	"duluin_invoice/app/model"
)

type fakeActivationCompanies struct {
	company *model.Company
	saved   int
}

func (f *fakeActivationCompanies) FindCompanyByID(id string) (*model.Company, error) {
	return f.company, nil
}
func (f *fakeActivationCompanies) SaveCompany(c *model.Company) error {
	f.saved++
	f.company = c
	return nil
}

// fakeCounter is a single-value stand-in for CountActive/CountCreatedSince alone (partners, sales
// orders, purchase orders — each is its own small interface).
type fakeCounter struct{ n int64 }

func (f *fakeCounter) CountActive(companyID string) (int64, error)        { return f.n, nil }
func (f *fakeCounter) CountCreatedSince(string, time.Time) (int64, error) { return f.n, nil }

type fakeActivationCounts struct {
	salesInvoices     int64
	purchaseInvoices  int64
	salesThisMonth    int64
	purchaseThisMonth int64
}

func (f *fakeActivationCounts) CountByKind(companyID, kind string) (int64, error) {
	return f.salesInvoices, nil
}
func (f *fakeActivationCounts) CountCreatedSince(companyID string, since time.Time) (int64, error) {
	return f.salesThisMonth, nil
}
func (f *fakeActivationCounts) CountAll(companyID string) (int64, error) {
	return f.purchaseInvoices, nil
}

// purchase counter needs its own CountCreatedSince value, so wrap separately.
type fakePurchaseCounts struct{ *fakeActivationCounts }

func (f *fakePurchaseCounts) CountCreatedSince(companyID string, since time.Time) (int64, error) {
	return f.purchaseThisMonth, nil
}

func completeCompany() *model.Company {
	return &model.Company{ID: "c1", Name: "PT Contoh", Email: "a@b.com", Phone: "0812", JenisUsaha: "Retail", JumlahKaryawan: "1-10"}
}

func newActivationTestSvc(companies *fakeActivationCompanies, partners int64, invoices *fakeActivationCounts) *ActivationService {
	return NewActivationService(
		companies,
		&fakeCounter{n: partners},
		invoices,
		&fakePurchaseCounts{invoices},
		&fakeCounter{},
		&fakeCounter{},
	)
}

func TestActivationProgress_IncompleteStaysInitial(t *testing.T) {
	companies := &fakeActivationCompanies{company: completeCompany()}
	invoices := &fakeActivationCounts{salesInvoices: 0, purchaseInvoices: 0}
	svc := newActivationTestSvc(companies, 2, invoices)

	p, err := svc.Progress("c1")
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != domain.StatusInitial || p.Completed != 1 || p.Total != 3 {
		t.Fatalf("want initial 1/3, got status=%s completed=%d/%d", p.Status, p.Completed, p.Total)
	}
	if companies.saved != 0 {
		t.Fatalf("must not persist activation until all three are met, saved=%d times", companies.saved)
	}
}

func TestActivationProgress_CompletesAndFlipsOnce(t *testing.T) {
	companies := &fakeActivationCompanies{company: completeCompany()}
	invoices := &fakeActivationCounts{salesInvoices: 1, purchaseInvoices: 0}
	svc := newActivationTestSvc(companies, 3, invoices)

	p, err := svc.Progress("c1")
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != domain.StatusActivated || p.Completed != 3 || p.ActivatedAt == nil {
		t.Fatalf("want activated 3/3, got %+v", p)
	}
	if companies.saved != 1 {
		t.Fatalf("want exactly one save on the flip, got %d", companies.saved)
	}
	if p.Limits != domain.FullLimits {
		t.Fatalf("want full limits once activated, got %+v", p.Limits)
	}

	// The company row now carries "activated" — a fresh Progress call, even with a lower live
	// partner count, must not re-lock it (one-way flip).
	companies.company.ActivationStatus = domain.StatusActivated
	svc2 := newActivationTestSvc(companies, 1, invoices)
	p2, err := svc2.Progress("c1")
	if err != nil {
		t.Fatal(err)
	}
	if p2.Status != domain.StatusActivated {
		t.Fatalf("activation must be one-way, got status=%s after partner count dropped", p2.Status)
	}
	if companies.saved != 1 {
		t.Fatalf("must not save again once already activated, saved=%d", companies.saved)
	}
}

func TestActivationLimits_RejectOverInitialCap(t *testing.T) {
	companies := &fakeActivationCompanies{company: completeCompany()} // ActivationStatus == "" -> initial
	invoices := &fakeActivationCounts{}
	svc := newActivationTestSvc(companies, 10, invoices) // at the initial partner cap

	err := svc.CheckPartnerLimit("c1")
	var limit *domain.ErrLimitReached
	if !errors.As(err, &limit) || limit.Resource != "partners" || limit.Limit != 10 {
		t.Fatalf("want ErrLimitReached{partners,10}, got %v", err)
	}
}

func TestActivationLimits_FullTierAfterActivation(t *testing.T) {
	companies := &fakeActivationCompanies{company: &model.Company{ID: "c1", ActivationStatus: domain.StatusActivated}}
	invoices := &fakeActivationCounts{}
	svc := newActivationTestSvc(companies, 20, invoices) // over the initial 10 but under the full 50

	if err := svc.CheckPartnerLimit("c1"); err != nil {
		t.Fatalf("an activated company should get the full 50-partner limit, got %v", err)
	}
}

func TestActivationLimits_TransactionCountIncludesOrders(t *testing.T) {
	companies := &fakeActivationCompanies{company: &model.Company{ID: "c1", ActivationStatus: domain.StatusActivated}}
	invoices := &fakeActivationCounts{salesThisMonth: 10, purchaseThisMonth: 5}
	svc := NewActivationService(companies, &fakeCounter{}, invoices, &fakePurchaseCounts{invoices}, &fakeCounter{n: 3}, &fakeCounter{n: 2})

	// 10 + 5 + 3 + 2 = 20, at the full-tier cap.
	err := svc.CheckTransactionLimit("c1")
	var limit *domain.ErrLimitReached
	if !errors.As(err, &limit) || limit.Resource != "transactions_per_month" || limit.Limit != 20 {
		t.Fatalf("want the 20 cap to include sales/purchase orders, got %v", err)
	}
}
