package appgen

import (
	"strings"
	"testing"

	"github.com/repomz/rest/internal/config"
)

func TestValidateConfigRequiresGracefulShutdown(t *testing.T) {
	bundle := minimalBundle()
	bundle.Rest.HTTP.GracefulShutdown.Enabled = config.Enabled(false)

	err := validateConfig(bundle)
	if err == nil || !strings.Contains(err.Error(), "http.graceful_shutdown.enabled") {
		t.Fatalf("expected graceful shutdown validation error, got %v", err)
	}
}

func TestResolveSQLCPathUsesConfigDir(t *testing.T) {
	got := resolveSQLCPath("/project/rest_config", "../sqlc/sqlc.yaml")
	want := "/project/sqlc/sqlc.yaml"
	if got != want {
		t.Fatalf("sqlc path = %q, want %q", got, want)
	}
}

func minimalBundle() config.Bundle {
	return config.Bundle{
		Rest: config.Rest{
			Language: "go",
			HTTP: config.HTTP{
				Framework:        "std",
				Port:             8080,
				BasePath:         "/",
				GracefulShutdown: config.GeneratedSwitch{Enabled: config.Enabled(true)},
				Health:           config.Health{Path: "/health"},
				Middleware: config.Middleware{
					CORS: config.CORS{MaxAge: "12h"},
				},
			},
			Logging: config.Logging{
				Enabled: config.Enabled(false),
				Output:  config.LoggingOutput{Type: "stdout"},
			},
			OpenAPI: config.OpenAPI{
				SpecPath: "/swagger/openapi.yaml",
				UIPath:   "/swagger/index.html",
			},
			Observability: config.Observability{
				Metrics: config.Metrics{Path: "/metrics"},
			},
		},
	}
}
