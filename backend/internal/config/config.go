// Package config loads runtime settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// Config holds everything the server needs to start.
type Config struct {
	// DatabaseURL is a PostgreSQL connection string. Required.
	DatabaseURL string
	// Port is the TCP port the HTTP server listens on.
	Port string
	// StaticDir is the folder with the built frontend (index.html + assets).
	StaticDir string
}

// Load reads DATABASE_URL, PORT (default 8080) and STATIC_DIR (default "static").
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Port:        getenv("PORT", "8080"),
		StaticDir:   getenv("STATIC_DIR", "static"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if n, err := strconv.Atoi(cfg.Port); err != nil || n < 1 || n > 65535 {
		return Config{}, fmt.Errorf("PORT must be a number between 1 and 65535, got %q", cfg.Port)
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
