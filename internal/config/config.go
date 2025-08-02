package config

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	LogLevel            string   `envconfig:"LOG_LEVEL"             default:"debug"`
	PostgresUrl         string   `envconfig:"POSTGRES_URL"          default:"postgres://t3m8ch@localhost/productsdb"`
	RedisUrl            string   `envconfig:"REDIS_URL"             default:"redis://dragonfly:6379/0"`
	SessionCookieSecure bool     `envconfig:"SESSION_COOKIE_SECURE" default:"false"`
	SessionCookieDomain string   `envconfig:"SESSION_COOKIE_DOMAIN" default:"localhost:5173"`
	AllowOrigins        []string `envconfig:"ALLOW_ORIGINS"         default:"http://localhost:5173"`
	AdminUsername       string   `envconfig:"ADMIN_USERNAME"        default:"admin"`
	AdminPassword       string   `envconfig:"ADMIN_PASSWORD"        default:"123456"`
	AdminEmail          string   `envconfig:"ADMIN_EMAIL"           default:"admin@admin.ru"`
	GinMode             string   `envconfig:"GIN_MODE"              default:"debug"`
	Host                string   `envconfig:"HOST"                  default:"0.0.0.0"`
	Port                int      `envconfig:"PORT"                  default:"80"`
	SwaggerHost         string   `envconfig:"SWAGGER_HOST"          default:"localhost"`
	SwaggerPort         int      `envconfig:"SWAGGER_PORT"          default:"8080"`

	DocsTitle       string `envconfig:"DOCS_TITLE"       default:"MergeMinds API"`
	DocsDescription string `envconfig:"DOCS_DESCRIPTION" default:"MergeMinds API documentation"`
}

func LoadFromEnv() (*Config, error) {
	var config Config
	err := envconfig.Process("", &config)
	if err != nil {
		fmt.Printf("Failed to process env vars: %v", err)
		return nil, err
	}

	return &config, nil
}
