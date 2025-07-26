package config

import (
	"fmt"
	"os"
)

type Config struct {
	DBPath    string
	JWTSecret string
	Port      string
}

// Load reads configuration from environment variables and returns a Config struct.
func Load() (*Config, error) {
	cfg := &Config{
		DBPath:    getEnv("DB_PATH", "./battlewordle.db"),
		JWTSecret: os.Getenv("JWT_SECRET"),
		Port:      getEnv("PORT", "8080"),
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required but not set")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
