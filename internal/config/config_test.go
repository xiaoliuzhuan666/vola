package config

import (
	"testing"

	"github.com/google/uuid"
)

func TestLoadWithOverridesDefaultsUserStorageQuotaTo100MB(t *testing.T) {
	cfg, err := LoadWithOverrides(map[string]string{
		"JWT_SECRET":       "secret",
		"VAULT_MASTER_KEY": "vault",
	})
	if err != nil {
		t.Fatalf("LoadWithOverrides: %v", err)
	}
	const want = 100 * 1024 * 1024
	if cfg.UserStorageQuotaBytes != want {
		t.Fatalf("UserStorageQuotaBytes = %d, want %d", cfg.UserStorageQuotaBytes, want)
	}
}

func TestLoadWithOverridesParsesUserStorageQuotaUnits(t *testing.T) {
	cfg, err := LoadWithOverrides(map[string]string{
		"JWT_SECRET":               "secret",
		"VAULT_MASTER_KEY":         "vault",
		"USER_STORAGE_QUOTA_BYTES": "10GB",
	})
	if err != nil {
		t.Fatalf("LoadWithOverrides: %v", err)
	}
	const want = 10 * 1024 * 1024 * 1024
	if cfg.UserStorageQuotaBytes != want {
		t.Fatalf("UserStorageQuotaBytes = %d, want %d", cfg.UserStorageQuotaBytes, want)
	}
}

func TestLoadWithOverridesRejectsInvalidUserStorageQuota(t *testing.T) {
	_, err := LoadWithOverrides(map[string]string{
		"JWT_SECRET":               "secret",
		"VAULT_MASTER_KEY":         "vault",
		"USER_STORAGE_QUOTA_BYTES": "10XB",
	})
	if err == nil {
		t.Fatal("LoadWithOverrides succeeded for invalid storage quota")
	}
}

func TestLoadWithOverridesParsesGitMirrorManualSyncCooldown(t *testing.T) {
	cfg, err := LoadWithOverrides(map[string]string{
		"JWT_SECRET":       "secret",
		"VAULT_MASTER_KEY": "vault",
		"GIT_MIRROR_MANUAL_SYNC_COOLDOWN_SECONDS": "45",
	})
	if err != nil {
		t.Fatalf("LoadWithOverrides: %v", err)
	}
	if !cfg.GitMirrorManualSyncCooldownConfigured || cfg.GitMirrorManualSyncCooldownSeconds != 45 {
		t.Fatalf("cooldown = configured:%v seconds:%d, want configured:true seconds:45", cfg.GitMirrorManualSyncCooldownConfigured, cfg.GitMirrorManualSyncCooldownSeconds)
	}
}

func TestLoadWithOverridesRejectsInvalidGitMirrorManualSyncCooldown(t *testing.T) {
	_, err := LoadWithOverrides(map[string]string{
		"JWT_SECRET":       "secret",
		"VAULT_MASTER_KEY": "vault",
		"GIT_MIRROR_MANUAL_SYNC_COOLDOWN_SECONDS": "-1",
	})
	if err == nil {
		t.Fatal("LoadWithOverrides succeeded for invalid Git Mirror manual sync cooldown")
	}
}

func TestLoadWithOverridesParsesInstanceAdminUserIDs(t *testing.T) {
	first := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	second := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	cfg, err := LoadWithOverrides(map[string]string{
		"JWT_SECRET":              "secret",
		"VAULT_MASTER_KEY":        "vault",
		"INSTANCE_ADMIN_USER_IDS": first.String() + ", " + second.String() + "\n" + first.String(),
	})
	if err != nil {
		t.Fatalf("LoadWithOverrides: %v", err)
	}
	if len(cfg.InstanceAdminUserIDs) != 2 || cfg.InstanceAdminUserIDs[0] != first || cfg.InstanceAdminUserIDs[1] != second {
		t.Fatalf("InstanceAdminUserIDs = %#v", cfg.InstanceAdminUserIDs)
	}
}

