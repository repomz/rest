package generator

import (
	"fmt"
	"strings"
)

func BuildDeploymentGuideSource(features FeatureOptions) string {
	var b strings.Builder
	backend := features.Build.Backend
	if backend == "" {
		backend = "sql"
	}
	fmt.Fprintln(&b, "# Deployment Guide")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "This guide was generated for this application configuration. Use it as a practical checklist for local verification, local run, and production preparation.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## 1. Verify The Generated Project")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Run the generator diagnostics from the project root:")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "```bash")
	fmt.Fprintln(&b, "rest doctor")
	fmt.Fprintln(&b, "```")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Then run Go checks:")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "```bash")
	fmt.Fprintln(&b, "go test ./...")
	fmt.Fprintln(&b, "go build ./cmd")
	fmt.Fprintln(&b, "```")
	if features.OpenAPI.Enabled {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "The OpenAPI specification is generated at:")
		fmt.Fprintf(&b, "\n```text\n%s\n```\n", defaultString(features.OpenAPI.Output, "docs/swagger.yaml"))
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## 2. Configure Environment")
	fmt.Fprintln(&b)
	if features.Build.Env {
		fmt.Fprintf(&b, "Start from `%s` and provide real local values:\n", defaultString(features.Build.EnvPath, ".env.example"))
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "```bash")
		fmt.Fprintf(&b, "cp %s .env\n", defaultString(features.Build.EnvPath, ".env.example"))
		fmt.Fprintln(&b, "```")
	} else {
		fmt.Fprintln(&b, "Create environment variables manually before running the application.")
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Required runtime values:")
	if backend == "mongo" {
		fmt.Fprintf(&b, "- `%s`: MongoDB connection URI.\n", defaultString(features.Mongo.URIEnv, "MONGO_URI"))
		if features.Mongo.Database != "" {
			fmt.Fprintf(&b, "- MongoDB database: `%s`.\n", features.Mongo.Database)
		}
	} else {
		fmt.Fprintln(&b, "- `DB_DSN`: PostgreSQL connection string.")
		if features.Build.DBName != "" {
			fmt.Fprintf(&b, "- Database name: `%s`.\n", features.Build.DBName)
		}
		if features.Build.DBUser != "" {
			fmt.Fprintf(&b, "- Database user: `%s`.\n", features.Build.DBUser)
		}
	}
	if features.Auth.Enabled {
		switch strings.ToLower(features.Auth.Strategy) {
		case "jwt":
			fmt.Fprintf(&b, "- `%s`: JWT signing secret for HS256 projects.\n", defaultString(features.Auth.JWTSecretEnv, "JWT_SIGNING_KEY"))
		case "basic":
			fmt.Fprintf(&b, "- `%s`: Basic Auth username.\n", defaultString(features.Auth.BasicUsernameEnv, "BASIC_AUTH_USERNAME"))
			fmt.Fprintf(&b, "- `%s`: Basic Auth password.\n", defaultString(features.Auth.BasicPasswordEnv, "BASIC_AUTH_PASSWORD"))
		}
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## 3. Run Locally")
	fmt.Fprintln(&b)
	if features.Build.Makefile {
		fmt.Fprintln(&b, "If the generated Makefile is enabled:")
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "```bash")
		fmt.Fprintln(&b, "make run")
		fmt.Fprintln(&b, "```")
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Otherwise run the application directly:")
	} else {
		fmt.Fprintln(&b, "Run the application directly:")
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "```bash")
	fmt.Fprintln(&b, "go run ./cmd")
	fmt.Fprintln(&b, "```")
	if features.HTTP.Health {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Check the health endpoint:")
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "```bash")
		fmt.Fprintf(&b, "curl -fsS http://localhost:%d%s\n", nonZero(features.HTTP.Port, features.Build.HTTPPort, 8080), routePath(features.HTTP.BasePath, features.HTTP.HealthPath))
		fmt.Fprintln(&b, "```")
	}
	if features.OpenAPI.Enabled && features.OpenAPI.WithUI {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Open Swagger UI:")
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "```text")
		fmt.Fprintf(&b, "http://localhost:%d%s\n", nonZero(features.HTTP.Port, features.Build.HTTPPort, 8080), routePath(features.HTTP.BasePath, features.OpenAPI.UIPath))
		fmt.Fprintln(&b, "```")
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## 4. Production Preparation")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Before production deployment:")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "- Run `rest doctor` and fix warnings/errors that apply to your deployment.")
	fmt.Fprintln(&b, "- Run `go test ./...` in CI.")
	fmt.Fprintln(&b, "- Provide secrets through your deployment platform, not committed files.")
	fmt.Fprintln(&b, "- Use explicit CORS origins for production clients.")
	fmt.Fprintln(&b, "- Run behind HTTPS or a trusted reverse proxy.")
	if features.HTTP.SecurityHeaders {
		fmt.Fprintln(&b, "- Review generated security headers, especially CSP and HSTS, for your domain.")
	}
	if features.HTTP.RateLimit {
		fmt.Fprintln(&b, "- Tune rate limiting for real traffic and deployment topology.")
	}
	if features.OpenAPI.Enabled {
		fmt.Fprintln(&b, "- Review generated OpenAPI before exposing the API to other teams or clients.")
	}
	if backend == "sql" && features.Build.InitMigration {
		fmt.Fprintf(&b, "- Apply database migrations from `%s` before serving traffic.\n", defaultString(features.Build.MigrationsPath, "internal/sql/migrations"))
	}
	if backend == "mongo" {
		fmt.Fprintln(&b, "- Ensure MongoDB indexes and credentials are configured for the target environment.")
	}
	fmt.Fprintln(&b)
	if features.Docker.Enabled {
		fmt.Fprintln(&b, "Build the production image:")
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "```bash")
		fmt.Fprintf(&b, "docker build -t myapp:latest -f %s .\n", defaultString(features.Docker.Output, "Dockerfile"))
		fmt.Fprintln(&b, "```")
	}
	if features.Docker.Compose {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "For a local container smoke test:")
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "```bash")
		fmt.Fprintf(&b, "docker compose -f %s up --build\n", defaultString(features.Docker.ComposeOutput, "docker-compose.yml"))
		fmt.Fprintln(&b, "```")
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## 5. What To Build Next")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "The generated app is a strong starting point. Continue by adding domain-specific validation, business workflows, integrations, and deployment-specific observability.")
	return b.String()
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func nonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func routePath(base, path string) string {
	base = strings.TrimSpace(base)
	path = strings.TrimSpace(path)
	if base == "" || base == "/" {
		if path == "" {
			return "/"
		}
		if strings.HasPrefix(path, "/") {
			return path
		}
		return "/" + path
	}
	base = "/" + strings.Trim(strings.TrimPrefix(base, "/"), "/")
	if path == "" || path == "/" {
		return base
	}
	return base + "/" + strings.TrimPrefix(path, "/")
}
