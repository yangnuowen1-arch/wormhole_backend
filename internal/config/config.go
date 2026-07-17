package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config 保存应用运行所需的全部配置，均从环境变量读取。
type Config struct {
	AppPort string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// JWTSecret 用于签发本应用的会话令牌；它不是 Keycloak 的签名密钥。
	JWTSecret    string
	JWTExpireHrs int

	// KeycloakEnabled 开启后，用户名直登接口将不再注册，认证改由 Keycloak 完成。
	KeycloakEnabled         bool
	KeycloakIssuerURL       string
	KeycloakClientID        string
	KeycloakClientSecret    string
	KeycloakRedirectURL     string
	KeycloakFrontendURL     string
	KeycloakScopes          []string
	KeycloakStateSecret     string
	KeycloakStateCookieName string
	// KeycloakIDTokenCookieName 保存已验证的 ID Token，仅用于发起 RP-Initiated Logout。
	// 它与本应用的 AuthCookie 分离，且始终以 HttpOnly Cookie 方式保存。
	KeycloakIDTokenCookieName string
	KeycloakHTTPTimeout       time.Duration

	// AuthCookie* 控制后端建立的本应用会话 Cookie。
	AuthCookieName     string
	AuthCookieSecure   bool
	AuthCookieSameSite string

	// CORSAllowedOrigins 是允许携带会话 Cookie 调用 API 的前端 Origin 白名单。
	CORSAllowedOrigins []string
}

// LoadConfig 从 .env（若存在）与环境变量加载配置。
func LoadConfig() *Config {
	// .env 可选，加载失败忽略（生产环境通常直接注入环境变量）。
	_ = godotenv.Load()

	frontendURL := strings.TrimRight(getEnv("KEYCLOAK_FRONTEND_URL", "http://localhost:5173"), "/")

	return &Config{
		AppPort: getEnv("APP_PORT", "8080"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "wormhole"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		JWTSecret:    getEnv("JWT_SECRET", "change-me-in-production"),
		JWTExpireHrs: getEnvInt("JWT_EXPIRE_HOURS", 72),

		KeycloakEnabled:           getEnvBool("KEYCLOAK_ENABLED", false),
		KeycloakIssuerURL:         strings.TrimRight(getEnv("KEYCLOAK_ISSUER_URL", ""), "/"),
		KeycloakClientID:          getEnv("KEYCLOAK_CLIENT_ID", ""),
		KeycloakClientSecret:      getEnv("KEYCLOAK_CLIENT_SECRET", ""),
		KeycloakRedirectURL:       getEnv("KEYCLOAK_REDIRECT_URL", ""),
		KeycloakFrontendURL:       frontendURL,
		KeycloakScopes:            getEnvCSV("KEYCLOAK_SCOPES", "openid,profile,email"),
		KeycloakStateSecret:       getEnv("KEYCLOAK_STATE_SECRET", ""),
		KeycloakStateCookieName:   getEnv("KEYCLOAK_STATE_COOKIE_NAME", "wormhole_oidc_state"),
		KeycloakIDTokenCookieName: getEnv("KEYCLOAK_ID_TOKEN_COOKIE_NAME", "wormhole_oidc_id_token"),
		KeycloakHTTPTimeout:       time.Duration(getEnvInt("KEYCLOAK_HTTP_TIMEOUT_SECONDS", 10)) * time.Second,

		AuthCookieName:     getEnv("AUTH_COOKIE_NAME", "wormhole_session"),
		AuthCookieSecure:   getEnvBool("AUTH_COOKIE_SECURE", false),
		AuthCookieSameSite: strings.ToLower(getEnv("AUTH_COOKIE_SAME_SITE", "lax")),

		CORSAllowedOrigins: getEnvCSV("CORS_ALLOWED_ORIGINS", frontendURL),
	}
}

// ValidateKeycloak 校验 Keycloak SSO 模式启动所需的安全配置。
func (c *Config) ValidateKeycloak() error {
	if !c.KeycloakEnabled {
		return nil
	}

	missing := make([]string, 0, 5)
	for key, value := range map[string]string{
		"KEYCLOAK_ISSUER_URL":    c.KeycloakIssuerURL,
		"KEYCLOAK_CLIENT_ID":     c.KeycloakClientID,
		"KEYCLOAK_CLIENT_SECRET": c.KeycloakClientSecret,
		"KEYCLOAK_REDIRECT_URL":  c.KeycloakRedirectURL,
		"KEYCLOAK_STATE_SECRET":  c.KeycloakStateSecret,
	} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("Keycloak SSO 配置缺失: %s", strings.Join(missing, ", "))
	}
	if len(c.KeycloakStateSecret) < 32 {
		return errors.New("KEYCLOAK_STATE_SECRET 至少应为 32 个字节")
	}
	if c.JWTSecret == "" || c.JWTSecret == "change-me-in-production" || len(c.JWTSecret) < 32 {
		return errors.New("启用 Keycloak SSO 时 JWT_SECRET 必须是至少 32 个字节的随机密钥")
	}
	if c.JWTExpireHrs <= 0 {
		return errors.New("JWT_EXPIRE_HOURS 必须大于 0")
	}
	if c.KeycloakHTTPTimeout <= 0 {
		return errors.New("KEYCLOAK_HTTP_TIMEOUT_SECONDS 必须大于 0")
	}
	if len(c.KeycloakScopes) == 0 || c.KeycloakScopes[0] == "" {
		return errors.New("KEYCLOAK_SCOPES 不能为空，且必须包含 openid")
	}
	if !contains(c.KeycloakScopes, "openid") {
		return errors.New("KEYCLOAK_SCOPES 必须包含 openid")
	}
	if err := validateHTTPURL("KEYCLOAK_ISSUER_URL", c.KeycloakIssuerURL); err != nil {
		return err
	}
	if err := validateHTTPURL("KEYCLOAK_REDIRECT_URL", c.KeycloakRedirectURL); err != nil {
		return err
	}
	if err := validateHTTPURL("KEYCLOAK_FRONTEND_URL", c.KeycloakFrontendURL); err != nil {
		return err
	}
	if c.AuthCookieSameSite != "lax" && c.AuthCookieSameSite != "strict" && c.AuthCookieSameSite != "none" {
		return fmt.Errorf("AUTH_COOKIE_SAME_SITE 必须是 lax、strict 或 none，当前为 %q", c.AuthCookieSameSite)
	}
	if c.AuthCookieSameSite == "none" && !c.AuthCookieSecure {
		return errors.New("AUTH_COOKIE_SAME_SITE=none 时 AUTH_COOKIE_SECURE 必须为 true")
	}
	for _, origin := range c.CORSAllowedOrigins {
		if err := validateHTTPURL("CORS_ALLOWED_ORIGINS", origin); err != nil {
			return err
		}
	}
	return nil
}

func validateHTTPURL(key, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("%s 必须是有效的 http(s) URL", key)
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvCSV(key, fallback string) []string {
	value := getEnv(key, fallback)
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
