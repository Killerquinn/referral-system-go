package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/dotenv"
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
}

type ServerConfig struct {
	Port string `koanf:"port"`
	Env  string `koanf:"env"` // development, production, local
	Name string `koanf:"name"`
}

type PostgresConfig struct {
	DSN                       string `koanf:"dsn"`
	MaxConnections            int    `koanf:"max_connections" default:"20"`
	MinConnections            int    `koanf:"min_connections" default:"5"`
	MaxConnectionLifetime     int    `koanf:"max_connection_lifetime" default:"1h"`
	MaxConnectionIdleLifetime int    `koanf:"max_connection_idle_time" default:"30m"`
}

type StripeConfig struct {
	SecretKey     string `koanf:"secret_key"`
	WebhookSecret string `koanf:"webhook_secret"`
}

type JaegerConfig struct {
	Endpoint string
	Sampler  float64 `koanf:"sampler" default:"1.0"`
}

func LoadConfig() *Config {
	k = koanf.New(".")
	if err := k.Load(file.Provider(".env"), dotenv.Parser()); err != nil {
		fmt.Println("Error loading config, .env files not found, continue? (y/n)", err)
		if !AskToContinue("Continue config loading without .env variables?") {
			fmt.Println("Exiting...")
			os.Exit(1)
		}
		fmt.Println("Continuing loading without .env variables...")
	}
	if err := k.Load(env.Provider("APP_", ".", func(s string) string {
		return strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(s, "APP_")), "_", ".")
	}), nil); err != nil {
		log.Fatal(err)
	}

	var cfg Config

	if err := k.Unmarshal("", &cfg); err != nil {
		log.Fatal(err)
	}

	if cfg.Server.Port == "" {
		log.Println("No port specified, defaulting to 8080")
		cfg.Server.Port = "8080"
	}
	if cfg.Server.Env == "" {
		cfg.Server.Env = "development"
		fmt.Println("No environment specified, defaulting to development")
	}
	if cfg.Server.Name == "" {
		cfg.Server.Name = "local_project"
		fmt.Println("No name specified, defaulting to local_project")
	}

	return &cfg
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
