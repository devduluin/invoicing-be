package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppName string
	AppEnv  string
	AppPort string

	DB_Driver   string
	DB_URL      string
	DB_Host     string
	DB_Port     string
	DB_User     string
	DB_Password string
	DB_Name     string

	Redis_Host     string
	Redis_Port     string
	Redis_Password string
	Redis_DB       int

	// SSO / auth
	SSOAccountType string
	SSOURL         string
	LaunchpadURL   string
	APIKey         string // SSO API_KEY — bearer fallback for internal SSO calls

	// Public URL of the invoice-frontend — used to build invite accept links.
	WebURL string

	// Reference-data: the Duluin bank directory (Indonesian banks + codes).
	BankMetaURL string

	// Local RBAC (acc-master pattern). ON by default — invoice is greenfield.
	UseLocalRBAC          bool
	RBACMigrationFallback bool
	InvoiceOwnerRoleID    string // optional pin for the "Invoice Owner" SSO role uuid

	CORSAllowedOrigins string
}

var AppConfig Config

func LoadConfig() {
	driver := os.Getenv("DATABASE_DRIVER")

	AppConfig = Config{
		AppName: getEnvDefault("APP_NAME", "invoice-service"),
		AppEnv:  getEnvDefault("APP_ENV", "development"),
		AppPort: getEnvDefault("APP_PORT", "8090"),

		DB_Driver:   driver,
		DB_Host:     os.Getenv("DATABASE_HOST"),
		DB_Port:     os.Getenv("DATABASE_PORT"),
		DB_User:     os.Getenv("DATABASE_USER"),
		DB_Password: os.Getenv("DATABASE_PASSWORD"),
		DB_Name:     os.Getenv("DATABASE_NAME"),

		Redis_Host:     os.Getenv("REDIS_HOST"),
		Redis_Port:     os.Getenv("REDIS_PORT"),
		Redis_Password: os.Getenv("REDIS_PASSWORD"),
		Redis_DB:       parseIntDefault(os.Getenv("REDIS_DB"), 0),

		SSOAccountType: getEnvDefault("SSO_ACCOUNT_TYPE", "duluin_invoice"),
		SSOURL:         strings.TrimRight(os.Getenv("SSO_URL"), "/"),
		LaunchpadURL:   strings.TrimRight(os.Getenv("LAUNCHPAD_URL"), "/"),
		APIKey:         os.Getenv("API_KEY"),
		WebURL:         strings.TrimRight(getEnvDefault("INVOICE_WEB_URL", "http://localhost:3010"), "/"),
		BankMetaURL:    strings.TrimRight(getEnvDefault("BANK_META_URL", "https://dev-dashboard.duluin.com/api/meta/bank"), "/"),

		UseLocalRBAC:          parseBoolDefault(os.Getenv("USE_LOCAL_RBAC"), true),
		RBACMigrationFallback: parseBoolDefault(os.Getenv("RBAC_MIGRATION_FALLBACK"), true),
		InvoiceOwnerRoleID:    strings.TrimSpace(os.Getenv("INVOICE_OWNER_ROLE_ID")),

		CORSAllowedOrigins: getEnvDefault("CORS_ALLOWED_ORIGINS", "*"),
	}

	switch driver {
	case "postgres", "postgresql":
		AppConfig.DB_URL = fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
			AppConfig.DB_Host, AppConfig.DB_User, AppConfig.DB_Password, AppConfig.DB_Name, AppConfig.DB_Port,
		)
	default:
		AppConfig.DB_URL = ""
	}
}

func ValidateRequired() error {
	var missing []string
	required := map[string]string{
		"DATABASE_DRIVER": AppConfig.DB_Driver,
		"DATABASE_HOST":   AppConfig.DB_Host,
		"DATABASE_PORT":   AppConfig.DB_Port,
		"DATABASE_USER":   AppConfig.DB_User,
		"DATABASE_NAME":   AppConfig.DB_Name,
		"SSO_URL":         AppConfig.SSOURL,
	}
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}
	if AppConfig.DB_URL == "" {
		return fmt.Errorf("unsupported DATABASE_DRIVER: %s (only postgres)", AppConfig.DB_Driver)
	}
	return nil
}

func getEnvDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func parseIntDefault(value string, fallback int) int {
	if v, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
		return v
	}
	return fallback
}

func parseBoolDefault(value string, fallback bool) bool {
	v := strings.TrimSpace(value)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
