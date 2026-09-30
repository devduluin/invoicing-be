package main

import (
	"bytes"
	"encoding/json"
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

	if err := repository.BackfillMitraCodesOnce(database.DB); err != nil {
		log.Printf("⚠️  Partner code backfill warning: %v", err)
	}

	if err := repository.BackfillSalespersonsOnce(database.DB); err != nil {
		log.Printf("⚠️  Salesperson backfill warning: %v", err)
	}

	if err := repository.BackfillUnitsOnce(database.DB); err != nil {
		log.Printf("⚠️  Unit backfill warning: %v", err)
	}

	if err := repository.BackfillTaxIndonesianNamesOnce(database.DB); err != nil {
		log.Printf("⚠️  Tax rename backfill warning: %v", err)
	}

	if err := repository.BackfillAppliedDownPaymentsOnce(database.DB); err != nil {
		log.Printf("⚠️  Applied down-payment backfill warning: %v", err)
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
		// The standard library's json.Marshal (Fiber's default JSONEncoder) HTML-escapes & < >
		// inside string values (e.g. a URL with query params comes out as ...&...). Every
		// consumer that actually parses the JSON (any HTTP client, the frontend's fetch().json())
		// gets the right character either way, but anyone reading the raw response body directly —
		// logs, curl, Postman, a copy-pasted invite_url — sees the escaped form and a broken link.
		// This response is never embedded in HTML, so there's nothing to guard against here.
		JSONEncoder: marshalJSONWithoutHTMLEscaping,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{"success": false, "message": err.Error()})
		},
	})
}

func marshalJSONWithoutHTMLEscaping(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// json.Encoder.Encode appends a trailing newline that json.Marshal does not; trim it so this
	// is a drop-in replacement for the encoder Fiber otherwise uses.
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
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
