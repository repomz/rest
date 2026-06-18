package appgen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/repomz/rest/internal/config"
	"github.com/repomz/rest/internal/generator"
)

type Generator struct {
	features []Feature
}

func New(features ...Feature) Generator {
	return Generator{features: features}
}

func (g Generator) Generate(configDir string) error {
	bundle, err := config.Load(configDir)
	if err != nil {
		return err
	}
	if err := validateConfig(bundle); err != nil {
		return err
	}
	ctx := NewContext(bundle)
	hasEnabledFeature := false
	for _, feature := range g.features {
		if feature.Enabled(ctx) {
			hasEnabledFeature = true
			break
		}
	}
	if !hasEnabledFeature {
		return fmt.Errorf("no implemented generation feature is enabled; set sqlc.enable to enable in sqlc_rest.yaml after preparing SQLC files")
	}
	if err := runAutoSQLC(ctx); err != nil {
		return err
	}
	if err := ensureProjectModule(ctx); err != nil {
		return err
	}
	for _, feature := range g.features {
		if !feature.Enabled(ctx) {
			continue
		}
		if err := feature.Generate(ctx); err != nil {
			return fmt.Errorf("generate feature %s: %w", feature.Name(), err)
		}
	}
	if err := runGoModTidy(ctx.ProjectDir); err != nil {
		return err
	}
	return nil
}

