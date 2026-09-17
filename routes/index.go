package routes

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"duluin_invoice/config"
	v1 "duluin_invoice/routes/v1"
)

func HealthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"success": true,
		"service": config.AppConfig.AppName,
		"status":  "ok",
	})
}

func SetupRoutes(app *fiber.App, db *gorm.DB) {
	app.Get("/health", HealthCheck)

	apiV1 := app.Group("/api/v1")
	v1.RegisterRoutes(apiV1, db)

	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Route not found",
		})
	})
}
