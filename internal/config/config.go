package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration.
type Config struct {
	// Server
	Port string
	Mode string // "debug" | "release"

	// Database
	DatabaseURL string
	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string

	// Redis
	RedisURL string

	// LLM
	OpenAI_API_KEY string
	OpenAI_Model   string

	// ArXiv
	ArxivBaseURL string

	// INSPIRE-HEP
	InspireBaseURL string
	InspireToken   string

	// Python
	PythonPath string

	// Storage
	DataDir       string
	CacheDir      string
	PlotOutputDir string
}

// Load reads configuration from environment variables and .env file.
func Load() (*Config, error) {
	_ = godotenv.Load()

	getEnv := func(key, fallback string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fallback
	}
	getEnvInt := func(key string, fallback int) int {
		if v := os.Getenv(key); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		}
		return fallback
	}

	cfg := &Config{
		Port:           getEnv("PORT", "8080"),
		Mode:           getEnv("GIN_MODE", "debug"),
		DatabaseURL:    getEnv("DATABASE_URL", ""),
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBUser:         getEnv("DB_USER", "cepc"),
		DBPassword:     getEnv("DB_PASSWORD", "cepc_secret"),
		DBName:         getEnv("DB_NAME", "cepc_copilot"),
		RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379/0"),
		OpenAI_API_KEY: getEnv("OPENAI_API_KEY", ""),
		OpenAI_Model:   getEnv("OPENAI_MODEL", "gpt-4o"),
		ArxivBaseURL:   getEnv("ARXIV_BASE_URL", "http://export.arxiv.org/api/query"),
		InspireBaseURL: getEnv("INSPIRE_BASE_URL", "https://inspirehep.net/api"),
		InspireToken:   getEnv("INSPIRE_TOKEN", ""),
		PythonPath:     getEnv("PYTHON_PATH", "python3"),
		DataDir:        getEnv("DATA_DIR", "./data"),
		CacheDir:       getEnv("CACHE_DIR", "./cache"),
		PlotOutputDir:  getEnv("PLOT_OUTPUT_DIR", "./cache/plots"),
	}

	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s?sslmode=disable",
			cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName,
		)
	}

	return cfg, nil
}
