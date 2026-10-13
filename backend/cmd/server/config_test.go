package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/conf"

	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/env"
	"github.com/go-kratos/kratos/v3/config/file"
)

func TestSecureCookieConfigFileDefaultAndEnvironmentOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("auth:\n  secure_cookie: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	load := func(t *testing.T) bool {
		t.Helper()
		return loadBootstrap(t, path).GetAuth().GetSecureCookie()
	}

	t.Run("file default", func(t *testing.T) {
		unsetEnvForTest(t, "AUTH_SECURE_COOKIE")
		unsetEnvForTest(t, "KRATOS_AUTH_SECURE_COOKIE")
		if load(t) {
			t.Fatal("secure_cookie = true, want false")
		}
	})
	t.Run("prefixed environment override", func(t *testing.T) {
		unsetEnvForTest(t, "AUTH_SECURE_COOKIE")
		t.Setenv("KRATOS_AUTH_SECURE_COOKIE", "true")
		if !load(t) {
			t.Fatal("secure_cookie = false, want KRATOS_AUTH_SECURE_COOKIE=true override")
		}
	})
	t.Run("explicit false override", func(t *testing.T) {
		t.Setenv("KRATOS_AUTH_SECURE_COOKIE", "false")
		if load(t) {
			t.Fatal("secure_cookie = true, want KRATOS_AUTH_SECURE_COOKIE=false override")
		}
	})
	t.Run("invalid override fails fast", func(t *testing.T) {
		t.Setenv("KRATOS_AUTH_SECURE_COOKIE", "definitely")
		c := newTestConfig(path)
		defer func() { _ = c.Close() }()
		if err := c.Load(); err != nil {
			t.Fatal(err)
		}
		if _, err := scanBootstrap(c); err == nil || !strings.Contains(err.Error(), "KRATOS_AUTH_SECURE_COOKIE must be a boolean") {
			t.Fatalf("scanBootstrap() error = %v", err)
		}
	})
}

func TestCommittedConfigTemplatesKeepTypedAndStringValues(t *testing.T) {
	t.Setenv("KRATOS_AUTH_SECURE_COOKIE", "true")
	t.Setenv("KRATOS_DATABASE_SOURCE", "fixture:password@tcp(mysql:3306)/blog?parseTime=True")
	t.Setenv("KRATOS_JWT_SECRET", "12345")
	t.Setenv("KRATOS_EMBEDDING_BASE_URL", "https://embedding.example/v1")
	t.Setenv("KRATOS_EMBEDDING_API_KEY", "fixture-embedding-key")
	t.Setenv("KRATOS_EMBEDDING_MODEL", "fixture-embedding-model")
	t.Setenv("KRATOS_EMBEDDING_DIMENSIONS", "1024")
	t.Setenv("KRATOS_LLM_BASE_URL", "https://llm.example/v1")
	t.Setenv("KRATOS_LLM_API_KEY", "fixture-llm-key")
	t.Setenv("KRATOS_LLM_MODEL", "true")
	t.Setenv("KRATOS_LLM_REASONING_EFFORT", "none")

	tests := []struct {
		name    string
		path    string
		wantLLM bool
	}{
		{name: "development", path: filepath.Join("..", "..", "configs", "config.yaml")},
		{name: "production", path: filepath.Join("..", "..", "..", "deploy", "prod-conf", "config.yaml"), wantLLM: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := loadBootstrap(t, tt.path)
			if !bc.GetAuth().GetSecureCookie() {
				t.Fatal("secure_cookie = false, want environment override true")
			}
			if got := bc.GetData().GetDatabase().GetSource(); got != "fixture:password@tcp(mysql:3306)/blog?parseTime=True" {
				t.Fatalf("database source = %q", got)
			}
			if got := bc.GetAuth().GetJwt().GetSecret(); got != "12345" {
				t.Fatalf("JWT secret = %q", got)
			}
			if got := bc.GetAuth().GetRateLimit().GetLoginWindow().AsDuration(); got != 5*time.Minute {
				t.Fatalf("login window = %v, want 5m", got)
			}
			if got := bc.GetEmbedding().GetBaseUrl(); got != "https://embedding.example/v1" {
				t.Fatalf("embedding base URL = %q", got)
			}
			if got := bc.GetEmbedding().GetDimensions(); got != 1024 {
				t.Fatalf("embedding dimensions = %d, want 1024", got)
			}
			if tt.wantLLM {
				if got := bc.GetLlm().GetBaseUrl(); got != "https://llm.example/v1" {
					t.Fatalf("LLM base URL = %q", got)
				}
				if got := bc.GetLlm().GetModel(); got != "true" {
					t.Fatalf("LLM model = %q", got)
				}
				if got := bc.GetLlm().GetReasoningEffort(); got != "none" {
					t.Fatalf("LLM reasoning effort = %q", got)
				}
			}
		})
	}
}

func loadBootstrap(t *testing.T, path string) *conf.Bootstrap {
	t.Helper()
	c := newTestConfig(path)
	defer func() { _ = c.Close() }()
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	bc, err := scanBootstrap(c)
	if err != nil {
		t.Fatal(err)
	}
	return bc
}

func newTestConfig(path string) config.Config {
	return config.New(config.WithSource(file.NewSource(path), env.NewSource("KRATOS")))
}

func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()
	value, exists := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if exists {
			_ = os.Setenv(key, value)
			return
		}
		_ = os.Unsetenv(key)
	})
}
