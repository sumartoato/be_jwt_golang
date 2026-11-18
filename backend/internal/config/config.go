package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	JWTTTL      time.Duration
	// SwaggerEnabled mounts the API docs at /swagger/. Off by default so
	// production stays hidden unless explicitly turned on.
	SwaggerEnabled bool
	// SeedDemoUsers creates admin@medika.com, cs@medika.com and
	// perawat@medika.com on startup. Off by default: their password is public.
	SeedDemoUsers bool
}

func Load() (Config, error) {
	cfg := Config{
		Port:        getenv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
	}
	if cfg.DatabaseURL == "" {
		return cfg, errors.New("DATABASE_URL is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return cfg, errors.New("JWT_SECRET is required and must be at least 32 characters")
	}
	ttl, err := time.ParseDuration(getenv("JWT_TTL", "15m"))
	if err != nil || ttl <= 0 {
		return cfg, errors.New("JWT_TTL must be a positive duration, e.g. 15m")
	}
	cfg.JWTTTL = ttl
	swagger, err := strconv.ParseBool(getenv("SWAGGER_ENABLED", "false"))
	if err != nil {
		return cfg, errors.New("SWAGGER_ENABLED must be true or false")
	}
	cfg.SwaggerEnabled = swagger
	seed, err := strconv.ParseBool(getenv("SEED_DEMO_USERS", "false"))
	if err != nil {
		return cfg, errors.New("SEED_DEMO_USERS must be true or false")
	}
	cfg.SeedDemoUsers = seed
	return cfg, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
