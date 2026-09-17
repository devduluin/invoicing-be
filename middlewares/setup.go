package middlewares

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"

	"duluin_invoice/config"
)

// SetupAppMiddleware wires the base HTTP middleware stack.
func SetupAppMiddleware(app *fiber.App) {
	app.Use(requestid.New())
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} ${method} ${path} (${latency})\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins:     resolveAllowOrigins(),
		AllowMethods:     "GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Account-Type,X-Company-ID,x-callback-token,X-Request-ID",
		AllowCredentials: config.AppConfig.CORSAllowedOrigins != "*",
	}))
}

func resolveAllowOrigins() string {
	raw := strings.TrimSpace(config.AppConfig.CORSAllowedOrigins)
	if raw == "" {
		return "*"
	}
	return raw
}