func validateConfig(bundle config.Bundle) error {
	if bundle.Rest.Mongo.Bool() {
		return fmt.Errorf("mongo generation is not implemented yet")
	}
	if bundle.Rest.Auth.Bool() {
		return fmt.Errorf("auth generation is not implemented yet")
	}
	if language := strings.ToLower(bundle.Rest.Language); language != "" && language != "golang" && language != "go" {
		return fmt.Errorf("unsupported language %q", bundle.Rest.Language)
	}
	if framework := strings.ToLower(bundle.Rest.HTTP.Framework); framework != "" && framework != "std" {
		return fmt.Errorf("unsupported HTTP framework %q", bundle.Rest.HTTP.Framework)
	}
	if bundle.Rest.Logging.Enabled.Bool() {
		if library := strings.ToLower(bundle.Rest.Logging.Library); library != "" && library != "zap" {
			return fmt.Errorf("unsupported logging library %q", bundle.Rest.Logging.Library)
		}
		if level := strings.ToLower(bundle.Rest.Logging.Level); level != "debug" && level != "info" && level != "warn" && level != "error" && level != "dpanic" && level != "panic" && level != "fatal" {
			return fmt.Errorf("unsupported logging level %q", bundle.Rest.Logging.Level)
		}
		if format := strings.ToLower(bundle.Rest.Logging.Format); format != "json" && format != "console" {
			return fmt.Errorf("unsupported logging format %q", bundle.Rest.Logging.Format)
		}
	}
	if bundle.Rest.HTTP.Port < 1 || bundle.Rest.HTTP.Port > 65535 {
		return fmt.Errorf("http.port must be between 1 and 65535")
	}
	if !bundle.Rest.HTTP.GracefulShutdown.Enabled.Bool() {
		return fmt.Errorf("http.graceful_shutdown.enabled supports only enable")
	}
	for name, path := range map[string]string{
		"http.base_path":             bundle.Rest.HTTP.BasePath,
		"http.health.path":           bundle.Rest.HTTP.Health.Path,
		"openapi.spec_path":          bundle.Rest.OpenAPI.SpecPath,
		"openapi.ui_path":            bundle.Rest.OpenAPI.UIPath,
		"observability.metrics.path": bundle.Rest.Observability.Metrics.Path,
	} {
		if path != "" && !strings.HasPrefix(path, "/") {
			return fmt.Errorf("%s must start with /", name)
		}
	}
	for name, value := range map[string]string{
		"http.timeouts.read_header":    bundle.Rest.HTTP.Timeouts.ReadHeader,
		"http.timeouts.read":           bundle.Rest.HTTP.Timeouts.Read,
		"http.timeouts.write":          bundle.Rest.HTTP.Timeouts.Write,
		"http.timeouts.idle":           bundle.Rest.HTTP.Timeouts.Idle,
		"http.timeouts.shutdown":       bundle.Rest.HTTP.Timeouts.Shutdown,
		"http.middleware.cors.max_age": bundle.Rest.HTTP.Middleware.CORS.MaxAge,
	} {
		if value == "" {
			continue
		}
		if _, err := time.ParseDuration(value); err != nil {
			return fmt.Errorf("%s must be a Go duration: %w", name, err)
		}
	}
	if bundle.Rest.HTTP.Middleware.CORS.Enabled.Bool() && bundle.Rest.HTTP.Middleware.CORS.AllowCredentials {
		for _, origin := range bundle.Rest.HTTP.Middleware.CORS.AllowOrigins {
			if origin == "*" {
				return fmt.Errorf("CORS allow_credentials cannot be used with wildcard origin")
			}
		}
	}
	if bundle.Rest.Logging.Enabled.Bool() && bundle.Rest.Logging.Output.Type != "stdout" && bundle.Rest.Logging.Output.Type != "stderr" && bundle.Rest.Logging.Output.Type != "file" {
		return fmt.Errorf("unsupported logging output %q", bundle.Rest.Logging.Output.Type)
	}
	if bundle.Rest.Logging.Enabled.Bool() && bundle.Rest.Logging.Output.Type == "file" && bundle.Rest.Logging.Output.File == "" {
		return fmt.Errorf("logging.output.file is required for file output")
	}
	if bundle.Rest.Logging.Rotation.Enabled.Bool() && bundle.Rest.Logging.Output.Type != "file" {
		return fmt.Errorf("logging rotation requires file output")
	}
	if bundle.Rest.Logging.Rotation.Enabled.Bool() && (bundle.Rest.Logging.Rotation.MaxSizeMB < 1 || bundle.Rest.Logging.Rotation.MaxBackups < 0 || bundle.Rest.Logging.Rotation.MaxAgeDays < 0) {
		return fmt.Errorf("logging rotation limits must be non-negative and max_size_mb must be positive")
	}
	if bundle.Rest.Observability.Metrics.Enabled.Bool() && strings.ToLower(bundle.Rest.Observability.Metrics.Provider) != "prometheus" {
		return fmt.Errorf("unsupported metrics provider %q", bundle.Rest.Observability.Metrics.Provider)
	}
	if bundle.Rest.Observability.Metrics.Enabled.Bool() && bundle.Rest.Observability.Metrics.Path == "" {
		return fmt.Errorf("observability.metrics.path is required")
	}
	for _, label := range bundle.Rest.Observability.Metrics.Labels {
		if label != "method" && label != "route" && label != "status" {
			return fmt.Errorf("unsupported metrics label %q", label)
		}
	}
	if bundle.Rest.Docker.Enabled.Bool() {
		for name, value := range map[string]string{
			"docker.healthcheck.interval": bundle.Rest.Docker.Healthcheck.Interval,
			"docker.healthcheck.timeout":  bundle.Rest.Docker.Healthcheck.Timeout,
		} {
			if value == "" {
				continue
			}
			if _, err := time.ParseDuration(value); err != nil {
				return fmt.Errorf("%s must be a duration: %w", name, err)
			}
		}
		if bundle.Rest.Docker.Port < 1 || bundle.Rest.Docker.Port > 65535 {
			return fmt.Errorf("docker.port must be between 1 and 65535")
		}
		if bundle.Rest.Docker.Healthcheck.Enabled.Bool() && !bundle.Rest.HTTP.Health.Enabled.Bool() {
			return fmt.Errorf("docker healthcheck requires http.health.enabled")
		}
		if bundle.Rest.Docker.Port != bundle.Rest.HTTP.Port {
			return fmt.Errorf("docker.port must match http.port")
		}
		for name, value := range map[string]string{
			"docker.build_image":         bundle.Rest.Docker.BuildImage,
			"docker.runtime_image":       bundle.Rest.Docker.RuntimeImage,
			"docker.binary":              bundle.Rest.Docker.Binary,
			"docker.user":                bundle.Rest.Docker.User,
			"docker.output":              bundle.Rest.Docker.Output,
			"docker.dockerignore_output": bundle.Rest.Docker.DockerignoreOutput,
		} {
			if value == "" {
				return fmt.Errorf("%s is required", name)
			}
		}
	}
	if bundle.SQL != nil {
		if database := strings.ToLower(bundle.SQL.Database); database != "" && database != "postgresql" && database != "postgres" {
			return fmt.Errorf("unsupported SQL database %q", bundle.SQL.Database)
		}
		if engine := strings.ToLower(bundle.SQL.Engine); engine != "" && engine != "sqlc" {
			return fmt.Errorf("unsupported SQL engine %q", bundle.SQL.Engine)
		}
		if bundle.SQL.InitMigration.Bool() {
			if engine := strings.ToLower(bundle.SQL.MigrationEngine); engine != "" && engine != "goose" {
				return fmt.Errorf("unsupported migration engine %q", bundle.SQL.MigrationEngine)
			}
			if bundle.SQL.MigrationOutput == "" {
				return fmt.Errorf("sql.migration_output is required when init_migration is enabled")
			}
		}
		required := map[string]string{
			"sql.db_connection.db_name":       bundle.SQL.Connection.DBName,
			"sql.db_connection.user_name":     bundle.SQL.Connection.UserName,
			"sql.db_connection.user_password": bundle.SQL.Connection.UserPassword,
		}
		if bundle.SQL.SQLC.Enabled.Bool() {
			required["sql.sqlc.sqlc_path"] = bundle.SQL.SQLC.Path
		}
		for name, value := range required {
			if value == "" {
				return fmt.Errorf("%s is required", name)
			}
		}
	}
	return nil
}

