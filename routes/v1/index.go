package v1

import (
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
	notifier := notification.NewLogService()
	ssoClient := sso.NewClient(config.AppConfig.SSOURL, config.AppConfig.SSOAccountType, config.AppConfig.APIKey)
	rbacCache := service.NewRedisRBACCache(database.Redis)

	// ── repositories ──
	onboardingRepo := repository.NewOnboardingRepository(db)
	membershipRepo := repository.NewMembershipRepository(db)
	mitraRepo := repository.NewMitraRepository(db)
	bankAccountRepo := repository.NewBankAccountRepository(db)
	accountRepo := repository.NewAccountRepository(db)
	taxRepo := repository.NewTaxRepository(db)
	journalBookRepo := repository.NewJournalBookRepository(db)
	journalRepo := repository.NewJournalRepository(db)
	reportRepo := repository.NewReportRepository(db)
	salesOrderRepo := repository.NewSalesOrderRepository(db)
	salesInvoiceRepo := repository.NewSalesInvoiceRepository(db)
	salesReceiptRepo := repository.NewSalesReceiptRepository(db)
	salesPaymentRepo := repository.NewSalesPaymentRepository(db)
	purchaseOrderRepo := repository.NewPurchaseOrderRepository(db)
	purchaseInvoiceRepo := repository.NewPurchaseInvoiceRepository(db)
	purchaseReceiptRepo := repository.NewPurchaseReceiptRepository(db)
	deliveryNoteRepo := repository.NewDeliveryNoteRepository(db)
	goodsReceiptRepo := repository.NewGoodsReceiptRepository(db)

	// ── services ──
	membershipSvc := service.NewMembershipService(
		membershipRepo, onboardingRepo, ssoClient, rbacCache, notifier,
		service.MembershipConfig{
			UseLocalRBAC:          config.AppConfig.UseLocalRBAC,
			RBACMigrationFallback: config.AppConfig.RBACMigrationFallback,
			OwnerRoleID:           config.AppConfig.InvoiceOwnerRoleID,
		},
		config.AppConfig.WebURL,
	)
	roleSvc := service.NewRoleService(ssoClient, membershipRepo, membershipSvc)
	defaultsSvc := service.NewMasterDefaultsService(accountRepo, taxRepo)
	onboardingSvc := service.NewOnboardingService(onboardingRepo, membershipSvc, defaultsSvc)
	companySvc := service.NewCompanyService(onboardingRepo, ssoClient)
	mitraSvc := service.NewMitraService(mitraRepo)
	bankDirSvc := service.NewBankDirectoryService(config.AppConfig.BankMetaURL)
	bankAccountSvc := service.NewBankAccountService(bankAccountRepo)
	accountSvc := service.NewAccountService(accountRepo)
	taxSvc := service.NewTaxService(taxRepo)
	journalBookSvc := service.NewJournalBookService(journalBookRepo)
	journalSvc := service.NewJournalService(journalRepo)
	reportSvc := service.NewReportService(reportRepo)
	salesOrderSvc := service.NewSalesOrderService(salesOrderRepo)
	salesInvoiceSvc := service.NewSalesInvoiceService(salesInvoiceRepo)
	salesReceiptSvc := service.NewSalesReceiptService(salesReceiptRepo)
	salesPaymentSvc := service.NewSalesPaymentService(salesPaymentRepo)
	purchaseOrderSvc := service.NewPurchaseOrderService(purchaseOrderRepo)
	purchaseInvoiceSvc := service.NewPurchaseInvoiceService(purchaseInvoiceRepo)
	purchaseReceiptSvc := service.NewPurchaseReceiptService(purchaseReceiptRepo)
	deliveryNoteSvc := service.NewDeliveryNoteService(deliveryNoteRepo)
	goodsReceiptSvc := service.NewGoodsReceiptService(goodsReceiptRepo)

	// ── controllers ──
	meCtrl := controller.NewMeController(membershipSvc)
	onboardingCtrl := controller.NewOnboardingController(onboardingSvc)
	companyCtrl := controller.NewCompanyController(membershipSvc, onboardingRepo, companySvc)
	memberCtrl := controller.NewMemberController(membershipSvc)
	roleCtrl := controller.NewRoleController(roleSvc)
	mitraCtrl := controller.NewMitraController(mitraSvc)
	metaCtrl := controller.NewMetaController(bankDirSvc)
	bankAccountCtrl := controller.NewBankAccountController(bankAccountSvc)
	accountCtrl := controller.NewAccountController(accountSvc)
	taxCtrl := controller.NewTaxController(taxSvc)
	journalBookCtrl := controller.NewJournalBookController(journalBookSvc)
	journalCtrl := controller.NewJournalController(journalSvc)
	reportCtrl := controller.NewReportController(reportSvc)
	salesOrderCtrl := controller.NewSalesOrderController(salesOrderSvc)
	salesInvoiceCtrl := controller.NewSalesInvoiceController(salesInvoiceSvc)
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

	rbac := rbacRouter(base, membershipSvc)
	RoleReadRoutes(rbac, roleCtrl)

	business := rbac.Group("", middlewares.RequireCompletedOnboarding())
	MitraRoutes(business.Group("/mitra"), mitraCtrl)
	BankAccountRoutes(business, bankAccountCtrl)
	AccountRoutes(business, accountCtrl)
	TaxRoutes(business, taxCtrl)
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
	RoleWriteRoutes(business, roleCtrl)
	CompanySettingsRoutes(business, companyCtrl)
}
