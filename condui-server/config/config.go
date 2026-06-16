package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port               string
	DBPath             string
	JWTSecret          []byte
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
	AllowedOrigins     string
}

func Load() Config {
	jwtExpMins := getEnvInt("JWT_EXPIRY_MINUTES", 15)
	refreshExpDays := getEnvInt("REFRESH_EXPIRY_DAYS", 30)

	return Config{
		Port:               getEnv("PORT", "8080"),
		DBPath:             getEnv("DB_PATH", "./data/condui.db"),
		JWTSecret:          []byte(mustEnv("JWT_SECRET", "change-me-in-production-min-32-chars!!")),
		AccessTokenExpiry:  time.Duration(jwtExpMins) * time.Minute,
		RefreshTokenExpiry: time.Duration(refreshExpDays) * 24 * time.Hour,
		AllowedOrigins:     getEnv("ALLOWED_ORIGINS", "*"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
