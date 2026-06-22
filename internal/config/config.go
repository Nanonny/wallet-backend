package config

import "os"

type Config struct {
	Storage     string
	RedisAddr   string
	HTTPPort    string
	GRPCPort    string
	MetricsPort string
}

func Load() Config {
	return Config{
		Storage:     env("STORAGE", "redis"),
		RedisAddr:   env("REDIS_ADDR", "localhost:6379"),
		HTTPPort:    env("HTTP_PORT", "8080"),
		GRPCPort:    env("GRPC_PORT", "50051"),
		MetricsPort: env("METRICS_PORT", "9090"),
	}
}

func env(key string, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
