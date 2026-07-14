package config

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		envVal   string
		fallback string
		want     string
	}{
		{
			name:     "returns env value when set",
			key:      "TEST_CONFIG_VAR",
			envVal:   "custom-value",
			fallback: "default-value",
			want:     "custom-value",
		},
		{
			name:     "returns fallback when env not set",
			key:      "TEST_CONFIG_VAR_MISSING",
			envVal:   "",
			fallback: "default-value",
			want:     "default-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				os.Setenv(tt.key, tt.envVal)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}

			got := getEnv(tt.key, tt.fallback)
			if got != tt.want {
				t.Errorf("getEnv(%q, %q) = %q, want %q", tt.key, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	// Clear environment to test defaults
	envVars := []string{
		"DATABASE_URL", "REDIS_URL", "SECRET_KEY",
		"NSCALE_BASE_URL", "NSCALE_TOKEN", "LLM_MODEL",
		"EMBEDDING_MODEL", "POLICY_VECTORSTORE_DIR", "WORKER_CONCURRENCY",
	}
	for _, v := range envVars {
		os.Unsetenv(v)
	}

	cfg := Load()

	if cfg.DatabaseURL != "postgresql://postgres:postgres@localhost:5432/smartplymouth" {
		t.Errorf("unexpected DatabaseURL: %s", cfg.DatabaseURL)
	}
	if cfg.RedisURL != "redis://localhost:6379/0" {
		t.Errorf("unexpected RedisURL: %s", cfg.RedisURL)
	}
	if cfg.SecretKey != "change-me-in-production" {
		t.Errorf("unexpected SecretKey: %s", cfg.SecretKey)
	}
	if cfg.NscaleBaseURL != "https://inference.api.nscale.com/v1" {
		t.Errorf("unexpected NscaleBaseURL: %s", cfg.NscaleBaseURL)
	}
	if cfg.NscaleToken != "" {
		t.Errorf("unexpected NscaleToken: %s", cfg.NscaleToken)
	}
	if cfg.LLMModel != "Qwen/Qwen3-32B" {
		t.Errorf("unexpected LLMModel: %s", cfg.LLMModel)
	}
	if cfg.EmbeddingModel != "Qwen/Qwen3-Embedding-8B" {
		t.Errorf("unexpected EmbeddingModel: %s", cfg.EmbeddingModel)
	}
	if cfg.PolicyVectorDir != "data/policy_vectorstore" {
		t.Errorf("unexpected PolicyVectorDir: %s", cfg.PolicyVectorDir)
	}
	if cfg.WorkerConcurrency != 4 {
		t.Errorf("unexpected WorkerConcurrency: %d", cfg.WorkerConcurrency)
	}
}

func TestLoadWithCustomEnv(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgresql://user:pass@db:5432/test")
	os.Setenv("REDIS_URL", "redis://redis:6379/1")
	os.Setenv("WORKER_CONCURRENCY", "8")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("REDIS_URL")
		os.Unsetenv("WORKER_CONCURRENCY")
	}()

	cfg := Load()

	if cfg.DatabaseURL != "postgresql://user:pass@db:5432/test" {
		t.Errorf("unexpected DatabaseURL: %s", cfg.DatabaseURL)
	}
	if cfg.RedisURL != "redis://redis:6379/1" {
		t.Errorf("unexpected RedisURL: %s", cfg.RedisURL)
	}
	if cfg.WorkerConcurrency != 8 {
		t.Errorf("unexpected WorkerConcurrency: %d", cfg.WorkerConcurrency)
	}
}

func TestLoadWithInvalidConcurrency(t *testing.T) {
	os.Setenv("WORKER_CONCURRENCY", "not-a-number")
	defer os.Unsetenv("WORKER_CONCURRENCY")

	cfg := Load()
	if cfg.WorkerConcurrency != 4 {
		t.Errorf("expected default concurrency 4, got %d", cfg.WorkerConcurrency)
	}
}

func TestLoadWithZeroConcurrency(t *testing.T) {
	os.Setenv("WORKER_CONCURRENCY", "0")
	defer os.Unsetenv("WORKER_CONCURRENCY")

	cfg := Load()
	if cfg.WorkerConcurrency != 4 {
		t.Errorf("expected default concurrency 4 for zero value, got %d", cfg.WorkerConcurrency)
	}
}

func TestRedisAddr(t *testing.T) {
	tests := []struct {
		name     string
		redisURL string
		want     string
	}{
		{
			name:     "standard redis URL",
			redisURL: "redis://localhost:6379/0",
			want:     "localhost:6379",
		},
		{
			name:     "redis with custom host and port",
			redisURL: "redis://myredis:6380/1",
			want:     "myredis:6380",
		},
		{
			name:     "invalid URL returns default",
			redisURL: "not-a-url",
			want:     "localhost:6379",
		},
		{
			name:     "empty host returns default",
			redisURL: "redis:///0",
			want:     "localhost:6379",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{RedisURL: tt.redisURL}
			got := cfg.RedisAddr()
			if got != tt.want {
				t.Errorf("RedisAddr() = %q, want %q", got, tt.want)
			}
		})
	}
}
