package config

import (
	"reflect"
	"testing"
)

func TestSplitList(t *testing.T) {
	got := splitList(" https://a.com, ,https://b.com ,")
	want := []string{"https://a.com", "https://b.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitList = %v, want %v", got, want)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("APP_CONFIG_PATH", "/nonexistent.yaml") // env only, no file
	t.Setenv("APP_RECOVERY_CODE_HMAC_SECRET_KEY", "x")
	t.Setenv("APP_FRONTEND_URL", "https://example.com/")
	t.Setenv("APP_TRUSTED_PROXIES", "172.16.0.0/12,127.0.0.1/32")

	cfg := LoadConfig()

	if cfg.Options.FrontendURL != "https://example.com" {
		t.Errorf("FrontendURL = %q", cfg.Options.FrontendURL)
	}
	// the frontend URL is always a CORS origin
	if want := []string{"https://example.com"}; !reflect.DeepEqual(cfg.Server.CORSOrigins, want) {
		t.Errorf("CORSOrigins = %v, want %v", cfg.Server.CORSOrigins, want)
	}
	if want := []string{"172.16.0.0/12", "127.0.0.1/32"}; !reflect.DeepEqual(cfg.Server.TrustedProxies, want) {
		t.Errorf("TrustedProxies = %v, want %v", cfg.Server.TrustedProxies, want)
	}
	if cfg.Server.WriteTimeout == 0 {
		t.Error("WriteTimeout must get a default")
	}
}

func TestCORSOriginsAlwaysIncludeFrontend(t *testing.T) {
	t.Setenv("APP_CONFIG_PATH", "/nonexistent.yaml")
	t.Setenv("APP_RECOVERY_CODE_HMAC_SECRET_KEY", "x")
	t.Setenv("APP_FRONTEND_URL", "https://example.com")
	// extras are added on top; listing the frontend again must not duplicate it
	t.Setenv("APP_CORS_ORIGINS", "http://localhost:5173,https://example.com")

	cfg := LoadConfig()

	want := []string{"http://localhost:5173", "https://example.com"}
	if !reflect.DeepEqual(cfg.Server.CORSOrigins, want) {
		t.Errorf("CORSOrigins = %v, want %v", cfg.Server.CORSOrigins, want)
	}
}

func TestCheckProductionSecrets(t *testing.T) {
	strong := "0123456789abcdef0123456789abcdef"
	cfg := &Config{
		JWT:     JWTokenConfig{Secret: strong},
		Refresh: RefreshConfig{HMACSecret: strong},
		Code:    RecoveryCodeConfig{HMACSecret: strong},
		Options: Options{FrontendURL: "https://example.com"},
	}
	if err := checkProductionSecrets(cfg); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}

	cfg.JWT.Secret = "supersecret xD"
	if checkProductionSecrets(cfg) == nil {
		t.Error("placeholder JWT secret accepted")
	}

	cfg.JWT.Secret = strong
	cfg.Options.FrontendURL = "http://localhost:5173"
	if checkProductionSecrets(cfg) == nil {
		t.Error("localhost frontend URL accepted")
	}
}
