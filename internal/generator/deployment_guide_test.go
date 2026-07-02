package generator

import (
	"strings"
	"testing"
)

func TestBuildDeploymentGuideSourceForSQL(t *testing.T) {
	guide := BuildDeploymentGuideSource(FeatureOptions{
		Build: BuildFeatures{
			Backend:         "sql",
			Env:             true,
			EnvPath:         ".env.example",
			Makefile:        true,
			InitMigration:   true,
			MigrationsPath:  "internal/sql/migrations",
			DBName:          "myapp_db",
			DBUser:          "app_user",
			DeploymentGuide: true,
		},
		HTTP:    HTTPFeatures{Port: 8080, BasePath: "/", Health: true, HealthPath: "/health", SecurityHeaders: true, RateLimit: true},
		Auth:    AuthFeatures{Enabled: true, Strategy: "jwt", JWTSecretEnv: "JWT_SIGNING_KEY"},
		OpenAPI: OpenAPIFeatures{Enabled: true, WithUI: true, Output: "docs/swagger.yaml", UIPath: "/swagger/index.html"},
		Docker:  DockerFeatures{Enabled: true, Output: "Dockerfile"},
	})
	for _, want := range []string{
		"# Deployment Guide",
		"rest doctor",
		"cp .env.example .env",
		"`DB_DSN`",
		"`JWT_SIGNING_KEY`",
		"curl -fsS http://localhost:8080/health",
		"docker build -t myapp:latest -f Dockerfile .",
		"Apply database migrations from `internal/sql/migrations`",
	} {
		if !strings.Contains(guide, want) {
			t.Fatalf("SQL deployment guide missing %q:\n%s", want, guide)
		}
	}
}

func TestBuildDeploymentGuideSourceForMongo(t *testing.T) {
	guide := BuildDeploymentGuideSource(FeatureOptions{
		Build: BuildFeatures{Backend: "mongo", DeploymentGuide: true},
		HTTP:  HTTPFeatures{Port: 8080, BasePath: "/api", Health: true, HealthPath: "/health"},
		Auth:  AuthFeatures{Enabled: true, Strategy: "basic", BasicUsernameEnv: "BASIC_AUTH_USERNAME", BasicPasswordEnv: "BASIC_AUTH_PASSWORD"},
		Mongo: MongoFeatures{URIEnv: "MONGO_URI", Database: "myapp_db"},
		Docker: DockerFeatures{
			Compose:       true,
			ComposeOutput: "docker-compose.yml",
		},
	})
	for _, want := range []string{
		"`MONGO_URI`",
		"MongoDB database: `myapp_db`",
		"`BASIC_AUTH_USERNAME`",
		"`BASIC_AUTH_PASSWORD`",
		"curl -fsS http://localhost:8080/api/health",
		"docker compose -f docker-compose.yml up --build",
		"Ensure MongoDB indexes and credentials",
	} {
		if !strings.Contains(guide, want) {
			t.Fatalf("Mongo deployment guide missing %q:\n%s", want, guide)
		}
	}
}
