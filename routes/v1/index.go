package v1

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"duluin_invoice/app/controller"
	audit "duluin_invoice/app/domain/audit"
	"duluin_invoice/app/notification"
	"duluin_invoice/app/repository"
	"duluin_invoice/app/service"
	"duluin_invoice/app/sso"
	"duluin_invoice/config"
	"duluin_invoice/database"
	"duluin_invoice/middlewares"
)

// RegisterRoutes builds the dependency graph once (constructor injection, no
// package-level globals) and mounts every /api/v1 route.
func RegisterRoutes(router fiber.Router, db *gorm.DB) {
	router.Get("/ping", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true, "message": "API v1 running"})
	})

	// ── infrastructure adapters ──
	ssoClient := sso.NewClient(config.AppConfig.SSOURL, config.AppConfig.SSOAccountType, config.AppConfig.APIKey)
	notifier := notification.NewSSOInviteService(ssoClient, "duluin_invoice_invite")
	rbacCache := service.NewRedisRBACCache(database.Redis)

	// ── repositories ──
	auditRepo := repository.NewAuditRepository(db)
	auditSvc := service.NewAuditService(auditRepo)
	onboardingRepo := repository.NewOnboardingRepository(db)
	membershipRepo := repository.NewMembershipRepository(db)
	mitraRepo := repository.NewMitraRepository(db)
	contactPersonSvc := service.NewContactPersonService(repository.NewContactPersonRepository(db))
	contactPersonCtrl := controller.NewContactPersonController(contactPersonSvc)
	bankAccountRepo := repository.NewBankAccountRepository(db)
	accountRepo := repository.NewAccountRepository(db)
	taxRepo := repository.NewTaxRepository(db)
	unitRepo := repository.NewUnitRepository(db)
	journalBookRepo := repository.NewJournalBookRepository(db)
	journalRepo := repository.NewJournalRepository(db)
	reportRepo := repository.NewReportRepository(db)
	salesOrderRepo := repository.NewSalesOrderRepository(db)
	salesInvoiceRepo := repository.NewSalesInvoiceRepository(db)
	documentTemplateRepo := repository.NewDocumentTemplateRepository(db)
	connectedDocumentCtrl := controller.NewConnectedDocumentController(repository.NewConnectedDocumentRepository(db))
	documentConfigCtrl := controller.NewDocumentConfigurationController(service.NewDocumentConfigurationService(repository.NewDocumentConfigurationRepository(db)), auditSvc)
	salesReceiptRepo := repository.NewSalesReceiptRepository(db)
	salesPaymentRepo := repository.NewSalesPaymentRepository(db)
	purchaseOrderRepo := repository.NewPurchaseOrderRepository(db)
	purchaseInvoiceRepo := repository.NewPurchaseInvoiceRepository(db)
	purchaseReceiptRepo := repository.NewPurchaseReceiptRepository(db)
	deliveryNoteRepo := repository.NewDeliveryNoteRepository(db)
	goodsReceiptRepo := repository.NewGoodsReceiptRepository(db)

	// activationSvc first: Mitra/Sales/Purchase Invoice services below enforce the Free-tier limits
	// it computes, and Membership reads the company row it maintains too.
	activationSvc := service.NewActivationService(onboardingRepo, mitraRepo, salesInvoiceRepo, purchaseInvoiceRepo, salesOrderRepo, purchaseOrderRepo)
	activationCtrl := controller.NewActivationController(activationSvc)

	// ── services ──
	membershipSvc := service.NewMembershipService(
		membershipRepo, onboardingRepo, ssoClient, rbacCache, notifier,
		service.MembershipConfig{
			UseLocalRBAC:          config.AppConfig.UseLocalRBAC,
			RBACMigrationFallback: config.AppConfig.RBACMigrationFallback,
			OwnerRoleID:           config.AppConfig.InvoiceOwnerRoleID,
			ExposeInviteURL:       !strings.EqualFold(config.AppConfig.AppEnv, "production"),
		},
		config.AppConfig.WebURL,
	)
	roleSvc := service.NewRoleService(ssoClient, membershipRepo, membershipSvc)
	defaultsSvc := service.NewMasterDefaultsService(accountRepo, taxRepo, unitRepo)
	onboardingSvc := service.NewOnboardingService(onboardingRepo, membershipSvc, defaultsSvc)
	companySvc := service.NewCompanyService(onboardingRepo, ssoClient)
	mitraSvc := service.NewMitraService(mitraRepo, activationSvc)
	bankDirSvc := service.NewBankDirectoryService(config.AppConfig.BankMetaURL)
	bankAccountSvc := service.NewBankAccountService(bankAccountRepo)
	accountSvc := service.NewAccountService(accountRepo)
	taxSvc := service.NewTaxService(taxRepo)
	unitSvc := service.NewUnitService(unitRepo)
	journalBookSvc := service.NewJournalBookService(journalBookRepo)
	journalSvc := service.NewJournalService(journalRepo)
	reportSvc := service.NewReportService(reportRepo)
	salesOrderSvc := service.NewSalesOrderService(salesOrderRepo, activationSvc)
	salesInvoiceSvc := service.NewSalesInvoiceService(salesInvoiceRepo, activationSvc)
	documentTemplateSvc := service.NewDocumentTemplateService(documentTemplateRepo)
	salesReceiptSvc := service.NewSalesReceiptService(salesReceiptRepo)
	salesPaymentSvc := service.NewSalesPaymentService(salesPaymentRepo)
	purchaseOrderSvc := service.NewPurchaseOrderService(purchaseOrderRepo, activationSvc)
	purchaseInvoiceSvc := service.NewPurchaseInvoiceService(purchaseInvoiceRepo, activationSvc)
	purchaseReceiptSvc := service.NewPurchaseReceiptService(purchaseReceiptRepo)
	deliveryNoteSvc := service.NewDeliveryNoteService(deliveryNoteRepo)
	goodsReceiptSvc := service.NewGoodsReceiptService(goodsReceiptRepo)

	// ── controllers ──
	meCtrl := controller.NewMeController(membershipSvc)
	onboardingCtrl := controller.NewOnboardingController(onboardingSvc, auditSvc)
	companyCtrl := controller.NewCompanyController(membershipSvc, onboardingRepo, companySvc, auditSvc)
	memberCtrl := controller.NewMemberController(membershipSvc, auditSvc)
	auditCtrl := controller.NewAuditController(auditSvc)
	roleCtrl := controller.NewRoleController(roleSvc)
	mitraCtrl := controller.NewMitraController(mitraSvc, auditSvc)
	metaCtrl := controller.NewMetaController(bankDirSvc)
	bankAccountCtrl := controller.NewBankAccountController(bankAccountSvc)
	accountCtrl := controller.NewAccountController(accountSvc)
	taxCtrl := controller.NewTaxController(taxSvc)
	unitCtrl := controller.NewUnitController(unitSvc)
	journalBookCtrl := controller.NewJournalBookController(journalBookSvc)
	journalCtrl := controller.NewJournalController(journalSvc)
	reportCtrl := controller.NewReportController(reportSvc)
	salesOrderCtrl := controller.NewSalesOrderController(salesOrderSvc, ssoClient, auditSvc)
	salesInvoiceCtrl := controller.NewSalesInvoiceController(salesInvoiceSvc, auditSvc, ssoClient)
	documentTemplateCtrl := controller.NewDocumentTemplateController(documentTemplateSvc)
	salesReceiptCtrl := controller.NewSalesReceiptController(salesReceiptSvc, auditSvc, ssoClient)
	salesPaymentCtrl := controller.NewSalesPaymentController(salesPaymentSvc, auditSvc)
	purchaseOrderCtrl := controller.NewPurchaseOrderController(purchaseOrderSvc, ssoClient, auditSvc)
	purchaseInvoiceCtrl := controller.NewPurchaseInvoiceController(purchaseInvoiceSvc, ssoClient, auditSvc)
	purchaseReceiptCtrl := controller.NewPurchaseReceiptController(purchaseReceiptSvc, auditSvc, ssoClient)
	deliveryNoteCtrl := controller.NewDeliveryNoteController(deliveryNoteSvc, ssoClient, auditSvc)
	goodsReceiptCtrl := controller.NewGoodsReceiptController(goodsReceiptSvc, ssoClient, auditSvc)

	// ── routing chain ──
	base := baseRouter(router, onboardingRepo.FindCompanyByID, membershipRepo.DefaultCompanyID)

	MeRoutes(base, meCtrl)
	MetaRoutes(base, metaCtrl)
	OnboardingRoutes(base, onboardingCtrl)
	CompanyRoutes(base, companyCtrl)
	base.Post("/members/me/accept", memberCtrl.Accept)
	// Same handler as POST /members/validate, mounted company-free: Step 4 of the onboarding wizard
	// runs before its company exists (nothing is committed until Submit), so it can't use the
	// company-scoped route. ValidateUser itself never required a company — it only checked the
	// caller's own companies when there were any.
	base.Post("/onboarding/validate-email", memberCtrl.Validate)
	AuditSessionRoute(base, auditCtrl)

	rbac := rbacRouter(base, membershipSvc)
	RoleReadRoutes(rbac, roleCtrl)

	business := rbac.Group("", middlewares.RequireCompletedOnboarding())

	// ── audit trail for the resources whose controllers don't record their own actions ──
	// (documents, partners and sign-in are recorded in their controllers; everything else here.)
	// Registered BEFORE the routes so it wraps them: only requests that succeed are recorded.
	auditWrites := func(prefix string, spec controller.AuditSpec) {
		business.Use(prefix, controller.AuditWrites(auditSvc, spec))
	}
	endsWith := func(suffix string) func(method, path string) bool {
		return func(_, path string) bool { return strings.HasSuffix(strings.TrimRight(path, "/"), suffix) }
	}
	auditWrites("/accounts", controller.AuditSpec{Module: audit.ModuleAccounting, Entity: "account", Label: "account", Lookup: controller.LookupBy(accountSvc.Get)})
	auditWrites("/journal-books", controller.AuditSpec{Module: audit.ModuleAccounting, Entity: "journal_book", Label: "journal book", Lookup: controller.LookupBy(journalBookSvc.Get)})
	auditWrites("/journal-entries", controller.AuditSpec{Module: audit.ModuleAccounting, Entity: "journal_entry", Label: "journal entry", Lookup: controller.LookupBy(journalSvc.Get)})
	auditWrites("/bank-accounts", controller.AuditSpec{Module: audit.ModuleMasterData, Entity: "bank_account", Label: "bank account", Lookup: controller.LookupBy(bankAccountSvc.Get)})
	auditWrites("/taxes", controller.AuditSpec{Module: audit.ModuleMasterData, Entity: "tax", Label: "tax", Lookup: controller.LookupBy(taxSvc.Get)})
	auditWrites("/units", controller.AuditSpec{Module: audit.ModuleMasterData, Entity: "unit", Label: "unit", Lookup: controller.LookupBy(unitSvc.Get)})
	auditWrites("/document-templates", controller.AuditSpec{Module: audit.ModuleSettings, Entity: "document_template", Label: "default template of"})
	auditWrites("/roles", controller.AuditSpec{Module: audit.ModuleRoleManagement, Entity: "role", Label: "role", Lookup: controller.RoleAuditLookup(roleSvc)})
	auditWrites("/mitra", controller.AuditSpec{
		Module: audit.ModulePartner, Entity: "contact_person", Label: "contact person",
		Match:  func(_, path string) bool { return strings.Contains(path, "/contact-persons") },
		Lookup: controller.ContactAuditLookup(contactPersonSvc),
	})
	// Per-document template choice (the documents' controllers record everything else themselves).
	auditWrites("/sales-orders", controller.AuditSpec{Module: audit.ModuleSalesOrder, Entity: "sales_order", Label: "sales order", Match: endsWith("/template"), Lookup: controller.LookupBy(salesOrderSvc.Get)})
	auditWrites("/sales-invoices", controller.AuditSpec{Module: audit.ModuleSalesInvoice, Entity: "sales_invoice", Label: "sales invoice", Match: endsWith("/template"), Lookup: controller.LookupBy(salesInvoiceSvc.Get)})
	auditWrites("/purchase-orders", controller.AuditSpec{Module: audit.ModulePurchaseOrder, Entity: "purchase_order", Label: "purchase order", Match: endsWith("/template"), Lookup: controller.LookupBy(purchaseOrderSvc.Get)})
	auditWrites("/purchase-invoices", controller.AuditSpec{Module: audit.ModulePurchaseInvoice, Entity: "purchase_invoice", Label: "purchase invoice", Match: endsWith("/template"), Lookup: controller.LookupBy(purchaseInvoiceSvc.Get)})
	// Team: sync-assignments, status changes, invite-multi and accepting an invitation are recorded by
	// the members controller itself; the rest (single invite, role, profile, remove, resend) here.
	auditWrites("/members", controller.AuditSpec{
		Module: audit.ModuleUserManagement, Entity: "member", Label: "user",
		Match: func(method, path string) bool {
			p := strings.TrimRight(path, "/")
			return method == "DELETE" || strings.HasSuffix(p, "/resend") || strings.HasSuffix(p, "/invite") ||
				(method == "PATCH" && !strings.HasSuffix(p, "/status"))
		},
		Lookup: controller.MemberAuditLookup(membershipSvc),
	})
	// contact-summary must be registered before MitraRoutes' "/:id"
	ContactPersonRoutes(business.Group("/mitra"), contactPersonCtrl)
	MitraRoutes(business.Group("/mitra"), mitraCtrl)
	BankAccountRoutes(business, bankAccountCtrl)
	AccountRoutes(business, accountCtrl)
	TaxRoutes(business, taxCtrl)
	UnitRoutes(business, unitCtrl)
	DocumentTemplateRoutes(business, documentTemplateCtrl)
	ConnectedDocumentRoutes(business, connectedDocumentCtrl)
	DocumentConfigurationRoutes(business, documentConfigCtrl)
	JournalBookRoutes(business, journalBookCtrl)
	JournalRoutes(business, journalCtrl)
	ReportRoutes(business, reportCtrl)
	SalesOrderRoutes(business, salesOrderCtrl)
	SalesInvoiceRoutes(business, salesInvoiceCtrl)
	SalesReceiptRoutes(business, salesReceiptCtrl)
	SalesPaymentRoutes(business, salesPaymentCtrl)
	PurchaseOrderRoutes(business, purchaseOrderCtrl)
	PurchaseInvoiceRoutes(business, purchaseInvoiceCtrl)
	PurchaseReceiptRoutes(business, purchaseReceiptCtrl)
	DeliveryNoteRoutes(business, deliveryNoteCtrl)
	GoodsReceiptRoutes(business, goodsReceiptCtrl)
	MemberRoutes(business, memberCtrl)
	AuditRoutes(business, auditCtrl)
	ActivationRoutes(business, activationCtrl)
	RoleWriteRoutes(business, roleCtrl)
	CompanySettingsRoutes(business, companyCtrl)
}
