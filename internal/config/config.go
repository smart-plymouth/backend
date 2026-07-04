package config

import (
	"net/url"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL        string
	RedisURL           string
	SecretKey          string
	NscaleBaseURL      string
	NscaleToken        string
	LLMModel           string
	EmbeddingModel     string
	PolicyVectorDir    string
	WorkerConcurrency  int
}

func Load() *Config {
	concurrency := 4
	if v := os.Getenv("WORKER_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			concurrency = n
		}
	}

	return &Config{
		DatabaseURL:       getEnv("DATABASE_URL", "postgresql://postgres:postgres@localhost:5432/smartplymouth"),
		RedisURL:          getEnv("REDIS_URL", "redis://localhost:6379/0"),
		SecretKey:         getEnv("SECRET_KEY", "change-me-in-production"),
		NscaleBaseURL:     getEnv("NSCALE_BASE_URL", "https://inference.api.nscale.com/v1"),
		NscaleToken:       getEnv("NSCALE_TOKEN", ""),
		LLMModel:          getEnv("LLM_MODEL", "Qwen/Qwen3-32B"),
		EmbeddingModel:    getEnv("EMBEDDING_MODEL", "Qwen/Qwen3-Embedding-8B"),
		PolicyVectorDir:   getEnv("POLICY_VECTORSTORE_DIR", "data/policy_vectorstore"),
		WorkerConcurrency: concurrency,
	}
}

// RedisAddr returns host:port for use with asynq (strips scheme and db).
func (c *Config) RedisAddr() string {
	u, err := url.Parse(c.RedisURL)
	if err != nil {
		return "localhost:6379"
	}
	host := u.Host
	if host == "" {
		return "localhost:6379"
	}
	return host
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