func ensureProjectModule(ctx Context) error {
	if ctx.ProjectDir == "" {
		return fmt.Errorf("project_path is required")
	}
	if err := os.MkdirAll(ctx.ProjectDir, 0o755); err != nil {
		return err
	}
	goMod := filepath.Join(ctx.ProjectDir, "go.mod")
	if _, err := os.Stat(goMod); err == nil {
		return ensureModuleRequirements(goMod, ctx.Config.Rest)
	} else if !os.IsNotExist(err) {
		return err
	}
	if ctx.Config.Rest.Module == "" {
		return fmt.Errorf("module is required when project go.mod does not exist")
	}
	content := fmt.Sprintf(`module %s

go %s

require (
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1
	github.com/lib/pq v1.12.3
)
`, ctx.Config.Rest.Module, defaultValue(ctx.Config.Rest.GoVersion, "1.24.3"))
	if err := os.WriteFile(goMod, []byte(content), 0o644); err != nil {
		return err
	}
	return ensureModuleRequirements(goMod, ctx.Config.Rest)
}

func ensureModuleRequirements(goMod string, rest config.Rest) error {
	content, err := os.ReadFile(goMod)
	if err != nil {
		return err
	}
	text := string(content)
	requirements := map[string]string{}
	requirements["github.com/joho/godotenv"] = "v1.5.1"
	if rest.Logging.Enabled.Bool() {
		requirements["go.uber.org/zap"] = "v1.27.0"
	}
	if rest.Logging.Enabled.Bool() && rest.Logging.Output.Type == "file" && rest.Logging.Rotation.Enabled.Bool() {
		requirements["gopkg.in/natefinch/lumberjack.v2"] = "v2.2.1"
	}
	var missing []string
	for module, version := range requirements {
		if !strings.Contains(text, module+" ") {
			missing = append(missing, "\t"+module+" "+version)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	text += "\nrequire (\n" + strings.Join(missing, "\n") + "\n)\n"
	return os.WriteFile(goMod, []byte(text), 0o644)
}

func defaultValue(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func runAutoSQLC(ctx Context) error {
	if !ctx.Config.Rest.AutoSQLC.Bool() || ctx.Config.SQL == nil || !ctx.Config.SQL.SQLC.Enabled.Bool() {
		return nil
	}
	sqlcPath := resolveSQLCPath(ctx.ConfigDir, ctx.Config.SQL.SQLC.Path)
	cmd := exec.Command("sqlc", "generate", "-f", sqlcPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run sqlc generate -f %s: %w; install sqlc with: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest", sqlcPath, err)
	}
	return nil
}

func runGoModTidy(projectDir string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = projectDir
	cmd.Env = goCommandEnv()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run go mod tidy: %w", err)
	}
	return nil
}

func goCommandEnv() []string {
	env := os.Environ()
	if os.Getenv("GOCACHE") == "" {
		env = append(env, "GOCACHE="+filepath.Join(os.TempDir(), "rest-go-build"))
	}
	return env
}

func resolveSQLCPath(configDir, sqlcPath string) string {
	if filepath.IsAbs(sqlcPath) {
		return sqlcPath
	}
	return filepath.Clean(filepath.Join(configDir, sqlcPath))
}

type SQLFeature struct{}

func (SQLFeature) Name() string { return "sqlc" }

func (SQLFeature) Enabled(ctx Context) bool {
	return ctx.Config.Rest.SQL.Bool() && ctx.Config.SQL != nil && ctx.Config.SQL.SQLC.Enabled.Bool()
}

func (SQLFeature) Generate(ctx Context) error {
	if ctx.Config.SQL == nil {
		return fmt.Errorf("sqlc_rest.yaml is required when sql is enabled")
	}
	sqlcPath := ctx.Config.SQL.SQLC.Path
	sqlcPath = resolveSQLCPath(ctx.ConfigDir, sqlcPath)
	configPath, err := filepath.Rel(ctx.ProjectDir, ctx.ConfigDir)
	if err != nil {
		return fmt.Errorf("resolve config path from project: %w", err)
	}
	options := generator.Options{
		SQLCPath: sqlcPath,
		OutDir:   ctx.ProjectDir,
		Features: generator.FeatureOptions{
			Build: generator.BuildFeatures{
				Configured:       true,
				Makefile:         ctx.Config.Rest.Features.Makefile.Enabled.Bool(),
				HandlerTests:     ctx.Config.Rest.Testing.HandlerTests.Bool(),
				Curl:             ctx.Config.Rest.Testing.Curl.Bool(),
				HTTPPort:         ctx.Config.Rest.HTTP.Port,
				MakefilePath:     ctx.Config.Rest.Features.Makefile.Output,
				Gitignore:        ctx.Config.Rest.Features.Gitignore.Enabled.Bool(),
				GitignorePath:    ctx.Config.Rest.Features.Gitignore.Output,
				GitignoreAppend:  ctx.Config.Rest.Features.Gitignore.Append,
				Env:              ctx.Config.Rest.Features.Env.Enabled.Bool(),
				EnvPath:          ctx.Config.Rest.Features.Env.Output,
				GenerateLocalEnv: ctx.Config.Rest.Features.Env.GenerateLocalEnv,
				ConfigPath:       configPath,
				InitDB:           ctx.Config.Rest.Features.InitDB.Enabled.Bool(),
				InitDBPath:       ctx.Config.Rest.Features.InitDB.Output,
				SafeReload:       ctx.Config.Rest.SafeReload.Bool(),
				CI:               ctx.Config.Rest.Features.CI.Enabled.Bool(),
				CIPath:           ctx.Config.Rest.Features.CI.Output,
				CD:               ctx.Config.Rest.Features.CD.Enabled.Bool(),
				CDPath:           ctx.Config.Rest.Features.CD.Output,
				InitMigration:    ctx.Config.SQL.InitMigration.Bool(),
				MigrationEngine:  ctx.Config.SQL.MigrationEngine,
				MigrationsPath:   ctx.Config.SQL.MigrationOutput,
				DBName:           ctx.Config.SQL.Connection.DBName,
				DBUser:           ctx.Config.SQL.Connection.UserName,
				DBPassword:       ctx.Config.SQL.Connection.UserPassword,
				DBOptions:        ctx.Config.SQL.Connection.Options,
			},
			HTTP: generator.HTTPFeatures{
				CORS:                  ctx.Config.Rest.HTTP.Middleware.CORS.Enabled.Bool(),
				AllowOrigins:          ctx.Config.Rest.HTTP.Middleware.CORS.AllowOrigins,
				AllowMethods:          ctx.Config.Rest.HTTP.Middleware.CORS.AllowMethods,
				AllowHeaders:          ctx.Config.Rest.HTTP.Middleware.CORS.AllowHeaders,
				ExposeHeaders:         ctx.Config.Rest.HTTP.Middleware.CORS.ExposeHeaders,
				AllowCredentials:      ctx.Config.Rest.HTTP.Middleware.CORS.AllowCredentials,
				CORSMaxAge:            ctx.Config.Rest.HTTP.Middleware.CORS.MaxAge,
				Recovery:              ctx.Config.Rest.HTTP.Middleware.Recovery.Enabled.Bool(),
				RecoveryExposeDetails: ctx.Config.Rest.HTTP.Middleware.Recovery.ExposeDetails,
				RequestID:             ctx.Config.Rest.HTTP.Middleware.RequestID.Enabled.Bool(),
				RequestIDHeader:       ctx.Config.Rest.HTTP.Middleware.RequestID.Header,
				Host:                  ctx.Config.Rest.HTTP.Host,
				Port:                  ctx.Config.Rest.HTTP.Port,
				BasePath:              ctx.Config.Rest.HTTP.BasePath,
				ReadHeaderTimeout:     ctx.Config.Rest.HTTP.Timeouts.ReadHeader,
				ReadTimeout:           ctx.Config.Rest.HTTP.Timeouts.Read,
				WriteTimeout:          ctx.Config.Rest.HTTP.Timeouts.Write,
				IdleTimeout:           ctx.Config.Rest.HTTP.Timeouts.Idle,
				ShutdownTimeout:       ctx.Config.Rest.HTTP.Timeouts.Shutdown,
				MaxBodyBytes:          ctx.Config.Rest.HTTP.Limits.MaxBodyBytes,
				GracefulShutdown:      ctx.Config.Rest.HTTP.GracefulShutdown.Enabled.Bool(),
				Health:                ctx.Config.Rest.HTTP.Health.Enabled.Bool(),
				HealthPath:            ctx.Config.Rest.HTTP.Health.Path,
			},
			Logging: generator.LoggingFeatures{
				Enabled:    ctx.Config.Rest.Logging.Enabled.Bool(),
				Library:    ctx.Config.Rest.Logging.Library,
				Level:      ctx.Config.Rest.Logging.Level,
				Format:     ctx.Config.Rest.Logging.Format,
				OutputType: ctx.Config.Rest.Logging.Output.Type,
				OutputFile: ctx.Config.Rest.Logging.Output.File,
				Rotation:   ctx.Config.Rest.Logging.Rotation.Enabled.Bool(),
				MaxSizeMB:  ctx.Config.Rest.Logging.Rotation.MaxSizeMB,
				MaxBackups: ctx.Config.Rest.Logging.Rotation.MaxBackups,
				MaxAgeDays: ctx.Config.Rest.Logging.Rotation.MaxAgeDays,
				Compress:   ctx.Config.Rest.Logging.Rotation.Compress,
				Fields:     ctx.Config.Rest.Logging.Fields,
				Redact:     ctx.Config.Rest.Logging.Redact,
			},
			OpenAPI: generator.OpenAPIFeatures{
				Enabled:         ctx.Config.Rest.OpenAPI.Enabled.Bool(),
				Output:          ctx.Config.Rest.OpenAPI.Output,
				WithUI:          ctx.Config.Rest.OpenAPI.WithUI.Bool(),
				Title:           ctx.Config.Rest.OpenAPI.Title,
				Version:         ctx.Config.Rest.OpenAPI.Version,
				Description:     ctx.Config.Rest.OpenAPI.Description,
				ServerURL:       ctx.Config.Rest.OpenAPI.ServerURL,
				UIPath:          ctx.Config.Rest.OpenAPI.UIPath,
				SpecPath:        ctx.Config.Rest.OpenAPI.SpecPath,
				SecuritySchemes: ctx.Config.Rest.OpenAPI.SecuritySchemes,
			},
			Metrics: generator.MetricsFeatures{
				Enabled:          ctx.Config.Rest.Observability.Metrics.Enabled.Bool(),
				Provider:         ctx.Config.Rest.Observability.Metrics.Provider,
				Path:             ctx.Config.Rest.Observability.Metrics.Path,
				Namespace:        ctx.Config.Rest.Observability.Metrics.Namespace,
				HTTPRequests:     ctx.Config.Rest.Observability.Metrics.Collect.HTTPRequests,
				RequestDuration:  ctx.Config.Rest.Observability.Metrics.Collect.RequestDuration,
				ResponseSize:     ctx.Config.Rest.Observability.Metrics.Collect.ResponseSize,
				InFlightRequests: ctx.Config.Rest.Observability.Metrics.Collect.InFlightRequests,
				Labels:           ctx.Config.Rest.Observability.Metrics.Labels,
			},
			Docker: generator.DockerFeatures{
				Enabled:            ctx.Config.Rest.Docker.Enabled.Bool(),
				Output:             ctx.Config.Rest.Docker.Output,
				DockerignoreOutput: ctx.Config.Rest.Docker.DockerignoreOutput,
				BuildImage:         ctx.Config.Rest.Docker.BuildImage,
				RuntimeImage:       ctx.Config.Rest.Docker.RuntimeImage,
				Binary:             ctx.Config.Rest.Docker.Binary,
				Port:               ctx.Config.Rest.Docker.Port,
				User:               ctx.Config.Rest.Docker.User,
				CGOEnabled:         ctx.Config.Rest.Docker.CGOEnabled,
				Healthcheck:        ctx.Config.Rest.Docker.Healthcheck.Enabled.Bool(),
				HealthPath:         routePath(ctx.Config.Rest.HTTP.BasePath, ctx.Config.Rest.Docker.Healthcheck.Path),
				HealthInterval:     ctx.Config.Rest.Docker.Healthcheck.Interval,
				HealthTimeout:      ctx.Config.Rest.Docker.Healthcheck.Timeout,
				HealthRetries:      ctx.Config.Rest.Docker.Healthcheck.Retries,
			},
		},
	}
	return generator.Generate(options)
}

func routePath(base, path string) string {
	if path == "" {
		path = "/"
	}
	if base == "" || base == "/" {
		return path
	}
	return strings.TrimRight(base, "/") + path
}