func TestLoadWithOverridesRejectsInvalidInstanceAdminUserID(t *testing.T) {
	_, err := LoadWithOverrides(map[string]string{
		"JWT_SECRET":              "secret",
		"VAULT_MASTER_KEY":        "vault",
		"INSTANCE_ADMIN_USER_IDS": "not-a-uuid",
	})
	if err == nil {
		t.Fatal("LoadWithOverrides succeeded for invalid instance admin user id")
	}
}

func TestLoadWithOverridesPublicRegistrationDefaults(t *testing.T) {
	localCfg, err := LoadWithOverrides(map[string]string{
		"JWT_SECRET":       "secret",
		"VAULT_MASTER_KEY": "vault",
		"PUBLIC_BASE_URL":  "http://127.0.0.1:18080",
	})
	if err != nil {
		t.Fatalf("LoadWithOverrides local: %v", err)
	}
	if !localCfg.EnablePublicRegistration {
		t.Fatal("EnablePublicRegistration = false for local base URL, want true")
	}

	publicCfg, err := LoadWithOverrides(map[string]string{
		"DATABASE_URL":     "postgres://vola:prod-password@db:5432/vola?sslmode=disable",
		"JWT_SECRET":       "prod-jwt-secret",
		"VAULT_MASTER_KEY": "prod-vault-key",
		"PUBLIC_BASE_URL":  "https://vola.example.com",
	})
	if err != nil {
		t.Fatalf("LoadWithOverrides public: %v", err)
	}
	if publicCfg.EnablePublicRegistration {
		t.Fatal("EnablePublicRegistration = true for public base URL, want false")
	}
}

func TestLoadWithOverridesPublicRegistrationOverride(t *testing.T) {
	cfg, err := LoadWithOverrides(map[string]string{
		"DATABASE_URL":                    "postgres://vola:prod-password@db:5432/vola?sslmode=disable",
		"JWT_SECRET":                      "prod-jwt-secret",
		"VAULT_MASTER_KEY":                "prod-vault-key",
		"PUBLIC_BASE_URL":                 "https://vola.example.com",
		"VOLA_ENABLE_PUBLIC_REGISTRATION": "true",
	})
	if err != nil {
		t.Fatalf("LoadWithOverrides: %v", err)
	}
	if !cfg.EnablePublicRegistration {
		t.Fatal("EnablePublicRegistration = false, want true")
	}
}

func TestLoadWithOverridesNormalizesCORSOrigins(t *testing.T) {
	cfg, err := LoadWithOverrides(map[string]string{
		"JWT_SECRET":       "secret",
		"VAULT_MASTER_KEY": "vault",
		"CORS_ORIGINS":     " https://one.example.com,HTTPS://TWO.EXAMPLE.COM/,https://one.example.com, ",
	})
	if err != nil {
		t.Fatalf("LoadWithOverrides: %v", err)
	}
	if len(cfg.CORSOrigins) != 2 || cfg.CORSOrigins[0] != "https://one.example.com" || cfg.CORSOrigins[1] != "https://two.example.com" {
		t.Fatalf("CORSOrigins = %#v", cfg.CORSOrigins)
	}
}

func TestLoadWithOverridesRejectsWildcardCORSOrigin(t *testing.T) {
	_, err := LoadWithOverrides(map[string]string{
		"JWT_SECRET":       "secret",
		"VAULT_MASTER_KEY": "vault",
		"CORS_ORIGINS":     "*",
	})
	if err == nil {
		t.Fatal("LoadWithOverrides succeeded for wildcard CORS origin")
	}
}

func TestLoadWithOverridesRejectsInvalidCORSOrigin(t *testing.T) {
	for _, value := range []string{
		"vola.example.com",
		"https://vola.example.com/path",
		"https://vola.example.com?debug=1",
		"https://user@vola.example.com",
		"ftp://vola.example.com",
	} {
		t.Run(value, func(t *testing.T) {
			_, err := LoadWithOverrides(map[string]string{
				"JWT_SECRET":       "secret",
				"VAULT_MASTER_KEY": "vault",
				"CORS_ORIGINS":     value,
			})
			if err == nil {
				t.Fatalf("LoadWithOverrides succeeded for invalid CORS origin %q", value)
			}
		})
	}
}

