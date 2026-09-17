package middlewares

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"duluin_invoice/config"
	"duluin_invoice/database"
	"duluin_invoice/utils"
)

const (
	tokenCacheTTL       = 5 * time.Minute
	tokenCacheKeyPrefix = "invoice:token:"
)

// CachedTokenData is the subset of the SSO user we keep in Redis / fiber locals.
type CachedTokenData struct {
	CompanyID   string   `json:"company_id"`
	UserID      string   `json:"user_id"`
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	IsActivated bool     `json:"is_activated"`
	IsBanned    bool     `json:"is_banned"`
}

type ssoSigninCookiesResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	User    struct {
		ID          string      `json:"id"`
		Name        string      `json:"name"`
		Email       string      `json:"email"`
		SecondaryID string      `json:"secondary_id"`
		Accounts    interface{} `json:"accounts"`
		Roles       interface{} `json:"roles"`
		Permissions interface{} `json:"permissions"`
		Banned      interface{} `json:"banned"`
		IsActive    interface{} `json:"is_active"`
		IsActivated interface{} `json:"is_activated"`
	} `json:"user"`
}

// ValidateToken authenticates the request against SSO for this product's account type.
func ValidateToken() fiber.Handler {
	return ValidateTokenForAccount(config.AppConfig.SSOAccountType)
}

// ValidateTokenForAccount validates the bearer token against a specific SSO account type.
func ValidateTokenForAccount(accountType string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		requestID := GetRequestID(c)

		authHeader := strings.Clone(c.Get("Authorization"))
		if authHeader == "" {
			if token := c.Cookies("app_token"); token != "" {
				authHeader = "Bearer " + strings.Clone(token)
			} else if token := c.Cookies("APP_TOKEN"); token != "" {
				authHeader = "Bearer " + strings.Clone(token)
			}
		}
		if authHeader == "" {
			return unauthorized(c, "Unauthenticated")
		}
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return unauthorized(c, "Invalid authorization format. Use: Bearer <token>")
		}

		activeCompanyID := ResolveActiveCompanyID(c)

		cacheKey := tokenCacheKeyPrefix + accountType + ":" + authHeader
		if activeCompanyID != "" {
			cacheKey += ":" + activeCompanyID
		}

		tokenData, found := getCachedToken(cacheKey)
		if !found {
			var err error
			tokenData, err = validateWithSSO(authHeader, activeCompanyID, accountType, requestID)
			if err != nil {
				return unauthorized(c, err.Error())
			}
			setCachedToken(cacheKey, tokenData)
		}

		if activeCompanyID != "" {
			tokenData.CompanyID = activeCompanyID
		}
		if tokenData.IsBanned {
			return utils.Forbidden(c, []string{"Account is banned"})
		}

		setLocals(c, tokenData)
		c.Locals("accountType", accountType)
		return c.Next()
	}
}

func validateWithSSO(authHeader, activeCompanyID, accountType, requestID string) (*CachedTokenData, error) {
	validateURL := fmt.Sprintf("%s/users/signin-cookies", strings.TrimSuffix(config.AppConfig.SSOURL, "/"))

	req, err := http.NewRequest(http.MethodGet, validateURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build SSO request: %v", err)
	}
	req.Header.Set("Authorization", strings.Clone(authHeader))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Account-Type", accountType)
	if activeCompanyID != "" {
		req.Header.Set("x-callback-token", activeCompanyID)
	}

	resp, err := utils.NewOutboundHTTPClient(10 * time.Second).Do(req)
	if err != nil {
		log.Printf("[ValidateToken] request_id=%s sso_request_failed err=%v", requestID, err)
		return nil, fmt.Errorf("failed to validate token")
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Printf("[ValidateToken] request_id=%s sso_non_200 status=%d body=%s", requestID, resp.StatusCode, string(body))
		return nil, fmt.Errorf("session expired or unauthorized")
	}

	var parsed ssoSigninCookiesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		log.Printf("[ValidateToken] request_id=%s sso_bad_json err=%v", requestID, err)
		return nil, fmt.Errorf("failed to parse SSO response")
	}
	if !parsed.Success {
		return nil, fmt.Errorf("token validation failed: %s", parsed.Message)
	}

	isActivated := toBool(parsed.User.IsActive) || toBool(parsed.User.IsActivated)
	isBanned := toBool(parsed.User.Banned)

	// NOTE: SSO user_accounts.secondary_id holds the user's own id (identity
	// link), NOT a company — the active company is always resolved from the
	// X-Company-ID / x-callback-token header (see ResolveActiveCompanyID).
	if accounts, ok := parsed.User.Accounts.(map[string]interface{}); ok {
		if entry, ok := accounts[accountType].(map[string]interface{}); ok {
			if v, ok := entry["is_active"]; ok {
				isActivated = toBool(v)
			}
			if v, ok := entry["is_banned"]; ok {
				isBanned = toBool(v)
			}
		}
	}

	return &CachedTokenData{
		UserID:      strings.TrimSpace(parsed.User.ID),
		Name:        parsed.User.Name,
		Email:       parsed.User.Email,
		Roles:       toStringSlice(parsed.User.Roles),
		Permissions: toStringSlice(parsed.User.Permissions),
		IsActivated: isActivated,
		IsBanned:    isBanned,
	}, nil
}

// ── cache ───────────────────────────────────────────────────────────────────

func getCachedToken(key string) (*CachedTokenData, bool) {
	if database.Redis == nil {
		return nil, false
	}
	raw, err := database.Redis.Get(context.Background(), key).Result()
	if err != nil || raw == "" {
		return nil, false
	}
	var data CachedTokenData
	if json.Unmarshal([]byte(raw), &data) != nil {
		return nil, false
	}
	return &data, true
}

func setCachedToken(key string, data *CachedTokenData) {
	if database.Redis == nil || data == nil {
		return
	}
	if payload, err := json.Marshal(data); err == nil {
		database.Redis.Set(context.Background(), key, payload, tokenCacheTTL)
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────

func setLocals(c *fiber.Ctx, data *CachedTokenData) {
	userID := strings.TrimSpace(data.UserID)
	if userID == "" {
		userID = ResolveSSOUserID(c)
	}
	c.Locals("companyID", data.CompanyID)
	c.Locals("userID", userID)
	c.Locals("name", data.Name)
	c.Locals("email", data.Email)
	c.Locals("roles", data.Roles)
	c.Locals("permissions", data.Permissions)
	c.Locals("isActivated", data.IsActivated)
	c.Locals("isBanned", data.IsBanned)
}

func unauthorized(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": message})
}

func toBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case float64:
		return val == 1
	case string:
		return val == "1" || val == "true"
	default:
		return false
	}
}

func toStringSlice(v interface{}) []string {
	switch val := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(val))
		for _, item := range val {
			switch it := item.(type) {
			case string:
				out = append(out, it)
			case map[string]interface{}:
				if name, ok := it["name"].(string); ok {
					out = append(out, name)
				}
			}
		}
		return out
	case []string:
		return val
	default:
		return []string{}
	}
}
