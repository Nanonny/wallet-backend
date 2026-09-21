package config

import (
	"fmt"
	"net/url"
	"os"
)

type Config struct {
	Storage       string
	RedisAddr     string
	RedisPassword string
	HTTPPort      string
	GRPCPort      string
	MetricsPort   string

	// PostgreSQL (user accounts). PG_DSN takes precedence over the
	// individual PG_* values when both are set.
	PGDSN      string
	PGHost     string
	PGPort     string
	PGUser     string
	PGPassword string
	PGDatabase string
	PGSSLMode  string

	// JWT_SECRET signs session tokens. When empty, the server generates a
	// random one at startup and logs a warning (sessions won't survive
	// restarts — fine for local dev only).
	JWTSecret string
}

func Load() Config {
	return Config{
		Storage:       env("STORAGE", "redis"),
		RedisAddr:     env("REDIS_ADDR", "localhost:6379"),
		RedisPassword: env("REDIS_PASSWORD", ""),
		HTTPPort:      env("HTTP_PORT", "8080"),
		GRPCPort:      env("GRPC_PORT", "50051"),
		MetricsPort:   env("METRICS_PORT", "9090"),

		PGDSN:      env("PG_DSN", ""),
		PGHost:     env("PGHOST", "localhost"),
		PGPort:     env("PGPORT", "5432"),
		PGUser:     env("PGUSER", "wallet"),
		PGPassword: env("PGPASSWORD", "wallet"),
		PGDatabase: env("PGDATABASE", "wallet"),
		PGSSLMode:  env("PGSSLMODE", "disable"),

		JWTSecret: env("JWT_SECRET", ""),
	}
}

// PostgresDSN returns a usable lib/pq data source name.
func (c Config) PostgresDSN() string {
	if c.PGDSN != "" {
		return c.PGDSN
	}
	host := fmt.Sprintf("%s:%s", c.PGHost, c.PGPort)
	if c.PGPassword != "" {
		auth := url.URL{User: url.UserPassword(c.PGUser, c.PGPassword)}
		return "postgres://" + auth.String() + "@" + host + "/" + c.PGDatabase + "?sslmode=" + c.PGSSLMode
	}
	return fmt.Sprintf("postgres://%s@%s/%s?sslmode=%s", c.PGUser, host, c.PGDatabase, c.PGSSLMode)
}

func env(key string, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