func TestLoadWithOverridesUsesOverridesForNumericAndBooleanConfig(t *testing.T) {
	cfg, err := LoadWithOverrides(map[string]string{
		"JWT_SECRET":                  "secret",
		"VAULT_MASTER_KEY":            "vault",
		"RATE_LIMIT":                  "250",
		"MAX_BODY_SIZE":               "10MB",
		"VOLA_ENABLE_SYSTEM_SETTINGS": "false",
		"VOLA_ENABLE_BILLING":         "true",
		"VOLA_CAPTURE_OAUTH":          "true",
	})
	if err != nil {
		t.Fatalf("LoadWithOverrides: %v", err)
	}
	if cfg.RateLimit != 250 {
		t.Fatalf("RateLimit = %d, want 250", cfg.RateLimit)
	}
	if cfg.MaxBodySize != 10*1024*1024 {
		t.Fatalf("MaxBodySize = %d, want 10485760", cfg.MaxBodySize)
	}
	if cfg.EnableSystemSettings {
		t.Fatal("EnableSystemSettings = true, want false")
	}
	if !cfg.EnableBilling {
		t.Fatal("EnableBilling = false, want true")
	}
	if !cfg.CaptureOAuth {
		t.Fatal("CaptureOAuth = false, want true")
	}
}

func TestLoadWithOverridesRejectsInvalidNumericConfig(t *testing.T) {
	for _, tc := range []struct {
		name  string
		key   string
		value string
	}{
		{name: "rate limit text", key: "RATE_LIMIT", value: "fast"},
		{name: "rate limit zero", key: "RATE_LIMIT", value: "0"},
		{name: "max body size text", key: "MAX_BODY_SIZE", value: "large"},
		{name: "max body size zero", key: "MAX_BODY_SIZE", value: "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			overrides := map[string]string{
				"JWT_SECRET":       "secret",
				"VAULT_MASTER_KEY": "vault",
				tc.key:             tc.value,
			}
			if _, err := LoadWithOverrides(overrides); err == nil {
				t.Fatalf("LoadWithOverrides succeeded for %s=%q", tc.key, tc.value)
			}
		})
	}
}

func TestLoadWithOverridesRejectsInvalidBooleanConfig(t *testing.T) {
	for _, key := range []string{
		"TENCENT_COS_PATH_STYLE",
		"VOLA_ENABLE_SYSTEM_SETTINGS",
		"VOLA_ENABLE_BILLING",
		"VOLA_ENABLE_PUBLIC_REGISTRATION",
		"VOLA_CAPTURE_OAUTH",
	} {
		t.Run(key, func(t *testing.T) {
			overrides := map[string]string{
				"JWT_SECRET":       "secret",
				"VAULT_MASTER_KEY": "vault",
				key:                "maybe",
			}
			if _, err := LoadWithOverrides(overrides); err == nil {
				t.Fatalf("LoadWithOverrides succeeded for %s=maybe", key)
			}
		})
	}
}

func TestLoadWithOverridesParsesCOSConfig(t *testing.T) {
	cfg, err := LoadWithOverrides(map[string]string{
		"JWT_SECRET":             "secret",
		"VAULT_MASTER_KEY":       "vault",
		"OBJECT_STORAGE_BACKEND": "cos",
		"TENCENT_COS_BUCKET":     "demo-1250000000",
		"TENCENT_COS_REGION":     "ap-guangzhou",
		"TENCENT_COS_SECRET_ID":  "secret-id",
		"TENCENT_COS_SECRET_KEY": "secret-key",
		"TENCENT_COS_PREFIX":     "prod/vola",
		"TENCENT_COS_PATH_STYLE": "true",
	})
	if err != nil {
		t.Fatalf("LoadWithOverrides: %v", err)
	}
	if cfg.ObjectStorageBackend != "cos" {
		t.Fatalf("ObjectStorageBackend = %q", cfg.ObjectStorageBackend)
	}
	if !cfg.TencentCOSPathStyle {
		t.Fatal("TencentCOSPathStyle = false, want true")
	}
	if cfg.TencentCOSPrefix != "prod/vola" {
		t.Fatalf("TencentCOSPrefix = %q", cfg.TencentCOSPrefix)
	}
}

