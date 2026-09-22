package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"

	"duluin_invoice/app/model"
	"duluin_invoice/app/repository"
	"duluin_invoice/config"
	"duluin_invoice/database"
	"duluin_invoice/middlewares"
	"duluin_invoice/routes"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system env")
	}

	config.LoadConfig()
	if err := config.ValidateRequired(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	if err := database.ConnectDB(); err != nil {
		log.Fatalf("Database startup failed: %v", err)
	}
	defer database.CloseDB()

	if err := model.AutoMigrateAll(database.DB); err != nil {
		log.Fatalf("Schema migration failed: %v", err)
	}

	if err := repository.BackfillUnitsOnce(database.DB); err != nil {
		log.Printf("⚠️  Unit backfill warning: %v", err)
	}

	if err := repository.RemovePICBackfilledContacts(database.DB); err != nil {
		log.Printf("⚠️  Contact person cleanup warning: %v", err)
	}

	if err := repository.BackfillPurchasePayments(database.DB); err != nil {
		log.Printf("⚠️  Purchase payment backfill warning: %v", err)
	}

	if err := database.ConnectRedis(); err != nil {
		log.Printf("Redis startup warning (token cache disabled): %v", err)
	}
	defer database.CloseRedis()

	app := newFiberApp()
	middlewares.SetupAppMiddleware(app)
	routes.SetupRoutes(app, database.DB)

	runServer(app, config.AppConfig.AppPort)
}

func newFiberApp() *fiber.App {
	return fiber.New(fiber.Config{
		AppName:   config.AppConfig.AppName,
		Immutable: true,
		BodyLimit: 25 * 1024 * 1024,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{"success": false, "message": err.Error()})
		},
	})
}

func runServer(app *fiber.App, port string) {
	if port == "" {
		port = "8090"
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		addr := fmt.Sprintf(":%s", port)
		log.Printf("🚀 %s running on %s (env: %s)", config.AppConfig.AppName, addr, config.AppConfig.AppEnv)
		if err := app.Listen(addr); err != nil {
			log.Fatalf("❌ Server error: %v", err)
		}
	}()

	<-quit
	log.Println("⏳ Shutting down gracefully...")
	_ = app.Shutdown()
	log.Println("✅ Stopped")
}
