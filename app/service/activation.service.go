package service

import (
	"fmt"
	"strings"
	"time"

	domain "duluin_invoice/app/domain/activation"
	"duluin_invoice/app/model"
)

const (
	requiredPartners = 3
	requiredInvoices = 1
)

// The slice of persistence this service needs — small, local interfaces (this codebase's usual
// pattern, see e.g. CompanyController's CompanyLister/CompanyDirectory) so activation doesn't force
// a dependency on any repository's full surface.
type activationCompanyStore interface {
	FindCompanyByID(id string) (*model.Company, error)
	SaveCompany(c *model.Company) error
}

type activationPartnerCounter interface {
	CountActive(companyID string) (int64, error)
}

type activationInvoiceCounter interface {
	CountByKind(companyID, kind string) (int64, error)
	CountCreatedSince(companyID string, since time.Time) (int64, error)
}

type activationPurchaseCounter interface {
	CountAll(companyID string) (int64, error)
	CountCreatedSince(companyID string, since time.Time) (int64, error)
}

// activationOrderCounter — Sales Order / Purchase Order also count toward the transactions/month
// limit (see the Free-plan transaction definition); they have no activation-milestone role.
type activationOrderCounter interface {
	CountCreatedSince(companyID string, since time.Time) (int64, error)
}

// ActivationService computes the three-step activation milestone (company profile + 3 partners +
// 1 invoice — Sales or Purchase, never a Product/Service, which Duluin Invoice has no master for)
// and enforces the Free-tier limits that depend on it.
type ActivationService struct {
	companies   activationCompanyStore
	partners    activationPartnerCounter
	sales       activationInvoiceCounter
	purchase    activationPurchaseCounter
	salesOrders activationOrderCounter
	purchOrders activationOrderCounter
}

func NewActivationService(
	companies activationCompanyStore,
	partners activationPartnerCounter,
	sales activationInvoiceCounter,
	purchase activationPurchaseCounter,
	salesOrders activationOrderCounter,
	purchOrders activationOrderCounter,
) *ActivationService {
	return &ActivationService{
		companies: companies, partners: partners,
		sales: sales, purchase: purchase, salesOrders: salesOrders, purchOrders: purchOrders,
	}
}

// companyProfileComplete — the Required fields from Settings → Company (§3 of the Free-plan spec):
// name, email, phone, industry, company size. Address/city/province/postal code/website/NPWP stay
// optional — never required for activation.
func companyProfileComplete(c *model.Company) bool {
	return strings.TrimSpace(c.Name) != "" &&
		strings.TrimSpace(c.Email) != "" &&
		strings.TrimSpace(c.Phone) != "" &&
		strings.TrimSpace(c.JenisUsaha) != "" &&
		strings.TrimSpace(c.JumlahKaryawan) != ""
}

func (s *ActivationService) invoiceCount(companyID string) (int64, error) {
	sales, err := s.sales.CountByKind(companyID, string(model.SalesInvoiceKindInvoice))
	if err != nil {
		return 0, err
	}
	purchase, err := s.purchase.CountAll(companyID)
	if err != nil {
		return 0, err
	}
	return sales + purchase, nil
}

