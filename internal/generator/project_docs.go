package generator

import (
	"fmt"
	"path"
	"strings"
)

func BuildArchitectureSource(module string, tables []table, features FeatureOptions) string {
	backend := generatedBackend(features)
	var b strings.Builder
	fmt.Fprintln(&b, "# Architecture")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Generated architecture for `%s`.\n", defaultString(module, "generated-rest-app"))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "```text")
	fmt.Fprintf(&b, "%s/\n", projectName(module))
	fmt.Fprintln(&b, "├── cmd/")
	fmt.Fprintln(&b, "│   └── main.go                         # application entrypoint")
	fmt.Fprintln(&b, "├── internal/")
	fmt.Fprintln(&b, "│   └── app/")
	fmt.Fprintln(&b, "│       ├── common/                     # shared HTTP responses and errors")
	fmt.Fprintln(&b, "│       ├── config/                     # environment-based runtime config")
	fmt.Fprintln(&b, "│       ├── domain/                     # generated domain models")
	fmt.Fprintln(&b, "│       ├── repository/")
	if backend == "mongo" {
		fmt.Fprintln(&b, "│       │   └── mongorepo/              # MongoDB repositories")
	} else {
		fmt.Fprintln(&b, "│       │   └── pgrepo/                 # PostgreSQL repositories over SQLC")
	}
	fmt.Fprintln(&b, "│       ├── services/                   # business seam for generated operations")
	fmt.Fprintln(&b, "│       ├── transport/")
	fmt.Fprintln(&b, "│       │   ├── httpmodels/             # request/response DTOs")
	fmt.Fprintln(&b, "│       │   ├── httpserver/             # handlers, routes, auth, swagger")
	fmt.Fprintln(&b, "│       │   └── middleware/             # HTTP middleware when enabled")
	if features.Logging.Enabled {
		fmt.Fprintln(&b, "│       ├── logging/                    # zap logger setup")
	}
	if features.Metrics.Enabled {
		fmt.Fprintln(&b, "│       └── metrics/                    # Prometheus metrics")
	} else {
		fmt.Fprintln(&b, "│       └── metrics/                    # generated when metrics are enabled")
	}
	if features.OpenAPI.Enabled {
		fmt.Fprintf(&b, "├── %s                    # OpenAPI specification\n", defaultString(features.OpenAPI.Output, "docs/swagger.yaml"))
	}
	if features.Build.Curl {
		fmt.Fprintln(&b, "├── curl/                              # curl examples")
	}
	if features.Docker.Enabled {
		fmt.Fprintf(&b, "├── %s                         # container image build\n", defaultString(features.Docker.Output, "Dockerfile"))
	}
	if features.Docker.Compose {
		fmt.Fprintf(&b, "├── %s                # local container stack\n", defaultString(features.Docker.ComposeOutput, "docker-compose.yml"))
	}
	if features.Build.Env {
		fmt.Fprintf(&b, "├── %s                      # local environment template\n", defaultString(features.Build.EnvPath, ".env.example"))
	}
	if features.Build.Makefile {
		fmt.Fprintf(&b, "├── %s                          # local developer commands\n", defaultString(features.Build.MakefilePath, "Makefile"))
	}
	fmt.Fprintln(&b, "├── go.mod")
	fmt.Fprintln(&b, "└── go.sum")
	fmt.Fprintln(&b, "```")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Runtime Shape")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Backend: `%s`.\n", backend)
	fmt.Fprintf(&b, "- HTTP address: `%s:%d`.\n", defaultString(features.HTTP.Host, "0.0.0.0"), nonZero(features.HTTP.Port, features.Build.HTTPPort, 8080))
	if features.HTTP.BasePath != "" && features.HTTP.BasePath != "/" {
		fmt.Fprintf(&b, "- Base path: `%s`.\n", features.HTTP.BasePath)
	}
	if features.HTTP.Health {
		fmt.Fprintf(&b, "- Health endpoint: `%s`.\n", routePath(features.HTTP.BasePath, defaultString(features.HTTP.HealthPath, "/health")))
	}
	if features.HTTP.Readiness {
		fmt.Fprintf(&b, "- Readiness endpoint: `%s`.\n", routePath(features.HTTP.BasePath, defaultString(features.HTTP.ReadinessPath, "/ready")))
	}
	if features.OpenAPI.Enabled {
		fmt.Fprintf(&b, "- OpenAPI file: `%s`.\n", defaultString(features.OpenAPI.Output, "docs/swagger.yaml"))
		if features.OpenAPI.WithUI {
			fmt.Fprintf(&b, "- Swagger UI: `http://localhost:%d%s`.\n", nonZero(features.HTTP.Port, features.Build.HTTPPort, 8080), routePath(features.HTTP.BasePath, defaultString(features.OpenAPI.UIPath, "/swagger/index.html")))
		}
	}
	if features.Auth.Enabled {
		fmt.Fprintf(&b, "- Auth strategy: `%s`.\n", defaultString(features.Auth.Strategy, "jwt"))
	}
	if features.Metrics.Enabled {
		fmt.Fprintf(&b, "- Metrics endpoint: `%s`.\n", routePath(features.HTTP.BasePath, defaultString(features.Metrics.Path, "/metrics")))
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Generated Domains")
	fmt.Fprintln(&b)
	if backend == "mongo" {
		writeMongoDomains(&b, features.Mongo.Models)
	} else {
		writeSQLDomains(&b, tables)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Request Flow")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "```text")
	fmt.Fprintln(&b, "client")
	fmt.Fprintln(&b, "  │")
	fmt.Fprintln(&b, "  ▼")
	fmt.Fprintln(&b, "HTTP middleware: request id, recovery, CORS, security headers, rate limit, metrics")
	fmt.Fprintln(&b, "  │")
	fmt.Fprintln(&b, "  ▼")
	fmt.Fprintln(&b, "HTTP handler")
	fmt.Fprintln(&b, "  │")
	fmt.Fprintln(&b, "  ▼")
	fmt.Fprintln(&b, "service")
	fmt.Fprintln(&b, "  │")
	fmt.Fprintln(&b, "  ▼")
	if backend == "mongo" {
		fmt.Fprintln(&b, "MongoDB repository → MongoDB")
	} else {
		fmt.Fprintln(&b, "PostgreSQL repository → SQLC queries → PostgreSQL")
	}
	fmt.Fprintln(&b, "```")
	return b.String()
}

func BuildReadmeSource(module string, tables []table, features FeatureOptions) string {
	backend := generatedBackend(features)
	port := nonZero(features.HTTP.Port, features.Build.HTTPPort, 8080)
	name := projectName(module)
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n", name)
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "%s\n", testsBadge(module, features))
	fmt.Fprintln(&b, "![Generated by rest](https://img.shields.io/badge/generated%20by-rest-6f42c1)")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Generated layered Go REST application.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## What Is Inside")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Backend: `%s`.\n", backend)
	fmt.Fprintln(&b, "- Layers: HTTP handlers, services, repositories, domain models, runtime config.")
	if features.OpenAPI.Enabled {
		fmt.Fprintf(&b, "- OpenAPI: `%s`", defaultString(features.OpenAPI.Output, "docs/swagger.yaml"))
		if features.OpenAPI.WithUI {
			fmt.Fprintf(&b, " and Swagger UI at `http://localhost:%d%s`", port, routePath(features.HTTP.BasePath, defaultString(features.OpenAPI.UIPath, "/swagger/index.html")))
		}
		fmt.Fprintln(&b, ".")
	}
	if features.Auth.Enabled {
		fmt.Fprintf(&b, "- Auth: `%s` with generated route protection.\n", defaultString(features.Auth.Strategy, "jwt"))
	}
	if features.HTTP.Health {
		fmt.Fprintf(&b, "- Health: `http://localhost:%d%s`.\n", port, routePath(features.HTTP.BasePath, defaultString(features.HTTP.HealthPath, "/health")))
	}
	if features.HTTP.Readiness {
		fmt.Fprintf(&b, "- Readiness: `http://localhost:%d%s`.\n", port, routePath(features.HTTP.BasePath, defaultString(features.HTTP.ReadinessPath, "/ready")))
	}
	if features.Metrics.Enabled {
		fmt.Fprintf(&b, "- Metrics: `http://localhost:%d%s`.\n", port, routePath(features.HTTP.BasePath, defaultString(features.Metrics.Path, "/metrics")))
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Quick Start")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Check the generated project first:")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "```bash")
	fmt.Fprintln(&b, "rest doctor")
	fmt.Fprintln(&b, "go test ./...")
	fmt.Fprintln(&b, "```")
	fmt.Fprintln(&b)
	if features.Build.Env {
		fmt.Fprintln(&b, "Create a local environment file:")
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "```bash")
		fmt.Fprintf(&b, "cp %s .env\n", defaultString(features.Build.EnvPath, ".env.example"))
		fmt.Fprintln(&b, "```")
		fmt.Fprintln(&b)
	}
	fmt.Fprintln(&b, "Run the application:")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "```bash")
	if features.Build.Makefile {
		fmt.Fprintln(&b, "make run")
	} else {
		fmt.Fprintln(&b, "go run ./cmd")
	}
	fmt.Fprintln(&b, "```")
	if features.Docker.Enabled {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Build a Docker image:")
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "```bash")
		fmt.Fprintf(&b, "docker build -t %s:local -f %s .\n", strings.ToLower(name), defaultString(features.Docker.Output, "Dockerfile"))
		fmt.Fprintln(&b, "```")
	}
	if features.Docker.Compose {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Run with Docker Compose:")
		if backend == "sql" {
			fmt.Fprintln(&b, "The one-shot `migrate` service applies pending migrations before the application starts.")
		} else {
			fmt.Fprintln(&b, "The one-shot `mongo-init` service prepares application credentials before the application starts.")
		}
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "```bash")
		fmt.Fprintf(&b, "docker compose -f %s up --build\n", defaultString(features.Docker.ComposeOutput, "docker-compose.yml"))
		fmt.Fprintln(&b, "```")
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Generated Domains")
	fmt.Fprintln(&b)
	if backend == "mongo" {
		writeMongoDomains(&b, features.Mongo.Models)
	} else {
		writeSQLDomains(&b, tables)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Project Architecture")
	fmt.Fprintln(&b)
	architecturePath := defaultString(features.Build.ArchitecturePath, "ARCHITECTURE.md")
	if features.Build.Architecture {
		fmt.Fprintf(&b, "See `%s` for the current generated architecture map.\n", architecturePath)
	} else {
		fmt.Fprintln(&b, "The application follows the standard generated layout: handlers → services → repositories → database.")
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Regeneration")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "When contracts or generator config change, run:")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "```bash")
	fmt.Fprintln(&b, "rest gen")
	fmt.Fprintln(&b, "```")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "If `safe_reload` is enabled, the generator asks before overwriting user-modified generated files.")
	return b.String()
}

func writeSQLDomains(b *strings.Builder, tables []table) {
	if len(tables) == 0 {
		fmt.Fprintln(b, "- No SQL tables were discovered.")
		return
	}
	for _, tbl := range tables {
		endpointCount := len(tbl.Endpoints)
		if endpointCount == 0 {
			endpointCount = sqlCRUDEndpointCount(tbl)
		}
		fmt.Fprintf(b, "- `%s` → `%s` (%d endpoint(s)).\n", tbl.GoName, tbl.RouteBase, endpointCount)
	}
}

func writeMongoDomains(b *strings.Builder, models []MongoModel) {
	count := 0
	for _, model := range models {
		if model.Embedded || model.Collection == "" {
			continue
		}
		count++
		methodCount := len(model.Methods)
		if methodCount == 0 {
			methodCount = 5
		}
		fmt.Fprintf(b, "- `%s` → collection `%s` (%d method(s)).\n", model.Name, model.Collection, methodCount)
	}
	if count == 0 {
		fmt.Fprintln(b, "- No active MongoDB models were discovered.")
	}
}

func sqlCRUDEndpointCount(tbl table) int {
	count := 0
	if tbl.Queries.GetAll {
		count++
	}
	if tbl.Queries.Create {
		count++
	}
	if tbl.Queries.DeleteAll {
		count++
	}
	if tbl.Queries.GetByID {
		count++
	}
	if tbl.Queries.Delete {
		count++
	}
	return count
}

func generatedBackend(features FeatureOptions) string {
	if strings.EqualFold(features.Build.Backend, "mongo") {
		return "mongo"
	}
	return "sql"
}

func projectName(module string) string {
	module = strings.TrimSpace(module)
	if module == "" {
		return "generated-rest-app"
	}
	name := path.Base(strings.TrimSuffix(module, "/"))
	if name == "." || name == "/" || name == "" {
		return "generated-rest-app"
	}
	return name
}

func testsBadge(module string, features FeatureOptions) string {
	workflow := defaultString(features.Build.CIPath, ".github/workflows/ci.yaml")
	if features.Build.CI && strings.HasPrefix(module, "github.com/") {
		repo := strings.TrimPrefix(module, "github.com/")
		workflowName := path.Base(workflow)
		return fmt.Sprintf("[![Tests](https://github.com/%s/actions/workflows/%s/badge.svg)](https://github.com/%s/actions/workflows/%s)", repo, workflowName, repo, workflowName)
	}
	return "![Tests](https://img.shields.io/badge/tests-go%20test-blue)"
}
