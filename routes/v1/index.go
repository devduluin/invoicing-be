package v1

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"duluin_invoice/app/controller"
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
	contactPersonCtrl := controller.NewContactPersonController(service.NewContactPersonService(repository.NewContactPersonRepository(db)))
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
	onboardingCtrl := controller.NewOnboardingController(onboardingSvc)
	companyCtrl := controller.NewCompanyController(membershipSvc, onboardingRepo, companySvc, auditSvc)
	memberCtrl := controller.NewMemberController(membershipSvc, auditSvc)
	auditCtrl := controller.NewAuditController(auditSvc)
	roleCtrl := controller.NewRoleController(roleSvc)
	mitraCtrl := controller.NewMitraController(mitraSvc)
	metaCtrl := controller.NewMetaController(bankDirSvc)
	bankAccountCtrl := controller.NewBankAccountController(bankAccountSvc)
	accountCtrl := controller.NewAccountController(accountSvc)
	taxCtrl := controller.NewTaxController(taxSvc)
	unitCtrl := controller.NewUnitController(unitSvc)
	journalBookCtrl := controller.NewJournalBookController(journalBookSvc)
	journalCtrl := controller.NewJournalController(journalSvc)
	reportCtrl := controller.NewReportController(reportSvc)
	salesOrderCtrl := controller.NewSalesOrderController(salesOrderSvc)
	salesInvoiceCtrl := controller.NewSalesInvoiceController(salesInvoiceSvc, auditSvc)
	documentTemplateCtrl := controller.NewDocumentTemplateController(documentTemplateSvc)
	salesReceiptCtrl := controller.NewSalesReceiptController(salesReceiptSvc)
	salesPaymentCtrl := controller.NewSalesPaymentController(salesPaymentSvc)
	purchaseOrderCtrl := controller.NewPurchaseOrderController(purchaseOrderSvc)
	purchaseInvoiceCtrl := controller.NewPurchaseInvoiceController(purchaseInvoiceSvc)
	purchaseReceiptCtrl := controller.NewPurchaseReceiptController(purchaseReceiptSvc)
	deliveryNoteCtrl := controller.NewDeliveryNoteController(deliveryNoteSvc)
	goodsReceiptCtrl := controller.NewGoodsReceiptController(goodsReceiptSvc)

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
