package generator

import (
	"strings"
	"testing"
)

func TestBuildArchitectureSourceReflectsSQLApp(t *testing.T) {
	features := FeatureOptions{
		HTTP: HTTPFeatures{
			Port:          9090,
			Health:        true,
			HealthPath:    "/health",
			Readiness:     true,
			ReadinessPath: "/ready",
		},
		OpenAPI: OpenAPIFeatures{Enabled: true, WithUI: true, Output: "docs/swagger.yaml", UIPath: "/swagger/index.html"},
		Build:   BuildFeatures{Backend: "sql", Curl: true},
		Auth:    AuthFeatures{Enabled: true, Strategy: "jwt"},
	}
	doc := BuildArchitectureSource("github.com/example/clinic", []table{{
		Name:      "studies",
		GoName:    "Study",
		RouteBase: "/studies",
		Queries:   querySet{GetAll: true, Create: true},
	}}, features)
	for _, expected := range []string{
		"# Architecture",
		"clinic/",
		"pgrepo/",
		"Backend: `sql`",
		"Swagger UI: `http://localhost:9090/swagger/index.html`",
		"`Study` → `/studies`",
		"PostgreSQL repository → SQLC queries → PostgreSQL",
	} {
		if !strings.Contains(doc, expected) {
			t.Fatalf("architecture doc does not contain %q:\n%s", expected, doc)
		}
	}
}

func TestBuildReadmeSourceReflectsMongoApp(t *testing.T) {
	features := FeatureOptions{
		HTTP: HTTPFeatures{Port: 8088, Health: true, HealthPath: "/health"},
		OpenAPI: OpenAPIFeatures{
			Enabled: true,
			WithUI:  true,
			Output:  "docs/swagger.yaml",
			UIPath:  "/swagger/index.html",
		},
		Build: BuildFeatures{
			Backend:          "mongo",
			Makefile:         true,
			Env:              true,
			EnvPath:          ".env.example",
			Architecture:     true,
			ArchitecturePath: "ARCHITECTURE.md",
		},
		Docker: DockerFeatures{Enabled: true, Output: "Dockerfile", Compose: true, ComposeOutput: "docker-compose.yml"},
		Mongo:  MongoFeatures{Models: []MongoModel{{Name: "Item", Collection: "items", Methods: []MongoMethod{{Name: "SearchItems"}}}}},
	}
	doc := BuildReadmeSource("github.com/example/store", nil, features)
	for _, expected := range []string{
		"# store",
		"generated%20by-rest",
		"Backend: `mongo`",
		"Swagger UI at `http://localhost:8088/swagger/index.html`",
		"rest doctor",
		"make run",
		"docker compose -f docker-compose.yml up --build",
		"`Item` → collection `items`",
		"See `ARCHITECTURE.md`",
	} {
		if !strings.Contains(doc, expected) {
			t.Fatalf("README doc does not contain %q:\n%s", expected, doc)
		}
	}
}
