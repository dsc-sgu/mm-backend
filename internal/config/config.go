package config

import (
	"context"
	"fmt"

	"github.com/joho/godotenv"
	"github.com/sethvargo/go-envconfig"
	"go.uber.org/zap"
)

type PostgresConfig struct {
	Host     string `env:"HOST,      default=localhost"`
	Port     int    `env:"PORT,      default=1301"`
	User     string `env:"USER,      default=postgres"`
	Password string `env:"PASSWORD,  default=postgres"`
	Database string `env:"DATABASE,  default=postgres"`
}

func (p *PostgresConfig) GetURL() string {
	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=disable", p.User, p.Password, p.Host, p.Port, p.Database)
}

type RedisConfig struct {
	Host     string `env:"HOST,      default=localhost"`
	Port     int    `env:"PORT,      default=1302"`
	Password string `env:"PASSWORD"`
	Database int    `env:"DATABASE,  default=0"`
}

func (r *RedisConfig) GetURL() string {
	return fmt.Sprintf("redis://%s:%d/%d", r.Host, r.Port, r.Database)
}

type OpenAPIConfig struct {
	Title       string `env:"DOCS_TITLE,       default=MergeMinds API"`
	Description string `env:"DOCS_DESCRIPTION, default=MergeMinds API documentation"`
	Host        string `env:"HOST,          default=localhost"`
	Port        int    `env:"PORT,          default=8080"`
}

type Config struct {
	Postgres PostgresConfig `env:", prefix=POSTGRES_"`
	Redis    RedisConfig    `env:", prefix=REDIS_"`
	OpenAPI  OpenAPIConfig  `env:", prefix=OPENAPI_"`

	EnableAuth          bool     `env:"ENABLE_AUTH, default=true"`
	SessionCookieSecure bool     `env:"SESSION_COOKIE_SECURE, default=false"`
	SessionCookieDomain string   `env:"SESSION_COOKIE_DOMAIN, default=localhost:5173"`
	AllowOrigins        []string `env:"ALLOW_ORIGINS,         default=http://localhost:5173"`

	GinMode  string `env:"GIN_MODE,              default=debug"`
	Host     string `env:"HOST,                  default=0.0.0.0"`
	HTTPPort int    `env:"HTTP_PORT,             default=8034"`
	SSHPort  int    `env:"PORT,                  default=2222"`

	LogLevel zap.AtomicLevel `env:"LOG_LEVEL,             default=debug"`
}

func LoadFromEnv() (*Config, error) {
	_ = godotenv.Load()
	config := Config{}
	if err := envconfig.Process(context.Background(), &config); err != nil {
		return nil, fmt.Errorf("populating config object from environment: %w", err)
	}

	return &config, nil
}
