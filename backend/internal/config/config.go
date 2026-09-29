package config

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL        string
	RedisURL           string
	Port               string
	JWTSecret          string
	JWTAccessTTL       time.Duration
	JWTRefreshTTL      time.Duration
	EphemeralTTL       time.Duration
	FrontendDir        string
	ClickBatchSize     int
	ClickFlushInterval time.Duration
}

// loadDotEnv searches for .env in current dir and parent dir
func loadDotEnv() {
	paths := []string{".env", "../.env"}
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				val = strings.Trim(val, `"'`)
				if os.Getenv(key) == "" {
					_ = os.Setenv(key, val)
				}
			}
		}
		log.Printf("Loaded environment from %s", p)
		break
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func parseDurationEnv(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}

func parseIntEnv(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func Load() *Config {
	loadDotEnv()

	frontendDir := os.Getenv("FRONTEND_DIR")
	if frontendDir == "" {
		frontendDir = "../frontend"
	}

	return &Config{
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://shorty:shorty_secret@localhost:5432/shorty_db?sslmode=disable"),
		RedisURL:           getEnv("REDIS_URL", "redis://localhost:6379/0"),
		Port:               getEnv("PORT", "8080"),
		JWTSecret:          getEnv("JWT_SECRET", "shorty-super-secret-jwt-key-2026-secure!"),
		JWTAccessTTL:       parseDurationEnv("JWT_ACCESS_TTL", 15*time.Minute),
		JWTRefreshTTL:      parseDurationEnv("JWT_REFRESH_TTL", 7*24*time.Hour),
		EphemeralTTL:       parseDurationEnv("EPHEMERAL_TTL", 2*time.Hour),
		FrontendDir:        frontendDir,
		ClickBatchSize:     parseIntEnv("CLICK_BATCH_SIZE", 100),
		ClickFlushInterval: parseDurationEnv("CLICK_FLUSH_INTERVAL", 2*time.Second),
	}
}
