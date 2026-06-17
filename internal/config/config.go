package config

import (
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

const (
	defaultDatabaseURL        = "postgres://localhost:5432/vola?sslmode=disable"
	developmentJWTSecret      = "dev-jwt-secret-change-in-production"
	localSQLiteJWTSecret      = "vola-local-sqlite-jwt-secret"
	developmentVaultMasterKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	developmentPostgresPass   = "vola_dev"
)

type Config struct {
	DatabaseURL                           string
	Port                                  string
	JWTSecret                             string
	UserStorageQuotaBytes                 int64
	ObjectStorageBackend                  string
	TencentCOSBucket                      string
	TencentCOSRegion                      string
	TencentCOSEndpoint                    string
	TencentCOSSecretID                    string
	TencentCOSSecretKey                   string
	TencentCOSPrefix                      string
	TencentCOSPathStyle                   bool
	GithubClientID                        string
	GithubClientSecret                    string
	PocketProviderID                      string
	PocketIssuer                          string
	PocketDiscoveryURL                    string
	PocketClientID                        string
	PocketClientSecret                    string
	PocketScopes                          []string
	GitHubAppClientID                     string
	GitHubAppClientSecret                 string
	GitHubAppSlug                         string
	GitMirrorHostedRoot                   string
	GitMirrorManualSyncCooldownSeconds    int
	GitMirrorManualSyncCooldownConfigured bool
	FeishuAppID                           string
	FeishuAppSecret                       string
	FeishuVerificationToken               string
	FeishuEncryptKey                      string
	VaultMasterKey                        string
	PublicBaseURL                         string
	CORSOrigins                           []string
	RateLimit                             int   // max requests per minute
	MaxBodySize                           int64 // max request body in bytes
	LogLevel                              string
	LogFormat                             string
	EnableSystemSettings                  bool
	EnableBilling                         bool
	EnablePublicRegistration              bool
	CaptureOAuth                          bool
	CaptureDir                            string
	InstanceAdminUserIDs                  []uuid.UUID
}

func Load() (*Config, error) {
	return LoadWithOverrides(nil)
}

func LoadWithOverrides(overrides map[string]string) (*Config, error) {
	envOrOverride := func(key, fallback string) string {
		if overrides != nil {
			if value, ok := overrides[key]; ok {
				return value
			}
		}
		return getEnv(key, fallback)
	}

	tencentCOSPathStyle, err := parseBoolConfig("TENCENT_COS_PATH_STYLE", envOrOverride("TENCENT_COS_PATH_STYLE", "0"), false)
	if err != nil {
		return nil, err
	}
	rateLimit, err := parsePositiveIntConfig("RATE_LIMIT", envOrOverride("RATE_LIMIT", ""), 100)
	if err != nil {
		return nil, err
	}
	maxBodySize, err := parsePositiveByteSizeConfig("MAX_BODY_SIZE", envOrOverride("MAX_BODY_SIZE", ""), 10*1024*1024)
	if err != nil {
		return nil, err
	}
	enableSystemSettings, err := parseBoolConfig("VOLA_ENABLE_SYSTEM_SETTINGS", envOrOverride("VOLA_ENABLE_SYSTEM_SETTINGS", ""), true)
	if err != nil {
		return nil, err
	}
	enableBilling, err := parseBoolConfig("VOLA_ENABLE_BILLING", envOrOverride("VOLA_ENABLE_BILLING", ""), false)
	if err != nil {
		return nil, err
	}
	enablePublicRegistration, err := parseBoolConfig("VOLA_ENABLE_PUBLIC_REGISTRATION", envOrOverride("VOLA_ENABLE_PUBLIC_REGISTRATION", ""), !isPublicBaseURL(envOrOverride("PUBLIC_BASE_URL", "")))
	if err != nil {
		return nil, err
	}
	captureOAuth, err := parseBoolConfig("VOLA_CAPTURE_OAUTH", envOrOverride("VOLA_CAPTURE_OAUTH", ""), false)
	if err != nil {
		return nil, err
	}
	instanceAdminUserIDs, err := parseUUIDListConfig("INSTANCE_ADMIN_USER_IDS", envOrOverride("INSTANCE_ADMIN_USER_IDS", ""))
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		DatabaseURL:              envOrOverride("DATABASE_URL", defaultDatabaseURL),
		Port:                     envOrOverride("PORT", "8080"),
		JWTSecret:                envOrOverride("JWT_SECRET", ""),
		ObjectStorageBackend:     strings.ToLower(strings.TrimSpace(envOrOverride("OBJECT_STORAGE_BACKEND", "db"))),
		TencentCOSBucket:         strings.TrimSpace(envOrOverride("TENCENT_COS_BUCKET", "")),
		TencentCOSRegion:         strings.TrimSpace(envOrOverride("TENCENT_COS_REGION", "ap-guangzhou")),
		TencentCOSEndpoint:       strings.TrimSpace(envOrOverride("TENCENT_COS_ENDPOINT", "")),
		TencentCOSSecretID:       strings.TrimSpace(envOrOverride("TENCENT_COS_SECRET_ID", "")),
		TencentCOSSecretKey:      strings.TrimSpace(envOrOverride("TENCENT_COS_SECRET_KEY", "")),
		TencentCOSPrefix:         strings.TrimSpace(envOrOverride("TENCENT_COS_PREFIX", "vola")),
		TencentCOSPathStyle:      tencentCOSPathStyle,
		GithubClientID:           envOrOverride("GITHUB_CLIENT_ID", ""),
		GithubClientSecret:       envOrOverride("GITHUB_CLIENT_SECRET", ""),
		PocketProviderID:         envOrOverride("POCKET_ID_PROVIDER_ID", "pocket"),
		PocketIssuer:             strings.TrimRight(envOrOverride("POCKET_ID_ISSUER", ""), "/"),
		PocketDiscoveryURL:       strings.TrimSpace(envOrOverride("POCKET_ID_DISCOVERY_URL", "")),
		PocketClientID:           envOrOverride("POCKET_ID_CLIENT_ID", ""),
		PocketClientSecret:       envOrOverride("POCKET_ID_CLIENT_SECRET", ""),
		PocketScopes:             splitScopes(envOrOverride("POCKET_ID_SCOPES", "openid profile email")),
		GitHubAppClientID:        envOrOverride("GITHUB_APP_CLIENT_ID", ""),
		GitHubAppClientSecret:    envOrOverride("GITHUB_APP_CLIENT_SECRET", ""),
		GitHubAppSlug:            envOrOverride("GITHUB_APP_SLUG", ""),
		GitMirrorHostedRoot:      envOrOverride("GIT_MIRROR_HOSTED_ROOT", ""),
		FeishuAppID:              envOrOverride("FEISHU_APP_ID", ""),
		FeishuAppSecret:          envOrOverride("FEISHU_APP_SECRET", ""),
		FeishuVerificationToken:  envOrOverride("FEISHU_VERIFICATION_TOKEN", ""),
		FeishuEncryptKey:         envOrOverride("FEISHU_ENCRYPT_KEY", ""),
		VaultMasterKey:           envOrOverride("VAULT_MASTER_KEY", ""),
		PublicBaseURL:            strings.TrimRight(envOrOverride("PUBLIC_BASE_URL", ""), "/"),
		RateLimit:                rateLimit,
		MaxBodySize:              maxBodySize,
		LogLevel:                 envOrOverride("LOG_LEVEL", "info"),
		LogFormat:                envOrOverride("LOG_FORMAT", "text"),
		EnableSystemSettings:     enableSystemSettings,
		EnableBilling:            enableBilling,
		EnablePublicRegistration: enablePublicRegistration,
		CaptureOAuth:             captureOAuth,
		CaptureDir:               envOrOverride("VOLA_CAPTURE_DIR", "tmp/oauth-captures"),
		InstanceAdminUserIDs:     instanceAdminUserIDs,
	}
	corsOrigins, err := parseCORSOrigins(envOrOverride("CORS_ORIGINS", "http://localhost:3000"))
	if err != nil {
		return nil, err
	}
	cfg.CORSOrigins = corsOrigins
	if rawCooldown := strings.TrimSpace(envOrOverride("GIT_MIRROR_MANUAL_SYNC_COOLDOWN_SECONDS", "")); rawCooldown != "" {
		cooldown, err := strconv.Atoi(rawCooldown)
		if err != nil || cooldown < 0 {
			return nil, fmt.Errorf("invalid GIT_MIRROR_MANUAL_SYNC_COOLDOWN_SECONDS: must be a non-negative integer")
		}
		cfg.GitMirrorManualSyncCooldownSeconds = cooldown
		cfg.GitMirrorManualSyncCooldownConfigured = true
	}

	quotaBytes, err := parseByteSize(envOrOverride("USER_STORAGE_QUOTA_BYTES", "100MB"))
	if err != nil {
		return nil, fmt.Errorf("invalid USER_STORAGE_QUOTA_BYTES: %w", err)
	}
	cfg.UserStorageQuotaBytes = quotaBytes

	switch cfg.ObjectStorageBackend {
	case "", "db":
		cfg.ObjectStorageBackend = "db"
	case "cos":
		if cfg.TencentCOSBucket == "" {
			return nil, fmt.Errorf("TENCENT_COS_BUCKET is required when OBJECT_STORAGE_BACKEND=cos")
		}
		if cfg.TencentCOSSecretID == "" {
			return nil, fmt.Errorf("TENCENT_COS_SECRET_ID is required when OBJECT_STORAGE_BACKEND=cos")
		}
		if cfg.TencentCOSSecretKey == "" {
			return nil, fmt.Errorf("TENCENT_COS_SECRET_KEY is required when OBJECT_STORAGE_BACKEND=cos")
		}
		if cfg.TencentCOSRegion == "" && cfg.TencentCOSEndpoint == "" {
			return nil, fmt.Errorf("TENCENT_COS_REGION or TENCENT_COS_ENDPOINT is required when OBJECT_STORAGE_BACKEND=cos")
		}
	default:
		return nil, fmt.Errorf("unsupported OBJECT_STORAGE_BACKEND %q", cfg.ObjectStorageBackend)
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET environment variable is required")
	}

	if cfg.VaultMasterKey == "" {
		return nil, fmt.Errorf("VAULT_MASTER_KEY environment variable is required")
	}

	if err := validatePublicDeploymentSecrets(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func parsePositiveIntConfig(key, value string, fallback int) (int, error) {
	s := strings.TrimSpace(value)
	if s == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: must be a positive integer", key)
	}
	if v <= 0 {
		return 0, fmt.Errorf("invalid %s: must be greater than 0", key)
	}
	return v, nil
}

func parsePositiveByteSizeConfig(key, value string, fallback int64) (int64, error) {
	s := strings.TrimSpace(value)
	if s == "" {
		return fallback, nil
	}
	size, err := parseByteSize(s)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	if size <= 0 {
		return 0, fmt.Errorf("invalid %s: must be greater than 0", key)
	}
	return size, nil
}

func parseBoolConfig(key, value string, fallback bool) (bool, error) {
	s := strings.TrimSpace(strings.ToLower(value))
	if s == "" {
		return fallback, nil
	}
	switch s {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid %s: must be one of 1, true, yes, on, 0, false, no, off", key)
	}
}

func parseUUIDListConfig(key, value string) ([]uuid.UUID, error) {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\t' || r == ' '
	})
	ids := make([]uuid.UUID, 0, len(parts))
	seen := make(map[uuid.UUID]struct{}, len(parts))
	for _, raw := range parts {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		id, err := uuid.Parse(item)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %q is not a UUID", key, item)
		}
		if id == uuid.Nil {
			return nil, fmt.Errorf("invalid %s: nil UUID is not allowed", key)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func splitScopes(value string) []string {
	parts := strings.Fields(strings.TrimSpace(value))
	if len(parts) == 0 {
		return []string{}
	}
	return parts
}

func parseCORSOrigins(value string) ([]string, error) {
	seen := make(map[string]struct{})
	origins := make([]string, 0)
	for _, raw := range strings.Split(value, ",") {
		origin := strings.TrimSpace(raw)
		if origin == "" {
			continue
		}
		if origin == "*" {
			return nil, fmt.Errorf("CORS_ORIGINS cannot contain * when credentialed requests are enabled")
		}
		normalized, err := normalizeCORSOrigin(origin)
		if err != nil {
			return nil, err
		}
		origin = normalized
		if _, ok := seen[origin]; ok {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}
	return origins, nil
}

func normalizeCORSOrigin(origin string) (string, error) {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Hostname() == "" {
		return "", fmt.Errorf("invalid CORS_ORIGINS origin %q: expected scheme://host[:port]", origin)
	}
	if parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("invalid CORS_ORIGINS origin %q: origin must not include user info, path, query, or fragment", origin)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = ""
	switch parsed.Scheme {
	case "http", "https", "tauri":
		return parsed.String(), nil
	default:
		return "", fmt.Errorf("invalid CORS_ORIGINS origin %q: unsupported scheme %q", origin, parsed.Scheme)
	}
}

func validatePublicDeploymentSecrets(cfg *Config) error {
	if !isPublicBaseURL(cfg.PublicBaseURL) {
		return nil
	}
	switch cfg.JWTSecret {
	case developmentJWTSecret, localSQLiteJWTSecret:
		return fmt.Errorf("PUBLIC_BASE_URL points to a non-local host; JWT_SECRET must not use a development default")
	}
	if cfg.VaultMasterKey == developmentVaultMasterKey {
		return fmt.Errorf("PUBLIC_BASE_URL points to a non-local host; VAULT_MASTER_KEY must not use the development default")
	}
	if strings.TrimSpace(cfg.DatabaseURL) == defaultDatabaseURL {
		return fmt.Errorf("PUBLIC_BASE_URL points to a non-local host; DATABASE_URL must not use the development default")
	}
	parsed, err := url.Parse(cfg.DatabaseURL)
	if err == nil && parsed.User != nil {
		if password, ok := parsed.User.Password(); ok && password == developmentPostgresPass {
			return fmt.Errorf("PUBLIC_BASE_URL points to a non-local host; DATABASE_URL must not use the development Postgres password")
		}
	}
	return nil
}

func isPublicBaseURL(value string) bool {
	baseURL := strings.TrimSpace(value)
	if baseURL == "" {
		return false
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Hostname() == "" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return !ip.IsLoopback()
	}
	return true
}

func parseByteSize(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}

	upper := strings.ToUpper(value)
	split := len(upper)
	for i, r := range upper {
		if r < '0' || r > '9' {
			split = i
			break
		}
	}

	numberPart := strings.TrimSpace(upper[:split])
	unitPart := strings.TrimSpace(upper[split:])
	if numberPart == "" {
		return 0, fmt.Errorf("missing numeric value")
	}

	number, err := strconv.ParseInt(numberPart, 10, 64)
	if err != nil {
		return 0, err
	}
	if number < 0 {
		return 0, fmt.Errorf("must be non-negative")
	}

	multiplier, ok := map[string]int64{
		"":      1,
		"B":     1,
		"BYTE":  1,
		"BYTES": 1,
		"K":     1024,
		"KB":    1024,
		"KIB":   1024,
		"M":     1024 * 1024,
		"MB":    1024 * 1024,
		"MIB":   1024 * 1024,
		"G":     1024 * 1024 * 1024,
		"GB":    1024 * 1024 * 1024,
		"GIB":   1024 * 1024 * 1024,
		"T":     1024 * 1024 * 1024 * 1024,
		"TB":    1024 * 1024 * 1024 * 1024,
		"TIB":   1024 * 1024 * 1024 * 1024,
		"P":     1024 * 1024 * 1024 * 1024 * 1024,
		"PB":    1024 * 1024 * 1024 * 1024 * 1024,
		"PIB":   1024 * 1024 * 1024 * 1024 * 1024,
	}[unitPart]
	if !ok {
		return 0, fmt.Errorf("unsupported size suffix %q", unitPart)
	}
	if number > math.MaxInt64/multiplier {
		return 0, fmt.Errorf("value too large")
	}
	return number * multiplier, nil
}
