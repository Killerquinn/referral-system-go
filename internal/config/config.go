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
	Stripe   StripeConfig
	Jaeger   JaegerConfig
	//Prometheus PrometheusConfig
	JWT  JWTokenConfig
	Opts Options
}

type ServerConfig struct {
	Port              int           `koanf:"port"`
	Env               string        `koanf:"env"` // development, production, local
	Name              string        `koanf:"name"`
	ReadTimeout       time.Duration `koanf:"read_timeout" default:"5s"`
	ReadHeaderTimeout time.Duration `koanf:"read_header_timeout" default:"5s"`
	IdleTimeout       time.Duration `koanf:"idle_timeout" default:"5s"`
	WriteTimeout      time.Duration `koanf:"write_timeout" default:"5s"`
}

type PostgresConfig struct {
	DSN                       string        `koanf:"dsn"`
	MaxConnections            int           `koanf:"max_connections" default:"20"`
	MinConnections            int           `koanf:"min_connections" default:"5"`
	MaxConnectionLifetime     time.Duration `koanf:"max_connection_lifetime" default:"1h"`
	MaxConnectionIdleLifetime time.Duration `koanf:"max_connection_idle_time" default:"30m"`
	HealthCheckPeriod         time.Duration `koanf:"health_check_period" default:"10s"`
}

type StripeConfig struct {
	SecretKey     string `koanf:"APP_STRIPE_SECRET_KEY"`
	WebhookSecret string `koanf:"APP_STRIPE_WEBHOOK_SECRET"`
}

type JaegerConfig struct {
	Endpoint string  `koanf:"endpoint"`
	Sampler  float64 `koanf:"sampler" default:"1.0"`
}

type JWTokenConfig struct {
	Secret string        `koanf:"secret_key"` //To-Do: make tokenizer!
	TTL    time.Duration `koanf:"ttl"`
}

type Options struct {
	BaseUrl string `koanf:"baseurl"`
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
		case "STRIPE_SECRET_KEY":
			return "stripe.secret_key"
		case "STRIPE_WEBHOOK_SECRET":
			return "stripe.webhook_secret"
		case "APP_JWT_SECRET_KEY":
			return "jwt.secret_key"
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

	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.Env == "" {
		cfg.Server.Env = "development"
	}
	if cfg.Server.Name == "" {
		cfg.Server.Name = "local_project"
	}

	//fmt.Printf("cfg.JWT.Secret is - %s", cfg.JWT.Secret)

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
