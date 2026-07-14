package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration parameters.
type Config struct {
	RESTPort      string
	GRPCPort      string
	DatabaseURL   string
	RedisURL      string
	QdrantURL     string
	OpenAIKey     string
	ClaudeKey     string
	GeminiKey     string
	DeepSeekKey   string
	OllamaURL     string
	DefaultModel  string
	DefaultProv   string
	ClientTimeout time.Duration
}

// LoadConfig loads configuration from environment variables with sensible defaults.
func LoadConfig() *Config {
	return &Config{
		RESTPort:      getEnv("REST_PORT", "8080"),
		GRPCPort:      getEnv("GRPC_PORT", "50051"),
		DatabaseURL:   getEnv("DATABASE_URL", ""), // empty defaults to local-only / memory-fallback
		RedisURL:      getEnv("REDIS_URL", "localhost:6379"),
		QdrantURL:     getEnv("QDRANT_URL", ""), // empty defaults to local-vector / memory-fallback
		OpenAIKey:     getEnv("OPENAI_API_KEY", ""),
		ClaudeKey:     getEnv("ANTHROPIC_API_KEY", ""),
		GeminiKey:     getEnv("GEMINI_API_KEY", ""),
		DeepSeekKey:   getEnv("DEEPSEEK_API_KEY", ""),
		OllamaURL:     getEnv("OLLAMA_URL", "http://localhost:11434"),
		DefaultModel:  getEnv("DEFAULT_MODEL", "mock-model"),
		DefaultProv:   getEnv("DEFAULT_PROVIDER", "mock"),
		ClientTimeout: time.Duration(getEnvInt("CLIENT_TIMEOUT_SECONDS", 30)) * time.Second,
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}
