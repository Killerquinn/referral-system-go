package config

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

var k = koanf.New(".")

type Config struct {
	Server   ServerConfig
	Postgres PostgresConfig
	R2       R2Config
	Stripe   StripeConfig
	Jaeger   JaegerConfig
	//Prometheus PrometheusConfig
	JWT     JWTokenConfig
	Refresh RefreshConfig
	Code    RecoveryCodeConfig
	Options Options
}

type ServerConfig struct {
	Port              int           `koanf:"port"`
	Env               string        `koanf:"env"` // development, production, local
	Name              string        `koanf:"name"`
	ReadTimeout       time.Duration `koanf:"read_timeout" default:"5s"`
	ReadHeaderTimeout time.Duration `koanf:"read_header_timeout" default:"5s"`
	IdleTimeout       time.Duration `koanf:"idle_timeout" default:"5s"`
	WriteTimeout      time.Duration `koanf:"write_timeout" default:"5s"`
	// CIDRs of reverse proxies allowed to set X-Forwarded-For.
	// Empty = no proxy in front, client IP is taken from the TCP connection.
	TrustedProxies []string `koanf:"trusted_proxies"`
}

type PostgresConfig struct {
	DSN                       string        `koanf:"dsn"`
	MaxConnections            int           `koanf:"max_connections" default:"20"`
	MinConnections            int           `koanf:"min_connections" default:"5"`
	MaxConnectionLifetime     time.Duration `koanf:"max_connection_lifetime" default:"1h"`
	MaxConnectionIdleLifetime time.Duration `koanf:"max_connection_idle_time" default:"30m"`
	HealthCheckPeriod         time.Duration `koanf:"health_check_period" default:"10s"`
}

// R2Config — Cloudflare R2 via its S3-compatible API.
// Only the S3 access key pair is needed; a Cloudflare API token is not used at runtime.
type R2Config struct {
	AccountID       string `koanf:"account_id"`
	AccessKeyID     string `koanf:"access_key_id"`
	SecretAccessKey string `koanf:"secret_access_key"`
	// Public bucket: avatars, page backgrounds, music — served directly via PublicBaseURL.
	PublicBucket string `koanf:"public_bucket"`
	// Private bucket: paid post media — served only via short-lived presigned GET URLs.
	PrivateBucket string `koanf:"private_bucket"`
	// r2.dev subdomain or custom domain bound to the public bucket, no trailing slash.
	PublicBaseURL string `koanf:"public_base_url"`
	// Lifetime of presigned upload / download URLs.
	PresignTTL time.Duration `koanf:"presign_ttl"`
}

type StripeConfig struct {
	SecretKey     string `koanf:"secret_key"`
	WebhookSecret string `koanf:"webhook_secret"`
}

type JaegerConfig struct {
	Endpoint string  `koanf:"endpoint"`
	Sampler  float64 `koanf:"sampler" default:"1.0"`
}

type JWTokenConfig struct {
	Secret string        `koanf:"secret_key"` //To-Do: make tokenizer!
	TTL    time.Duration `koanf:"ttl"`
}

type RefreshConfig struct {
	HMACSecret string        `koanf:"hmac_secret"`
	TTL        time.Duration `koanf:"refresh_ttl"`
}

type RecoveryCodeConfig struct {
	HMACSecret string        `koanf:"hmac_secret"`
	TTL        time.Duration `koanf:"code_ttl"` // password reset: the user is waiting for it right now
	// email verification: sent on register, the user may open the email much later
	VerifyTTL time.Duration `koanf:"verify_ttl"`
}

type Options struct {
	BaseUrl string `koanf:"baseurl"`
	// where the Vue app lives; used for links that open the frontend (e.g. page referral links)
	FrontendURL string `koanf:"frontend_url"`
}

func LoadConfig() *Config {
	k := koanf.New(".")

	_ = godotenv.Load()

	configPath := findConfigFile()
	if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
		fmt.Printf("Warning: %s not found, relying on env vars\n", configPath)
	}

	if err := k.Load(env.Provider("", ".", func(s string) string {
		switch s {
		case "APP_POSTGRES_DSN":
			return "postgres.dsn"
		case "APP_STRIPE_SECRET_KEY":
			return "stripe.secret_key"
		case "APP_STRIPE_WEBHOOK_SECRET":
			return "stripe.webhook_secret"
		case "APP_JWT_SECRET_KEY":
			return "jwt.secret_key"
		case "APP_HMAC_SECRET_KEY":
			return "refresh.hmac_secret"
		case "APP_R2_ACCOUNT_ID":
			return "r2.account_id"
		case "APP_R2_ACCESS_ID_S3":
			return "r2.access_key_id"
		case "APP_R2_SECRET_ACCESS_KEY":
			return "r2.secret_access_key"
		case "APP_RECOVERY_CODE_HMAC_SECRET_KEY":
			return "code.hmac_secret"
		default:
			return strings.ToLower(s)
		}
	}), nil); err != nil {
		log.Fatalf("Error loading env vars: %v", err)
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		log.Fatalf("Error unmarshaling config: %v", err)
	}

	if cfg.Code.HMACSecret == "" {
		log.Fatal("APP_RECOVERY_CODE_HMAC_SECRET_KEY is not set")
	}

	if cfg.Code.VerifyTTL == 0 {
		cfg.Code.VerifyTTL = 24 * time.Hour
	}

	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.Env == "" {
		cfg.Server.Env = "development"
	}
	if cfg.Server.Name == "" {
		cfg.Server.Name = "local_project"
	}
	if cfg.R2.PresignTTL == 0 {
		cfg.R2.PresignTTL = 15 * time.Minute
	}
	cfg.R2.PublicBaseURL = strings.TrimRight(cfg.R2.PublicBaseURL, "/")

	return &cfg
}

func findConfigFile() string {
	paths := []string{
		"config.yaml",
		"./config.yaml",
		"../../config.yaml",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "config.yaml"
}

func Get(key string) interface{} {
	return k.String(key)
}

func AskToContinue(sentence string) bool {
	for {
		var resp string
		fmt.Println(sentence)

		_, err := fmt.Scanln(&resp)
		if err != nil {
			fmt.Println("Error reading input, try again...")
			continue
		}

		resp = strings.ToLower(strings.TrimSpace(resp))
		if resp == "y" || resp == "yes" {
			return true

		}
		if resp == "n" || resp == "no" {
			return false
		}
		fmt.Println("Invalid input, try again...")
	}
}