func TestLoadWithOverridesRejectsIncompleteCOSConfig(t *testing.T) {
	_, err := LoadWithOverrides(map[string]string{
		"JWT_SECRET":             "secret",
		"VAULT_MASTER_KEY":       "vault",
		"OBJECT_STORAGE_BACKEND": "cos",
	})
	if err == nil {
		t.Fatal("LoadWithOverrides succeeded for incomplete COS config")
	}
}

func TestLoadWithOverridesAllowsDevelopmentDefaultsForLocalPublicBaseURL(t *testing.T) {
	_, err := LoadWithOverrides(map[string]string{
		"DATABASE_URL":     "postgres://vola:vola_dev@db:5432/vola?sslmode=disable",
		"JWT_SECRET":       developmentJWTSecret,
		"VAULT_MASTER_KEY": developmentVaultMasterKey,
		"PUBLIC_BASE_URL":  "http://127.0.0.1:18080",
	})
	if err != nil {
		t.Fatalf("LoadWithOverrides rejected local development defaults: %v", err)
	}
}

func TestLoadWithOverridesRejectsDevelopmentDefaultsForPublicBaseURL(t *testing.T) {
	cases := []struct {
		name      string
		overrides map[string]string
	}{
		{
			name: "jwt secret",
			overrides: map[string]string{
				"DATABASE_URL":     "postgres://vola:prod-password@db:5432/vola?sslmode=disable",
				"JWT_SECRET":       developmentJWTSecret,
				"VAULT_MASTER_KEY": "prod-vault-key",
				"PUBLIC_BASE_URL":  "https://www.vola.ai",
			},
		},
		{
			name: "sqlite jwt secret",
			overrides: map[string]string{
				"DATABASE_URL":     "postgres://vola:prod-password@db:5432/vola?sslmode=disable",
				"JWT_SECRET":       localSQLiteJWTSecret,
				"VAULT_MASTER_KEY": "prod-vault-key",
				"PUBLIC_BASE_URL":  "https://www.vola.ai",
			},
		},
		{
			name: "vault master key",
			overrides: map[string]string{
				"DATABASE_URL":     "postgres://vola:prod-password@db:5432/vola?sslmode=disable",
				"JWT_SECRET":       "prod-jwt-secret",
				"VAULT_MASTER_KEY": developmentVaultMasterKey,
				"PUBLIC_BASE_URL":  "https://www.vola.ai",
			},
		},
		{
			name: "database url",
			overrides: map[string]string{
				"JWT_SECRET":       "prod-jwt-secret",
				"VAULT_MASTER_KEY": "prod-vault-key",
				"PUBLIC_BASE_URL":  "https://www.vola.ai",
			},
		},
		{
			name: "postgres password",
			overrides: map[string]string{
				"DATABASE_URL":     "postgres://vola:vola_dev@db:5432/vola?sslmode=disable",
				"JWT_SECRET":       "prod-jwt-secret",
				"VAULT_MASTER_KEY": "prod-vault-key",
				"PUBLIC_BASE_URL":  "https://www.vola.ai",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := LoadWithOverrides(tc.overrides); err == nil {
				t.Fatal("LoadWithOverrides succeeded with development defaults for public base URL")
			}
		})
	}
}

func TestLoadWithOverridesAllowsProductionSecretsForPublicBaseURL(t *testing.T) {
	_, err := LoadWithOverrides(map[string]string{
		"DATABASE_URL":     "postgres://vola:prod-password@db:5432/vola?sslmode=disable",
		"JWT_SECRET":       "prod-jwt-secret",
		"VAULT_MASTER_KEY": "prod-vault-key",
		"PUBLIC_BASE_URL":  "https://www.vola.ai",
	})
	if err != nil {
		t.Fatalf("LoadWithOverrides rejected production config: %v", err)
	}
}