// Progress computes the checklist live and, the first time all three steps are met, persists
// ActivationStatus = activated (a one-way flip — later deleting a partner or invoice back below
// the threshold never re-locks it).
func (s *ActivationService) Progress(companyID string) (*domain.Progress, error) {
	c, err := s.companies.FindCompanyByID(companyID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, &domain.ErrCompanyNotFound{}
	}

	partners, err := s.partners.CountActive(companyID)
	if err != nil {
		return nil, err
	}
	invoices, err := s.invoiceCount(companyID)
	if err != nil {
		return nil, err
	}

	profileDone := companyProfileComplete(c)
	partnersDone := partners >= int64(requiredPartners)
	invoiceDone := invoices >= int64(requiredInvoices)

	toInt := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	reqs := []domain.Requirement{
		{Key: "company_profile", Done: profileDone, Current: toInt(profileDone), Required: 1},
		{Key: "partners", Done: partnersDone, Current: int(partners), Required: requiredPartners},
		{Key: "invoice", Done: invoiceDone, Current: int(invoices), Required: requiredInvoices},
	}
	completed := 0
	for _, r := range reqs {
		if r.Done {
			completed++
		}
	}

	status := c.ActivationStatus
	if status == "" {
		status = domain.StatusInitial
	}
	activatedAt := c.ActivatedAt

	if status != domain.StatusActivated && completed == len(reqs) {
		now := time.Now()
		c.ActivationStatus = domain.StatusActivated
		c.ActivatedAt = &now
		if err := s.companies.SaveCompany(c); err != nil {
			return nil, fmt.Errorf("activate company: %w", err)
		}
		status = domain.StatusActivated
		activatedAt = &now
	}

	return &domain.Progress{
		Status:       status,
		ActivatedAt:  activatedAt,
		Completed:    completed,
		Total:        len(reqs),
		Requirements: reqs,
		Limits:       domain.LimitsFor(status),
	}, nil
}

// Recompute is Progress, called for its side effect (the one-way activation flip) right after a
// partner or invoice is created, so "Your Free workspace is activated!" shows up immediately
// instead of waiting for the next page load. Best-effort: a failure here never fails the create
// that triggered it.
func (s *ActivationService) Recompute(companyID string) {
	_, _ = s.Progress(companyID)
}

func (s *ActivationService) limitsFor(companyID string) (domain.Limits, error) {
	c, err := s.companies.FindCompanyByID(companyID)
	if err != nil {
		return domain.Limits{}, err
	}
	if c == nil {
		return domain.Limits{}, &domain.ErrCompanyNotFound{}
	}
	return domain.LimitsFor(c.ActivationStatus), nil
}

func (s *ActivationService) activated(companyID string) bool {
	c, err := s.companies.FindCompanyByID(companyID)
	return err == nil && c != nil && c.ActivationStatus == domain.StatusActivated
}

// CheckPartnerLimit — called before a partner is created.
func (s *ActivationService) CheckPartnerLimit(companyID string) error {
	limits, err := s.limitsFor(companyID)
	if err != nil {
		return err
	}
	n, err := s.partners.CountActive(companyID)
	if err != nil {
		return err
	}
	if int(n) >= limits.Partners {
		return &domain.ErrLimitReached{Resource: "partners", Limit: limits.Partners, Activated: s.activated(companyID)}
	}
	return nil
}

// CheckTransactionLimit — called before a Sales Order, Down Payment, Sales Invoice, Purchase Order
// or Purchase Invoice is created (the Free plan's transaction definition — edit/delete/print/PDF/
// view/record-payment never call this). Down Payment is a SalesInvoice row (Kind=down_payment), so
// it is already included in the sales-invoice count below.
func (s *ActivationService) CheckTransactionLimit(companyID string) error {
	limits, err := s.limitsFor(companyID)
	if err != nil {
		return err
	}
	since := time.Now()
	since = time.Date(since.Year(), since.Month(), 1, 0, 0, 0, 0, since.Location())
	salesInvoiceN, err := s.sales.CountCreatedSince(companyID, since)
	if err != nil {
		return err
	}
	purchaseInvoiceN, err := s.purchase.CountCreatedSince(companyID, since)
	if err != nil {
		return err
	}
	salesOrderN, err := s.salesOrders.CountCreatedSince(companyID, since)
	if err != nil {
		return err
	}
	purchaseOrderN, err := s.purchOrders.CountCreatedSince(companyID, since)
	if err != nil {
		return err
	}
	total := int(salesInvoiceN + purchaseInvoiceN + salesOrderN + purchaseOrderN)
	if total >= limits.TransactionsPerMonth {
		return &domain.ErrLimitReached{Resource: "transactions_per_month", Limit: limits.TransactionsPerMonth, Activated: s.activated(companyID)}
	}
	return nil
}
