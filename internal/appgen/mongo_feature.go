package appgen

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/repomz/rest/internal/generator"
)

type MongoFeature struct{}

func (MongoFeature) Name() string { return "mongo" }

func (MongoFeature) Enabled(ctx Context) bool {
	return ctx.Config.Rest.Mongo.Bool() && ctx.Config.Mongo != nil && !(SQLFeature{}).Enabled(ctx)
}

func (MongoFeature) Generate(ctx Context) error {
	models, err := discoverMongoOpenAPIModels(ctx)
	if err != nil {
		return err
	}
	features := mongoFeatureOptions(ctx, models)
	module := ctx.Config.Rest.Module
	if module == "" {
		module = "generated-mongo-api"
	}
	swagger := generator.BuildMongoOpenAPISpec(module, features)
	files := map[string]string{
		"cmd/main.go":                                        mongoMainSource(ctx, models),
		"internal/app/common/server/response.go":             mongoServerResponseSource(),
		"internal/app/domain/document.go":                    mongoDomainSource(),
		"internal/app/repository/mongorepo/filter.go":        mongoRepositoryFilterSource(),
		"internal/app/transport/httpserver/server.go":        mongoHTTPServerSource(ctx, models),
		"internal/app/transport/httpserver/swagger.go":       mongoSwaggerSource(ctx, swagger),
		"internal/app/transport/httpserver/mongo_helpers.go": mongoHTTPHelpersSource(),
	}
	if ctx.Config.Rest.Features.Makefile.Enabled.Bool() {
		output := ctx.Config.Rest.Features.Makefile.Output
		if output == "" {
			output = "Makefile"
		}
		files[output] = mongoMakefileSource(ctx)
	}
	if ctx.Config.Rest.Features.InitDB.Enabled.Bool() {
		output := ctx.Config.Rest.Features.InitDB.Output
		if output == "" {
			output = "init_db.sh"
		}
		source, err := generator.BuildInitDBSource(features)
		if err != nil {
			return err
		}
		files[output] = source
	}
	if ctx.Config.Rest.Features.Gitignore.Enabled.Bool() {
		output := ctx.Config.Rest.Features.Gitignore.Output
		if output == "" {
			output = ".gitignore"
		}
		source, err := generator.BuildGitignoreSource(features)
		if err != nil {
			return err
		}
		files[output] = source
	}
	if mongoMiddlewareEnabled(ctx) {
		files["internal/app/transport/middleware/http.go"] = mongoMiddlewareSource(ctx)
	}
	if ctx.Config.Rest.Logging.Enabled.Bool() {
		source, err := generator.BuildLoggingSource(features)
		if err != nil {
			return err
		}
		files["internal/app/logging/logger.go"] = source
	}
	if ctx.Config.Rest.Observability.Metrics.Enabled.Bool() {
		source, err := generator.BuildMetricsSource(features)
		if err != nil {
			return err
		}
		files["internal/app/metrics/metrics.go"] = source
	}
	files["internal/app/transport/httpserver/auth_middleware.go"] = mongoAuthMiddlewareSource(ctx)
	for _, model := range models {
		if model.Embedded || model.Collection == "" {
			continue
		}
		name := strings.ToLower(model.Name)
		files["internal/app/repository/mongorepo/"+name+"_repo.go"] = mongoRepositorySource(model)
		files["internal/app/services/"+name+".go"] = mongoServiceSource(model)
		files["internal/app/transport/httpserver/"+name+"_handlers.go"] = mongoHandlersSource(model)
		if ctx.Config.Rest.Testing.HandlerTests.Bool() {
			files["internal/app/transport/httpserver/"+name+"_handlers_test.go"] = mongoHandlersTestSource(model)
		}
	}
	openAPIOutput := ctx.Config.Rest.OpenAPI.Output
	if openAPIOutput == "" {
		openAPIOutput = "docs/swagger.yaml"
	}
	files[openAPIOutput] = swagger
	if ctx.Config.Rest.Features.Env.Enabled.Bool() {
		envPath := ctx.Config.Rest.Features.Env.Output
		if envPath == "" {
			envPath = ".env.example"
		}
		envSource := mongoEnvSource(ctx)
		files[envPath] = envSource
		if ctx.Config.Rest.Features.Env.GenerateLocalEnv {
			files[".env"] = envSource
		}
	}
	if ctx.Config.Rest.Docker.Enabled.Bool() {
		output := ctx.Config.Rest.Docker.Output
		if output == "" {
			output = "Dockerfile"
		}
		files[output] = mongoDockerfileSource(ctx)
		ignoreOutput := ctx.Config.Rest.Docker.DockerignoreOutput
		if ignoreOutput == "" {
			ignoreOutput = ".dockerignore"
		}
		files[ignoreOutput] = mongoDockerignoreSource()
	}
	if ctx.Config.Rest.Docker.Compose.Enabled.Bool() {
		output := ctx.Config.Rest.Docker.Compose.Output
		if output == "" {
			output = "docker-compose.yml"
		}
		files[output] = mongoDockerComposeSource(ctx)
	}
	if ctx.Config.Rest.Features.DeploymentGuide.Enabled.Bool() {
		output := ctx.Config.Rest.Features.DeploymentGuide.Output
		if output == "" {
			output = "DEPLOYMENT.md"
		}
		files[output] = generator.BuildDeploymentGuideSource(mongoFeatureOptions(ctx, models))
	}
	if ctx.Config.Rest.Features.Architecture.Enabled.Bool() {
		output := ctx.Config.Rest.Features.Architecture.Output
		if output == "" {
			output = "ARCHITECTURE.md"
		}
		files[output] = generator.BuildArchitectureSource(module, nil, features)
	}
	if ctx.Config.Rest.Features.Readme.Enabled.Bool() {
		output := ctx.Config.Rest.Features.Readme.Output
		if output == "" {
			output = "README.md"
		}
		files[output] = generator.BuildReadmeSource(module, nil, features)
	}
	if ctx.Config.Rest.Features.CI.Enabled.Bool() {
		output := ctx.Config.Rest.Features.CI.Output
		if output == "" {
			output = ".github/workflows/ci.yaml"
		}
		source, err := generator.BuildCIWorkflowSource(features)
		if err != nil {
			return err
		}
		files[output] = source
	}
	if ctx.Config.Rest.Features.CD.Enabled.Bool() {
		output := ctx.Config.Rest.Features.CD.Output
		if output == "" {
			output = ".github/workflows/cd.yaml"
		}
		source, err := generator.BuildCDWorkflowSource(features)
		if err != nil {
			return err
		}
		files[output] = source
	}
	generatedFiles := make([]string, 0, len(files))
	for path := range files {
		generatedFiles = append(generatedFiles, path)
	}
	var preserved map[string][]byte
	if ctx.Config.Rest.SafeReload.Bool() {
		preserved, err = generator.ResolveSafeReload(ctx.ProjectDir, generatedFiles, os.Stdin, os.Stdout)
		if err != nil {
			return err
		}
	}
	for path, content := range files {
		content = strings.ReplaceAll(content, "{{MODULE}}", module)
		target := filepath.Join(ctx.ProjectDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(path, ".sh") {
			mode = 0o755
		} else if filepath.Base(path) == ".env" {
			mode = 0o600
		}
		if err := os.WriteFile(target, []byte(content), mode); err != nil {
			return err
		}
	}
	if ctx.Config.Rest.SafeReload.Bool() {
		if err := generator.SaveSafeReload(ctx.ProjectDir, generatedFiles); err != nil {
			return err
		}
		if err := generator.RestoreSafeReload(ctx.ProjectDir, preserved); err != nil {
			return err
		}
	}
	return nil
}

func mongoFeatureOptions(ctx Context, models []generator.MongoModel) generator.FeatureOptions {
	return generator.FeatureOptions{
		HTTP: generator.HTTPFeatures{
			BasePath:        ctx.Config.Rest.HTTP.BasePath,
			Host:            ctx.Config.Rest.HTTP.Host,
			Port:            ctx.Config.Rest.HTTP.Port,
			RequestID:       ctx.Config.Rest.HTTP.Middleware.RequestID.Enabled.Bool(),
			RequestIDHeader: ctx.Config.Rest.HTTP.Middleware.RequestID.Header,
			Health:          ctx.Config.Rest.HTTP.Health.Enabled.Bool(),
			HealthPath:      ctx.Config.Rest.HTTP.Health.Path,
			Readiness:       ctx.Config.Rest.HTTP.Readiness.Enabled.Bool(),
			ReadinessPath:   ctx.Config.Rest.HTTP.Readiness.Path,
		},
		Auth: authFeatures(ctx.Config),
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
		Build: generator.BuildFeatures{
			Configured:       true,
			Backend:          "mongo",
			HTTPPort:         ctx.Config.Rest.HTTP.Port,
			Makefile:         ctx.Config.Rest.Features.Makefile.Enabled.Bool(),
			MakefilePath:     ctx.Config.Rest.Features.Makefile.Output,
			Gitignore:        ctx.Config.Rest.Features.Gitignore.Enabled.Bool(),
			GitignorePath:    ctx.Config.Rest.Features.Gitignore.Output,
			GitignoreAppend:  ctx.Config.Rest.Features.Gitignore.Append,
			Readme:           ctx.Config.Rest.Features.Readme.Enabled.Bool(),
			ReadmePath:       ctx.Config.Rest.Features.Readme.Output,
			Architecture:     ctx.Config.Rest.Features.Architecture.Enabled.Bool(),
			ArchitecturePath: ctx.Config.Rest.Features.Architecture.Output,
			Env:              ctx.Config.Rest.Features.Env.Enabled.Bool(),
			EnvPath:          ctx.Config.Rest.Features.Env.Output,
			GenerateLocalEnv: ctx.Config.Rest.Features.Env.GenerateLocalEnv,
			InitDB:           ctx.Config.Rest.Features.InitDB.Enabled.Bool(),
			InitDBPath:       ctx.Config.Rest.Features.InitDB.Output,
			DeploymentGuide:  ctx.Config.Rest.Features.DeploymentGuide.Enabled.Bool(),
			DeploymentPath:   ctx.Config.Rest.Features.DeploymentGuide.Output,
			CI:               ctx.Config.Rest.Features.CI.Enabled.Bool(),
			CIPath:           ctx.Config.Rest.Features.CI.Output,
			CD:               ctx.Config.Rest.Features.CD.Enabled.Bool(),
			CDPath:           ctx.Config.Rest.Features.CD.Output,
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
			Compose:            ctx.Config.Rest.Docker.Compose.Enabled.Bool(),
			ComposeOutput:      ctx.Config.Rest.Docker.Compose.Output,
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
		Mongo: generator.MongoFeatures{
			Models:   models,
			URIEnv:   ctx.Config.Mongo.Connection.URIEnv,
			Database: ctx.Config.Mongo.Connection.Database,
			User:     ctx.Config.Mongo.Connection.UserName,
			Password: ctx.Config.Mongo.Connection.UserPassword,
		},
	}
}

func mongoMainSource(ctx Context, models []generator.MongoModel) string {
	module := ctx.Config.Rest.Module
	if module == "" {
		module = "generated-mongo-api"
	}
	var repoVars strings.Builder
	var serviceVars strings.Builder
	var serverFields strings.Builder
	for _, model := range models {
		if model.Embedded || model.Collection == "" {
			continue
		}
		lower := strings.ToLower(model.Name[:1]) + model.Name[1:]
		fmt.Fprintf(&repoVars, "\t%sRepo := mongorepo.New%sRepository(database.Collection(%q))\n", lower, model.Name, model.Collection)
		fmt.Fprintf(&serviceVars, "\t%sService := services.New%sService(%sRepo)\n", lower, model.Name, lower)
		fmt.Fprintf(&serverFields, "\t\t%sService: %sService,\n", model.Name, lower)
	}
	middlewareImport := ""
	if mongoMiddlewareEnabled(ctx) {
		middlewareImport = fmt.Sprintf("\n\t%q", module+"/internal/app/transport/middleware")
	}
	loggingImport := ""
	zapImport := ""
	if ctx.Config.Rest.Logging.Enabled.Bool() {
		loggingImport = fmt.Sprintf("\n\t%q", module+"/internal/app/logging")
		zapImport = "\n\t\"go.uber.org/zap\""
	}
	metricsImport := ""
	if ctx.Config.Rest.Observability.Metrics.Enabled.Bool() {
		metricsImport = fmt.Sprintf("\n\t%q", module+"/internal/app/metrics")
	}
	handlerSetup := "handler := http.Handler(router)"
	if ctx.Config.Rest.HTTP.Middleware.RateLimit.Enabled.Bool() {
		window := ctx.Config.Rest.HTTP.Middleware.RateLimit.Window
		if window == "" {
			window = "1m"
		}
		handlerSetup += fmt.Sprintf("\n\thandler = middleware.RateLimit(%d, mustDuration(%q), handler)", ctx.Config.Rest.HTTP.Middleware.RateLimit.RequestsPerWindow, window)
	}
	if ctx.Config.Rest.HTTP.Middleware.CORS.Enabled.Bool() {
		handlerSetup += "\n\thandler = middleware.CORS(handler)"
	}
	if ctx.Config.Rest.HTTP.Middleware.SecurityHeaders.Enabled.Bool() {
		handlerSetup += "\n\thandler = middleware.SecurityHeaders(handler)"
	}
	if ctx.Config.Rest.HTTP.Middleware.Recovery.Enabled.Bool() {
		handlerSetup += "\n\thandler = middleware.Recovery(handler)"
	}
	if ctx.Config.Rest.HTTP.Middleware.RequestID.Enabled.Bool() {
		handlerSetup += "\n\thandler = middleware.RequestID(handler)"
	}
	if ctx.Config.Rest.Observability.Metrics.Enabled.Bool() {
		handlerSetup += "\n\thandler = metrics.Middleware(handler)"
	}
	if ctx.Config.Rest.Logging.Enabled.Bool() {
		handlerSetup += "\n\thandler = logging.Middleware(logger, handler)"
	}
	readinessPath := routePath(ctx.Config.Rest.HTTP.BasePath, ctx.Config.Rest.HTTP.Readiness.Path)
	metricsPath := routePath(ctx.Config.Rest.HTTP.BasePath, ctx.Config.Rest.Observability.Metrics.Path)
	return fmt.Sprintf(`package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
%s
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"%s/internal/app/repository/mongorepo"
	"%s/internal/app/services"
	"%s/internal/app/transport/httpserver"
%s
%s
%s
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	%s
	timeout, err := time.ParseDuration(%q)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(env(%q, %q)))
	if err != nil {
		return err
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = client.Disconnect(ctx)
	}()
	database := client.Database(%q)
%s%s
	router := mux.NewRouter()
	httpServer := httpserver.HttpServer{
%s		BasicAuth: httpserver.BasicAuthConfig{
			Username: env(%q, ""),
			Password: env(%q, ""),
			Realm:    %q,
			Roles:    []string{%s},
		},
		JWTSecret:    env(%q, ""),
		JWTHeader:    %q,
		JWTScheme:    %q,
		JWTIssuer:    %q,
		JWTAudience:  %q,
		JWTRoleClaim: %q,
	}
	httpServer.RegisterRoutes(router)
	%s
	%s
	%s
	server := &http.Server{
		Addr:              env("HTTP_ADDR", %q),
		Handler:           handler,
		ReadHeaderTimeout: mustDuration(%q),
		ReadTimeout:       mustDuration(%q),
		WriteTimeout:      mustDuration(%q),
		IdleTimeout:       mustDuration(%q),
	}
	stopped := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		<-sigint
		ctx, cancel := context.WithTimeout(context.Background(), mustDuration(%q))
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			%s
		}
		close(stopped)
	}()

	%s
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	<-stopped
	return nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func mustDuration(value string) time.Duration {
	duration, err := time.ParseDuration(value)
	if err != nil {
		panic(err)
	}
	return duration
}
`, zapImport, module, module, module, middlewareImport, loggingImport, metricsImport, mongoLoggerSetup(ctx), mongoTimeout(ctx), mongoURIEnv(ctx), mongoConnectionURI(ctx, "localhost:27017"), ctx.Config.Mongo.Connection.Database, repoVars.String(), serviceVars.String(), serverFields.String(), mongoAuthBasicUsernameEnv(ctx), mongoAuthBasicPasswordEnv(ctx), mongoAuthBasicRealm(ctx), mongoQuotedSlice(mongoAuthBasicRoles(ctx)), mongoAuthJWTSecretEnv(ctx), mongoAuthJWTHeader(ctx), mongoAuthJWTScheme(ctx), mongoAuthJWTIssuer(ctx), mongoAuthJWTAudience(ctx), mongoAuthRoleClaim(ctx), mongoReadinessRouteSource(ctx, readinessPath), mongoMetricsRouteSource(ctx, metricsPath), handlerSetup, mongoHTTPAddr(ctx), defaultString(ctx.Config.Rest.HTTP.Timeouts.ReadHeader, "5s"), defaultString(ctx.Config.Rest.HTTP.Timeouts.Read, "30s"), defaultString(ctx.Config.Rest.HTTP.Timeouts.Write, "30s"), defaultString(ctx.Config.Rest.HTTP.Timeouts.Idle, "60s"), defaultString(ctx.Config.Rest.HTTP.Timeouts.Shutdown, "10s"), mongoShutdownLog(ctx), mongoStartupLog(ctx))
}

func mongoServerResponseSource() string {
	return `package server

import (
	"encoding/json"
	"net/http"
)

func RespondOK(value any, w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, value)
}

func BadRequest(code string, err error, w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusBadRequest, code, err)
}

func NotFound(code string, err error, w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, code, err)
}

func Unauthorised(code string, err error, w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusUnauthorized, code, err)
}

func Forbidden(code string, err error, w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusForbidden, code, err)
}

func InternalError(code string, err error, w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusInternalServerError, code, err)
}

func RespondWithError(err error, w http.ResponseWriter, r *http.Request) {
	InternalError("internal-error", err, w, r)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code string, err error) {
	response := map[string]string{"error": code}
	if err != nil {
		response["message"] = err.Error()
	}
	writeJSON(w, status, response)
}
`
}

func mongoMiddlewareEnabled(ctx Context) bool {
	middleware := ctx.Config.Rest.HTTP.Middleware
	return middleware.CORS.Enabled.Bool() || middleware.SecurityHeaders.Enabled.Bool() || middleware.RateLimit.Enabled.Bool() || middleware.Recovery.Enabled.Bool() || middleware.RequestID.Enabled.Bool()
}

func mongoMakefileSource(ctx Context) string {
	httpAddr := mongoHTTPAddr(ctx)
	uriEnv := "MONGO_URI"
	if ctx.Config.Mongo != nil && ctx.Config.Mongo.Connection.URIEnv != "" {
		uriEnv = ctx.Config.Mongo.Connection.URIEnv
	}
	dbVariable := ""
	dbPhony := ""
	dbTarget := ""
	if ctx.Config.Rest.Features.InitDB.Enabled.Bool() {
		dbScript := ctx.Config.Rest.Features.InitDB.Output
		if dbScript == "" {
			dbScript = "init_db.sh"
		}
		dbVariable = "DB_SCRIPT ?= " + dbScript + "\n"
		dbPhony = " db"
		dbTarget = `
db:
	@test -f $(DB_SCRIPT) || { echo "Error: $(DB_SCRIPT) is missing"; exit 1; }
	@chmod +x $(DB_SCRIPT)
	@./$(DB_SCRIPT)
`
	}
	return fmt.Sprintf(`-include .env

APP_NAME ?= app
BUILD_DIR ?= ./bin
%sHTTP_ADDR ?= %s
%s ?= %s
GOCACHE ?= $(CURDIR)/.cache/go-build
GOLANGCI_LINT_VERSION ?= v1.64.8
REST ?= rest

export

.PHONY: build rest-gen run test clean install-lint lint%s

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd

rest-gen:
	$(REST) gen

run:
	@mkdir -p $(BUILD_DIR) && \
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd && \
	HTTP_ADDR=$(HTTP_ADDR) \
	%s=$(%s) \
	$(BUILD_DIR)/$(APP_NAME)

test:
	go test -race -v ./...

install-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

lint:
	@command -v golangci-lint >/dev/null 2>&1 || $(MAKE) install-lint
	golangci-lint run ./...

clean:
	rm -rf $(BUILD_DIR)
%s
`, dbVariable, httpAddr, uriEnv, mongoConnectionURI(ctx, "localhost:27017"), dbPhony, uriEnv, uriEnv, dbTarget)
}

func mongoMiddlewareSource(ctx Context) string {
	middleware := ctx.Config.Rest.HTTP.Middleware
	corsEnabled := middleware.CORS.Enabled.Bool()
	securityEnabled := middleware.SecurityHeaders.Enabled.Bool()
	rateEnabled := middleware.RateLimit.Enabled.Bool()
	origins := mongoBoolMapEntries(middleware.CORS.AllowOrigins)
	methods := strings.Join(middleware.CORS.AllowMethods, ", ")
	headers := strings.Join(middleware.CORS.AllowHeaders, ", ")
	exposeHeaders := strings.Join(middleware.CORS.ExposeHeaders, ", ")
	maxAge := ""
	if middleware.CORS.MaxAge != "" {
		if duration, err := time.ParseDuration(middleware.CORS.MaxAge); err == nil {
			maxAge = strconv.FormatInt(int64(duration.Seconds()), 10)
		}
	}
	return fmt.Sprintf(`package middleware

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

func CORS(next http.Handler) http.Handler {
	if !%t {
		return next
	}
	allowed := map[string]bool{%s}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowed["*"] {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", %q)
		w.Header().Set("Access-Control-Allow-Methods", %q)
		if %q != "" {
			w.Header().Set("Access-Control-Expose-Headers", %q)
		}
		if %t {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if %q != "" {
			w.Header().Set("Access-Control-Max-Age", %q)
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func SecurityHeaders(next http.Handler) http.Handler {
	if !%t {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setHeader(w, "X-Content-Type-Options", %q)
		setHeader(w, "X-Frame-Options", %q)
		setHeader(w, "Referrer-Policy", %q)
		setHeader(w, "Permissions-Policy", %q)
		setHeader(w, "Content-Security-Policy", %q)
		setHeader(w, "Strict-Transport-Security", %q)
		next.ServeHTTP(w, r)
	})
}

func setHeader(w http.ResponseWriter, name, value string) {
	if value != "" {
		w.Header().Set(name, value)
	}
}

func Recovery(next http.Handler) http.Handler {
	if !%t {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic recovered: %%v", recovered)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func RequestID(next http.Handler) http.Handler {
	if !%t {
		return next
	}
	const header = %q
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(header)
		if id == "" {
			id = uuid.NewString()
		}
		r.Header.Set(header, id)
		w.Header().Set(header, id)
		next.ServeHTTP(w, r)
	})
}

func RateLimit(requestsPerWindow int, window time.Duration, next http.Handler) http.Handler {
	if !%t || requestsPerWindow < 1 || window <= 0 {
		return next
	}
	type bucket struct {
		start time.Time
		count int
	}
	var mu sync.Mutex
	buckets := map[string]bucket{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		key := clientIP(r)
		mu.Lock()
		current := buckets[key]
		if current.start.IsZero() || now.Sub(current.start) >= window {
			current = bucket{start: now}
		}
		current.count++
		buckets[key] = current
		limited := current.count > requestsPerWindow
		for key, value := range buckets {
			if now.Sub(value.start) >= window*2 {
				delete(buckets, key)
			}
		}
		mu.Unlock()
		if limited {
			w.Header().Set("Retry-After", fmt.Sprintf("%%.0f", window.Seconds()))
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
`, corsEnabled, origins, headers, methods, exposeHeaders, exposeHeaders, middleware.CORS.AllowCredentials, maxAge, maxAge, securityEnabled, middleware.SecurityHeaders.ContentTypeOptions, middleware.SecurityHeaders.FrameOptions, middleware.SecurityHeaders.ReferrerPolicy, middleware.SecurityHeaders.PermissionsPolicy, middleware.SecurityHeaders.ContentSecurityPolicy, middleware.SecurityHeaders.StrictTransportSecurity, middleware.Recovery.Enabled.Bool(), middleware.RequestID.Enabled.Bool(), defaultString(middleware.RequestID.Header, "X-Request-ID"), rateEnabled)
}

func mongoDomainSource() string {
	return `package domain

import (
	"errors"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrNotFound = errors.New("document not found")

type Document map[string]any

func NormalizeDocument(doc Document) Document {
	if doc == nil {
		return Document{}
	}
	if value, ok := doc["_id"].(primitive.ObjectID); ok {
		doc["id"] = value.Hex()
	}
	return doc
}

func NormalizeDocuments(docs []Document) []Document {
	if docs == nil {
		return []Document{}
	}
	for i := range docs {
		docs[i] = NormalizeDocument(docs[i])
	}
	return docs
}
`
}

func mongoRepositoryFilterSource() string {
	return `package mongorepo

import "go.mongodb.org/mongo-driver/bson"

type methodFilter struct {
	Field string
	Op    string
	Param string
	Value any
}

func buildMethodFilter(params map[string]any, filters []methodFilter) bson.M {
	if len(filters) == 0 {
		filter := bson.M{}
		for key, value := range params {
			if key == "body" || value == nil {
				continue
			}
			filter[key] = value
		}
		return filter
	}
	filter := bson.M{}
	for _, item := range filters {
		if item.Field == "" {
			continue
		}
		value := item.Value
		if item.Param != "" {
			value = params[item.Param]
		}
		if value == nil {
			continue
		}
		switch item.Op {
		case "", "eq":
			filter[item.Field] = value
		case "ne":
			filter[item.Field] = bson.M{"$ne": value}
		case "gt":
			filter[item.Field] = bson.M{"$gt": value}
		case "gte":
			filter[item.Field] = bson.M{"$gte": value}
		case "lt":
			filter[item.Field] = bson.M{"$lt": value}
		case "lte":
			filter[item.Field] = bson.M{"$lte": value}
		case "regex":
			filter[item.Field] = bson.M{"$regex": value}
		case "in":
			filter[item.Field] = bson.M{"$in": value}
		case "nin":
			filter[item.Field] = bson.M{"$nin": value}
		case "exists":
			filter[item.Field] = bson.M{"$exists": value}
		default:
			filter[item.Field] = value
		}
	}
	return filter
}
`
}

func mongoRepositorySource(model generator.MongoModel) string {
	var methods strings.Builder
	for _, method := range model.Methods {
		fmt.Fprint(&methods, mongoRepositoryMethodSource(method))
	}
	return fmt.Sprintf(`package mongorepo

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"%s/internal/app/domain"
)

type %sRepository struct {
	collection *mongo.Collection
}

func New%sRepository(collection *mongo.Collection) %sRepository {
	return %sRepository{collection: collection}
}

func (r %sRepository) List(ctx context.Context) ([]domain.Document, error) {
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var items []domain.Document
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return domain.NormalizeDocuments(items), nil
}

func (r %sRepository) Create(ctx context.Context, input domain.Document) (domain.Document, error) {
	delete(input, "id")
	delete(input, "_id")
	result, err := r.collection.InsertOne(ctx, input)
	if err != nil {
		return nil, err
	}
	input["_id"] = result.InsertedID
	return domain.NormalizeDocument(input), nil
}

func (r %sRepository) GetByID(ctx context.Context, id primitive.ObjectID) (domain.Document, error) {
	var item domain.Document
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&item)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return domain.NormalizeDocument(item), nil
}

func (r %sRepository) Update(ctx context.Context, id primitive.ObjectID, input domain.Document) (domain.Document, error) {
	delete(input, "id")
	delete(input, "_id")
	if len(input) == 0 {
		return nil, errors.New("empty update body")
	}
	var item domain.Document
	err := r.collection.FindOneAndUpdate(ctx, bson.M{"_id": id}, bson.M{"$set": input}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&item)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return domain.NormalizeDocument(item), nil
}

func (r %sRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}
%s`, mongoModulePlaceholder(), model.Name, model.Name, model.Name, model.Name, model.Name, model.Name, model.Name, model.Name, model.Name, methods.String())
}

func mongoRepositoryMethodSource(method generator.MongoMethod) string {
	filters := mongoMethodFiltersLiteral(method.Filters)
	switch strings.ToLower(method.Operation) {
	case "find_one":
		return fmt.Sprintf(`

func (r %sRepository) %s(ctx context.Context, params map[string]any) (any, error) {
	filter := buildMethodFilter(params, %s)
	var item domain.Document
	err := r.collection.FindOne(ctx, filter).Decode(&item)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return domain.NormalizeDocument(item), nil
}
`, method.Model, method.Name, filters)
	case "update_one":
		return fmt.Sprintf(`

func (r %sRepository) %s(ctx context.Context, params map[string]any) (any, error) {
	filter := buildMethodFilter(params, %s)
	update, _ := params["body"].(domain.Document)
	if len(update) == 0 {
		return nil, errors.New("empty update body")
	}
	delete(update, "id")
	delete(update, "_id")
	var item domain.Document
	err := r.collection.FindOneAndUpdate(ctx, filter, bson.M{"$set": update}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&item)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return domain.NormalizeDocument(item), nil
}
`, method.Model, method.Name, filters)
	case "delete_one":
		return fmt.Sprintf(`

func (r %sRepository) %s(ctx context.Context, params map[string]any) (any, error) {
	filter := buildMethodFilter(params, %s)
	result, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return nil, err
	}
	if result.DeletedCount == 0 {
		return nil, domain.ErrNotFound
	}
	return map[string]bool{"deleted": true}, nil
}
`, method.Model, method.Name, filters)
	case "aggregate":
		return fmt.Sprintf(`

func (r %sRepository) %s(ctx context.Context, params map[string]any) (any, error) {
	filter := buildMethodFilter(params, %s)
	pipeline := mongo.Pipeline{}
	if len(filter) > 0 {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: filter}})
	}
	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var items []domain.Document
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return domain.NormalizeDocuments(items), nil
}
`, method.Model, method.Name, filters)
	default:
		return fmt.Sprintf(`

func (r %sRepository) %s(ctx context.Context, params map[string]any) (any, error) {
	filter := buildMethodFilter(params, %s)
	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var items []domain.Document
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return domain.NormalizeDocuments(items), nil
}
`, method.Model, method.Name, filters)
	}
}

func mongoServiceSource(model generator.MongoModel) string {
	var methods strings.Builder
	for _, method := range model.Methods {
		fmt.Fprintf(&methods, `

func (s %sService) %s(ctx context.Context, params map[string]any) (any, error) {
	return s.repo.%s(ctx, params)
}
`, model.Name, method.Name, method.Name)
	}
	return fmt.Sprintf(`package services

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"%s/internal/app/domain"
	"%s/internal/app/repository/mongorepo"
)

type %sService struct {
	repo mongorepo.%sRepository
}

func New%sService(repo mongorepo.%sRepository) %sService {
	return %sService{repo: repo}
}

func (s %sService) List(ctx context.Context) ([]domain.Document, error) {
	return s.repo.List(ctx)
}

func (s %sService) Create(ctx context.Context, input domain.Document) (domain.Document, error) {
	return s.repo.Create(ctx, input)
}

func (s %sService) GetByID(ctx context.Context, id primitive.ObjectID) (domain.Document, error) {
	return s.repo.GetByID(ctx, id)
}

func (s %sService) Update(ctx context.Context, id primitive.ObjectID, input domain.Document) (domain.Document, error) {
	return s.repo.Update(ctx, id, input)
}

func (s %sService) Delete(ctx context.Context, id primitive.ObjectID) error {
	return s.repo.Delete(ctx, id)
}
%s`, mongoModulePlaceholder(), mongoModulePlaceholder(), model.Name, model.Name, model.Name, model.Name, model.Name, model.Name, model.Name, model.Name, model.Name, model.Name, model.Name, methods.String())
}

func mongoHTTPServerSource(ctx Context, models []generator.MongoModel) string {
	module := ctx.Config.Rest.Module
	if module == "" {
		module = "generated-mongo-api"
	}
	var interfaces strings.Builder
	var fields strings.Builder
	var routes strings.Builder
	for _, model := range models {
		if model.Embedded || model.Collection == "" {
			continue
		}
		fmt.Fprint(&interfaces, mongoServiceInterfaceSource(model))
		fmt.Fprintf(&fields, "\t%sService %sService\n", model.Name, model.Name)
		base := "/" + strings.Trim(model.Collection, "/")
		addMongoRoute(&routes, ctx, "GET", base, "h.GetAll"+pluralizeGoName(model.Name))
		addMongoRoute(&routes, ctx, "POST", base, "h.Create"+model.Name)
		for _, method := range model.Methods {
			httpMethod := method.Method
			if httpMethod == "" {
				httpMethod = mongoHTTPMethodForOperation(method.Operation)
			}
			addMongoRoute(&routes, ctx, httpMethod, method.Path, "h."+method.Name)
		}
		addMongoRoute(&routes, ctx, "GET", base+"/{id}", "h.Get"+model.Name+"ByID")
		addMongoRoute(&routes, ctx, "PATCH", base+"/{id}", "h.Update"+model.Name)
		addMongoRoute(&routes, ctx, "DELETE", base+"/{id}", "h.Delete"+model.Name)
	}
	rootPath := routePath(ctx.Config.Rest.HTTP.BasePath, "/")
	healthPath := routePath(ctx.Config.Rest.HTTP.BasePath, ctx.Config.Rest.HTTP.Health.Path)
	specPath := routePath(ctx.Config.Rest.HTTP.BasePath, ctx.Config.Rest.OpenAPI.SpecPath)
	uiPath := routePath(ctx.Config.Rest.HTTP.BasePath, ctx.Config.Rest.OpenAPI.UIPath)
	return fmt.Sprintf(`package httpserver

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"%s/internal/app/common/server"
	"%s/internal/app/domain"
)

%s

type HttpServer struct {
%s	BasicAuth    BasicAuthConfig
	JWTSecret    string
	JWTHeader    string
	JWTScheme    string
	JWTIssuer    string
	JWTAudience  string
	JWTRoleClaim string
}

func (h HttpServer) RegisterRoutes(router *mux.Router) {
	router.HandleFunc(%q, func(w http.ResponseWriter, r *http.Request) {
		server.RespondOK(map[string]string{"status": "ok"}, w, r)
	}).Methods(http.MethodGet)
	router.HandleFunc(%q, func(w http.ResponseWriter, r *http.Request) {
		server.RespondOK(map[string]string{"status": "ok"}, w, r)
	}).Methods(http.MethodGet)
	router.HandleFunc(%q, h.OpenAPISpec).Methods(http.MethodGet)
	router.HandleFunc(%q, h.SwaggerUI).Methods(http.MethodGet)
%s}
`, module, module, interfaces.String(), fields.String(), rootPath, healthPath, specPath, uiPath, routes.String())
}

func mongoServiceInterfaceSource(model generator.MongoModel) string {
	var custom strings.Builder
	for _, method := range model.Methods {
		fmt.Fprintf(&custom, "\t%s(ctx context.Context, params map[string]any) (any, error)\n", method.Name)
	}
	return fmt.Sprintf(`type %sService interface {
	List(ctx context.Context) ([]domain.Document, error)
	Create(ctx context.Context, input domain.Document) (domain.Document, error)
	GetByID(ctx context.Context, id primitive.ObjectID) (domain.Document, error)
	Update(ctx context.Context, id primitive.ObjectID, input domain.Document) (domain.Document, error)
	Delete(ctx context.Context, id primitive.ObjectID) error
%s}

`, model.Name, custom.String())
}

func mongoSwaggerSource(ctx Context, swagger string) string {
	specPath := routePath(ctx.Config.Rest.HTTP.BasePath, ctx.Config.Rest.OpenAPI.SpecPath)
	return fmt.Sprintf(`package httpserver

import "net/http"

const swaggerSpec = %s

func (h HttpServer) OpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write([]byte(swaggerSpec))
}

func (h HttpServer) SwaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`+"`"+`<!doctype html><html><head><title>Generated REST API</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css"></head>
<body><div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>SwaggerUIBundle({url:%q,dom_id:'#swagger-ui'});</script></body></html>`+"`"+`))
}
`, strconv.Quote(swagger), specPath)
}

func mongoHTTPHelpersSource() string {
	return `package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"{{MODULE}}/internal/app/common/server"
	"{{MODULE}}/internal/app/domain"
)

func objectID(r *http.Request) (primitive.ObjectID, error) {
	return primitive.ObjectIDFromHex(mux.Vars(r)["id"])
}

func decodeDocument(r *http.Request) (domain.Document, error) {
	var input domain.Document
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		return nil, err
	}
	return input, nil
}

func parseMongoParam(raw string, typ string) (any, error) {
	switch typ {
	case "object_id":
		return primitive.ObjectIDFromHex(raw)
	case "int", "int32", "int64":
		return strconv.ParseInt(raw, 10, 64)
	case "float", "float64", "double", "decimal":
		return strconv.ParseFloat(raw, 64)
	case "bool", "boolean":
		return strconv.ParseBool(raw)
	default:
		return raw, nil
	}
}

func methodParams(r *http.Request, specs []methodParamSpec) (map[string]any, error) {
	params := map[string]any{}
	pathVars := mux.Vars(r)
	for _, spec := range specs {
		var raw string
		switch spec.Source {
		case "path":
			raw = pathVars[spec.Name]
		case "query", "":
			raw = r.URL.Query().Get(spec.Name)
		case "header":
			raw = r.Header.Get(spec.Name)
		case "body":
			body, err := decodeDocument(r)
			if err != nil {
				return nil, err
			}
			params["body"] = body
			params[spec.Name] = body
			continue
		default:
			raw = r.URL.Query().Get(spec.Name)
		}
		if raw == "" {
			if spec.Required {
				return nil, errors.New(spec.Name + " is required")
			}
			continue
		}
		value, err := parseMongoParam(raw, spec.Type)
		if err != nil {
			return nil, err
		}
		params[spec.Name] = value
	}
	return params, nil
}

type methodParamSpec struct {
	Name     string
	Type     string
	Source   string
	Required bool
}

func writeHandlerError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, domain.ErrNotFound) {
		server.NotFound("not-found", err, w, r)
		return
	}
	server.RespondWithError(err, w, r)
}
`
}

func mongoHandlersSource(model generator.MongoModel) string {
	var custom strings.Builder
	for _, method := range model.Methods {
		fmt.Fprint(&custom, mongoHandlerMethodSource(model, method))
	}
	return fmt.Sprintf(`package httpserver

import (
	"encoding/json"
	"net/http"

	"%s/internal/app/common/server"
)

func (h HttpServer) GetAll%s(w http.ResponseWriter, r *http.Request) {
	items, err := h.%sService.List(r.Context())
	if err != nil {
		writeHandlerError(w, r, err)
		return
	}
	server.RespondOK(items, w, r)
}

func (h HttpServer) Create%s(w http.ResponseWriter, r *http.Request) {
	input, err := decodeDocument(r)
	if err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}
	item, err := h.%sService.Create(r.Context(), input)
	if err != nil {
		writeHandlerError(w, r, err)
		return
	}
	server.RespondOK(item, w, r)
}

func (h HttpServer) Get%sByID(w http.ResponseWriter, r *http.Request) {
	id, err := objectID(r)
	if err != nil {
		server.BadRequest("invalid-id", err, w, r)
		return
	}
	item, err := h.%sService.GetByID(r.Context(), id)
	if err != nil {
		writeHandlerError(w, r, err)
		return
	}
	server.RespondOK(item, w, r)
}

func (h HttpServer) Update%s(w http.ResponseWriter, r *http.Request) {
	id, err := objectID(r)
	if err != nil {
		server.BadRequest("invalid-id", err, w, r)
		return
	}
	input, err := decodeDocument(r)
	if err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}
	item, err := h.%sService.Update(r.Context(), id, input)
	if err != nil {
		writeHandlerError(w, r, err)
		return
	}
	server.RespondOK(item, w, r)
}

func (h HttpServer) Delete%s(w http.ResponseWriter, r *http.Request) {
	id, err := objectID(r)
	if err != nil {
		server.BadRequest("invalid-id", err, w, r)
		return
	}
	if err := h.%sService.Delete(r.Context(), id); err != nil {
		writeHandlerError(w, r, err)
		return
	}
	server.RespondOK(map[string]bool{"deleted": true}, w, r)
}

var _ = json.Valid
%s`, mongoModulePlaceholder(), pluralizeGoName(model.Name), model.Name, model.Name, model.Name, model.Name, model.Name, model.Name, model.Name, model.Name, model.Name, custom.String())
}

func mongoHandlersTestSource(model generator.MongoModel) string {
	var customServiceMethods strings.Builder
	var customRoutes strings.Builder
	var customCases strings.Builder
	for _, method := range model.Methods {
		fmt.Fprintf(&customServiceMethods, `

func (fake%sService) %s(ctx context.Context, params map[string]any) (any, error) {
	return domain.Document{"ok": true}, nil
}
`, model.Name, method.Name)
		httpMethod := method.Method
		if httpMethod == "" {
			httpMethod = mongoHTTPMethodForOperation(method.Operation)
		}
		body := ""
		if mongoMethodNeedsBody(method) {
			body = `{"title":"updated"}`
		}
		fmt.Fprintf(&customRoutes, "\trouter.HandleFunc(%q, httpServer.%s).Methods(%s)\n", method.Path, method.Name, mongoHTTPMethodConst(httpMethod))
		fmt.Fprintf(&customCases, "\t\t{name: %q, method: %s, url: %q, body: `%s`},\n", method.Name, mongoHTTPMethodConst(httpMethod), mongoMethodTestURL(method), body)
	}
	base := "/" + strings.Trim(model.Collection, "/")
	plural := pluralizeGoName(model.Name)
	return fmt.Sprintf(`package httpserver

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"{{MODULE}}/internal/app/domain"
)

type fake%sService struct{}

func sample%sDocument() domain.Document {
	return domain.Document{"id": primitive.NewObjectID().Hex(), "title": "sample"}
}

func (fake%sService) List(ctx context.Context) ([]domain.Document, error) {
	return []domain.Document{sample%sDocument()}, nil
}

func (fake%sService) Create(ctx context.Context, input domain.Document) (domain.Document, error) {
	if input == nil {
		input = domain.Document{}
	}
	input["id"] = primitive.NewObjectID().Hex()
	return input, nil
}

func (fake%sService) GetByID(ctx context.Context, id primitive.ObjectID) (domain.Document, error) {
	return sample%sDocument(), nil
}

func (fake%sService) Update(ctx context.Context, id primitive.ObjectID, input domain.Document) (domain.Document, error) {
	if input == nil {
		input = domain.Document{}
	}
	input["id"] = id.Hex()
	return input, nil
}

func (fake%sService) Delete(ctx context.Context, id primitive.ObjectID) error {
	return nil
}
%s

func test%sHandlersRouter() *mux.Router {
	httpServer := HttpServer{%sService: fake%sService{}}
	router := mux.NewRouter()
	router.HandleFunc(%q, httpServer.GetAll%s).Methods(http.MethodGet)
	router.HandleFunc(%q, httpServer.Create%s).Methods(http.MethodPost)
%s	router.HandleFunc(%q, httpServer.Get%sByID).Methods(http.MethodGet)
	router.HandleFunc(%q, httpServer.Update%s).Methods(http.MethodPatch)
	router.HandleFunc(%q, httpServer.Delete%s).Methods(http.MethodDelete)
	return router
}

func Test%sHandlers(t *testing.T) {
	router := test%sHandlersRouter()
	id := primitive.NewObjectID().Hex()
	tests := []struct {
		name   string
		method string
		url    string
		body   string
	}{
		{name: "get all %s", method: http.MethodGet, url: %q},
		{name: "create %s", method: http.MethodPost, url: %q, body: `+"`"+`{"title":"sample"}`+"`"+`},
		{name: "get %s by id", method: http.MethodGet, url: %q + "/" + id},
		{name: "update %s", method: http.MethodPatch, url: %q + "/" + id, body: `+"`"+`{"title":"updated"}`+"`"+`},
		{name: "delete %s", method: http.MethodDelete, url: %q + "/" + id},
%s	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, bytes.NewBufferString(tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected status %%d, got %%d: %%s", http.StatusOK, rec.Code, rec.Body.String())
			}
		})
	}
}
`, model.Name, model.Name, model.Name, model.Name, model.Name, model.Name, model.Name, model.Name, model.Name, customServiceMethods.String(), model.Name, model.Name, model.Name, base, plural, base, model.Name, customRoutes.String(), base+"/{id}", model.Name, base+"/{id}", model.Name, base+"/{id}", model.Name, model.Name, model.Name, strings.ToLower(model.Collection), base, strings.ToLower(model.Name), base, strings.ToLower(model.Name), base, strings.ToLower(model.Name), base, strings.ToLower(model.Name), base, customCases.String())
}

func mongoHandlerMethodSource(model generator.MongoModel, method generator.MongoMethod) string {
	return fmt.Sprintf(`

func (h HttpServer) %s(w http.ResponseWriter, r *http.Request) {
	params, err := methodParams(r, %s)
	if err != nil {
		server.BadRequest("invalid-parameters", err, w, r)
		return
	}
	result, err := h.%sService.%s(r.Context(), params)
	if err != nil {
		writeHandlerError(w, r, err)
		return
	}
	server.RespondOK(result, w, r)
}
`, method.Name, mongoMethodParamsLiteral(method.Params), model.Name, method.Name)
}

func mongoAuthMiddlewareSource(ctx Context) string {
	if !authFeatures(ctx.Config).Enabled {
		return `package httpserver

import "net/http"

type ContextKey string

const ContextUserKey ContextKey = "user"

type BasicAuthConfig struct {
	Username string
	Password string
	Realm    string
	Roles    []string
}

func (h HttpServer) CheckRoles(next http.HandlerFunc, allowedRoles ...string) http.HandlerFunc {
	return next
}

func (h HttpServer) CheckAuthorizedUser(next http.HandlerFunc) http.HandlerFunc {
	return next
}
`
	}
	strategy := authStrategy(ctx)
	if strategy == "basic" {
		return `package httpserver

import (
	"context"
	"crypto/subtle"
	"net/http"

	"{{MODULE}}/internal/app/common/server"
)

type ContextKey string

const ContextUserKey ContextKey = "user"

type BasicAuthConfig struct {
	Username string
	Password string
	Realm    string
	Roles    []string
}

func (h HttpServer) CheckRoles(next http.HandlerFunc, allowedRoles ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := h.authenticateRequest(w, r)
		if !ok {
			return
		}
		if !hasAllowedRole(roles, allowedRoles) {
			server.Forbidden("not-authorized", nil, w, r)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ContextUserKey, user)))
	}
}

func (h HttpServer) CheckAuthorizedUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := h.authenticateRequest(w, r)
		if !ok {
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ContextUserKey, user)))
	}
}

func (h HttpServer) authenticateRequest(w http.ResponseWriter, r *http.Request) (any, []string, bool) {
	username, password, ok := r.BasicAuth()
	if !ok || h.BasicAuth.Username == "" || h.BasicAuth.Password == "" {
		h.basicChallenge(w)
		server.Unauthorised("missing-basic-auth", nil, w, r)
		return nil, nil, false
	}
	usernameOK := subtle.ConstantTimeCompare([]byte(username), []byte(h.BasicAuth.Username)) == 1
	passwordOK := subtle.ConstantTimeCompare([]byte(password), []byte(h.BasicAuth.Password)) == 1
	if !usernameOK || !passwordOK {
		h.basicChallenge(w)
		server.Unauthorised("invalid-basic-auth", nil, w, r)
		return nil, nil, false
	}
	return username, append([]string(nil), h.BasicAuth.Roles...), true
}

func (h HttpServer) basicChallenge(w http.ResponseWriter) {
	realm := h.BasicAuth.Realm
	if realm == "" {
		realm = "Restricted"
	}
	w.Header().Set("WWW-Authenticate", ` + "`" + `Basic realm="` + "`" + `+realm+` + "`" + `"` + "`" + `)
}

func hasAllowedRole(userRoles []string, allowedRoles []string) bool {
	if len(allowedRoles) == 0 {
		return true
	}
	for _, userRole := range userRoles {
		for _, allowedRole := range allowedRoles {
			if userRole == allowedRole {
				return true
			}
		}
	}
	return false
}
`
	}
	return `package httpserver

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt"

	"{{MODULE}}/internal/app/common/server"
)

type ContextKey string

const ContextUserKey ContextKey = "user"

type BasicAuthConfig struct {
	Username string
	Password string
	Realm    string
	Roles    []string
}

func (h HttpServer) CheckRoles(next http.HandlerFunc, allowedRoles ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := h.authenticateRequest(w, r)
		if !ok {
			return
		}
		if !hasAllowedRole(roles, allowedRoles) {
			server.Forbidden("not-authorized", nil, w, r)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ContextUserKey, user)))
	}
}

func (h HttpServer) CheckAuthorizedUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := h.authenticateRequest(w, r)
		if !ok {
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ContextUserKey, user)))
	}
}

func (h HttpServer) authenticateRequest(w http.ResponseWriter, r *http.Request) (any, []string, bool) {
	if h.JWTSecret == "" {
		server.Unauthorised("missing-jwt-secret", nil, w, r)
		return nil, nil, false
	}
	header := h.JWTHeader
	if header == "" {
		header = "Authorization"
	}
	scheme := h.JWTScheme
	if scheme == "" {
		scheme = "Bearer"
	}
	tokenValue := strings.TrimSpace(r.Header.Get(header))
	tokenValue = strings.TrimSpace(strings.TrimPrefix(tokenValue, scheme+" "))
	if tokenValue == "" {
		server.Unauthorised("missing-token", nil, w, r)
		return nil, nil, false
	}
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(tokenValue, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(h.JWTSecret), nil
	})
	if err != nil || !parsed.Valid {
		server.Unauthorised("invalid-token", err, w, r)
		return nil, nil, false
	}
	return claims, rolesFromClaims(claims, h.JWTRoleClaim), true
}

func rolesFromClaims(claims jwt.MapClaims, roleClaim string) []string {
	if roleClaim == "" {
		roleClaim = "roles"
	}
	value, ok := claims[roleClaim]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return nil
		}
		return []string{typed}
	case []any:
		roles := make([]string, 0, len(typed))
		for _, item := range typed {
			if role, ok := item.(string); ok {
				roles = append(roles, role)
			}
		}
		return roles
	default:
		return nil
	}
}

func hasAllowedRole(userRoles []string, allowedRoles []string) bool {
	if len(allowedRoles) == 0 {
		return true
	}
	for _, userRole := range userRoles {
		for _, allowedRole := range allowedRoles {
			if userRole == allowedRole {
				return true
			}
		}
	}
	return false
}
`
}

func mongoEnvSource(ctx Context) string {
	database := defaultMongoConnectionValue(ctx, "database")
	user := defaultMongoConnectionValue(ctx, "user")
	password := defaultMongoConnectionValue(ctx, "password")
	lines := []string{
		"MONGO_DB=" + database,
		"MONGO_USER=" + user,
		"MONGO_PASS=" + password,
		mongoURIEnv(ctx) + "=" + mongoConnectionURI(ctx, "localhost:27017"),
		"HTTP_ADDR=" + mongoHTTPAddr(ctx),
	}
	if authFeatures(ctx.Config).Enabled {
		if authStrategy(ctx) == "basic" {
			lines = append(lines, mongoAuthBasicUsernameEnv(ctx)+"=admin", mongoAuthBasicPasswordEnv(ctx)+"=change-me")
		} else {
			lines = append(lines, mongoAuthJWTSecretEnv(ctx)+"=change-me")
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func mongoDockerfileSource(ctx Context) string {
	cgo := "0"
	if ctx.Config.Rest.Docker.CGOEnabled {
		cgo = "1"
	}
	healthcheck := ""
	if ctx.Config.Rest.Docker.Healthcheck.Enabled.Bool() {
		healthcheck = fmt.Sprintf(
			"HEALTHCHECK --interval=%s --timeout=%s --retries=%d CMD wget -qO- http://127.0.0.1:%d%s || exit 1\n",
			ctx.Config.Rest.Docker.Healthcheck.Interval,
			ctx.Config.Rest.Docker.Healthcheck.Timeout,
			ctx.Config.Rest.Docker.Healthcheck.Retries,
			ctx.Config.Rest.HTTP.Port,
			routePath(ctx.Config.Rest.HTTP.BasePath, ctx.Config.Rest.Docker.Healthcheck.Path),
		)
	}
	return fmt.Sprintf(`FROM %s AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=%s go build -o /out/%s ./cmd

FROM %s
RUN addgroup -S %s && adduser -S %s -G %s
WORKDIR /app
COPY --from=build /out/%s /app/%s
EXPOSE %d
USER %s
%s
ENTRYPOINT ["/app/%s"]
`, ctx.Config.Rest.Docker.BuildImage, cgo, ctx.Config.Rest.Docker.Binary, ctx.Config.Rest.Docker.RuntimeImage, ctx.Config.Rest.Docker.User, ctx.Config.Rest.Docker.User, ctx.Config.Rest.Docker.User, ctx.Config.Rest.Docker.Binary, ctx.Config.Rest.Docker.Binary, ctx.Config.Rest.HTTP.Port, ctx.Config.Rest.Docker.User, strings.TrimSuffix(healthcheck, "\n"), ctx.Config.Rest.Docker.Binary)
}

func mongoDockerignoreSource() string {
	return `.git
tmp
dist
node_modules
coverage.out
`
}

func mongoDockerComposeSource(ctx Context) string {
	port := ctx.Config.Rest.HTTP.Port
	database := defaultMongoConnectionValue(ctx, "database")
	user := defaultMongoConnectionValue(ctx, "user")
	password := defaultMongoConnectionValue(ctx, "password")
	if user == "" {
		return fmt.Sprintf(`services:
  app:
    build:
      context: .
      dockerfile: %s
    environment:
      HTTP_ADDR: 0.0.0.0:%d
      %s: mongodb://mongo:27017
    ports:
      - "%d:%d"
    depends_on:
      mongo:
        condition: service_healthy

  mongo:
    image: mongo:7
    ports:
      - "27017:27017"
    volumes:
      - mongo_data:/data/db
    healthcheck:
      test: ["CMD-SHELL", "mongosh --quiet --eval 'quit(db.runCommand({ ping: 1 }).ok ? 0 : 1)'"]
      interval: 5s
      timeout: 3s
      retries: 20

volumes:
  mongo_data:
`, ctx.Config.Rest.Docker.Output, port, mongoURIEnv(ctx), port, port)
	}
	return fmt.Sprintf(`services:
  app:
    build:
      context: .
      dockerfile: %s
    environment:
      HTTP_ADDR: 0.0.0.0:%d
      %s: %q
    ports:
      - "%d:%d"
    depends_on:
      mongo-init:
        condition: service_completed_successfully

  mongo:
    image: mongo:7
    environment:
      MONGO_INITDB_ROOT_USERNAME: rest_admin
      MONGO_INITDB_ROOT_PASSWORD: rest_admin_password
    ports:
      - "27017:27017"
    volumes:
      - mongo_data:/data/db
    healthcheck:
      test: ["CMD-SHELL", "mongosh --quiet --username rest_admin --password rest_admin_password --authenticationDatabase admin --eval 'quit(db.runCommand({ ping: 1 }).ok ? 0 : 1)'"]
      interval: 5s
      timeout: 3s
      retries: 20

  mongo-init:
    image: mongo:7
    depends_on:
      mongo:
        condition: service_healthy
    environment:
      MONGO_DB: %q
      MONGO_USER: %q
      MONGO_PASS: %q
    entrypoint: ["/bin/sh", "-ec"]
    command:
      - |
        mongosh mongodb://rest_admin:rest_admin_password@mongo:27017/admin?authSource=admin --quiet --eval '
          const target = db.getSiblingDB(process.env.MONGO_DB);
          const roles = [{ role: "readWrite", db: process.env.MONGO_DB }];
          if (target.getUser(process.env.MONGO_USER)) {
            target.updateUser(process.env.MONGO_USER, { pwd: process.env.MONGO_PASS, roles });
          } else {
            target.createUser({ user: process.env.MONGO_USER, pwd: process.env.MONGO_PASS, roles });
          }
        '

volumes:
  mongo_data:
`, ctx.Config.Rest.Docker.Output, port, mongoURIEnv(ctx), mongoConnectionURI(ctx, "mongo:27017"), port, port, database, user, password)
}

func mongoConnectionURI(ctx Context, host string) string {
	database := defaultMongoConnectionValue(ctx, "database")
	user := defaultMongoConnectionValue(ctx, "user")
	password := defaultMongoConnectionValue(ctx, "password")
	uri := &url.URL{Scheme: "mongodb", Host: host, Path: "/" + database}
	if user != "" {
		uri.User = url.UserPassword(user, password)
		query := uri.Query()
		query.Set("authSource", database)
		uri.RawQuery = query.Encode()
	}
	return uri.String()
}

func defaultMongoConnectionValue(ctx Context, field string) string {
	if ctx.Config.Mongo != nil {
		switch field {
		case "database":
			if ctx.Config.Mongo.Connection.Database != "" {
				return ctx.Config.Mongo.Connection.Database
			}
		case "user":
			return ctx.Config.Mongo.Connection.UserName
		case "password":
			return ctx.Config.Mongo.Connection.UserPassword
		}
	}
	switch field {
	case "database":
		return "app_db"
	case "user":
		return "app_user"
	case "password":
		return "app_password"
	default:
		return ""
	}
}

func addMongoRoute(routes *strings.Builder, ctx Context, method, path, handler string) {
	method = strings.ToUpper(method)
	methodConst := mongoHTTPMethodConst(method)
	if methodConst == "" {
		return
	}
	fullPath := routePath(ctx.Config.Rest.HTTP.BasePath, normalizeMongoEndpointPath(path))
	wrapped := mongoAuthHandler(authFeatures(ctx.Config), ctx.Config.Rest.HTTP.BasePath, method, normalizeMongoEndpointPath(path), handler)
	fmt.Fprintf(routes, "\trouter.HandleFunc(%q, %s).Methods(%s)\n", fullPath, wrapped, methodConst)
}

func mongoAuthHandler(auth generator.AuthFeatures, basePath, method, path, handler string) string {
	if !auth.Enabled {
		return handler
	}
	key := strings.ToUpper(method) + " " + routePath(basePath, path)
	policy, ok := auth.Policies[key]
	if ok && policy.Public {
		return handler
	}
	if !ok && strings.EqualFold(auth.DefaultPolicy, "allow") {
		return handler
	}
	if len(policy.Roles) == 0 {
		return "h.CheckAuthorizedUser(" + handler + ")"
	}
	quoted := make([]string, 0, len(policy.Roles))
	for _, role := range policy.Roles {
		quoted = append(quoted, strconv.Quote(role))
	}
	return "h.CheckRoles(" + handler + ", " + strings.Join(quoted, ", ") + ")"
}

func mongoMethodParamsLiteral(params []generator.MongoMethodParam) string {
	if len(params) == 0 {
		return "nil"
	}
	var items []string
	for _, param := range params {
		items = append(items, fmt.Sprintf(`{Name:%q, Type:%q, Source:%q, Required:%t}`, param.Name, param.Type, param.Source, param.Required))
	}
	return "[]methodParamSpec{" + strings.Join(items, ", ") + "}"
}

func mongoMethodFiltersLiteral(filters []generator.MongoFilter) string {
	if len(filters) == 0 {
		return "nil"
	}
	var items []string
	for _, filter := range filters {
		items = append(items, fmt.Sprintf(`{Field:%q, Op:%q, Param:%q, Value:%s}`, filter.Field, filter.Op, filter.Param, mongoFilterValueLiteral(filter.Value)))
	}
	return "[]methodFilter{" + strings.Join(items, ", ") + "}"
}

func mongoFilterValueLiteral(value any) string {
	switch typed := value.(type) {
	case nil:
		return "nil"
	case string:
		return strconv.Quote(typed)
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			values = append(values, mongoFilterValueLiteral(item))
		}
		return "[]any{" + strings.Join(values, ", ") + "}"
	case []string:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			values = append(values, strconv.Quote(item))
		}
		return "[]any{" + strings.Join(values, ", ") + "}"
	case int, int64, int32, float64, float32, bool:
		return fmt.Sprintf("%v", typed)
	default:
		return strconv.Quote(fmt.Sprint(typed))
	}
}

func mongoHTTPMethodConst(method string) string {
	switch strings.ToUpper(method) {
	case "GET":
		return "http.MethodGet"
	case "POST":
		return "http.MethodPost"
	case "PUT":
		return "http.MethodPut"
	case "PATCH":
		return "http.MethodPatch"
	case "DELETE":
		return "http.MethodDelete"
	case "OPTIONS":
		return "http.MethodOptions"
	default:
		return ""
	}
}

func mongoMethodNeedsBody(method generator.MongoMethod) bool {
	for _, param := range method.Params {
		if param.Source == "body" {
			return true
		}
	}
	switch strings.ToLower(method.Operation) {
	case "update_one", "aggregate":
		return strings.EqualFold(method.Method, "POST") || strings.EqualFold(method.Method, "PUT") || strings.EqualFold(method.Method, "PATCH")
	default:
		return false
	}
}

func mongoMethodTestURL(method generator.MongoMethod) string {
	path := method.Path
	var query []string
	for _, param := range method.Params {
		value := mongoMethodTestParamValue(param.Type)
		switch param.Source {
		case "path":
			path = strings.ReplaceAll(path, "{"+param.Name+"}", value)
		case "query", "":
			if param.Required {
				query = append(query, param.Name+"="+value)
			}
		}
	}
	for _, param := range method.Params {
		path = strings.ReplaceAll(path, "{"+param.Name+"}", mongoMethodTestParamValue(param.Type))
	}
	if len(query) > 0 {
		path += "?" + strings.Join(query, "&")
	}
	return path
}

func mongoMethodTestParamValue(typ string) string {
	switch strings.ToLower(typ) {
	case "object_id":
		return "000000000000000000000001"
	case "int", "int32", "int64":
		return "1"
	case "float", "float64", "double", "decimal":
		return "1.5"
	case "bool", "boolean":
		return "true"
	default:
		return "sample"
	}
}

func mongoHTTPAddr(ctx Context) string {
	host := ctx.Config.Rest.HTTP.Host
	if host == "" {
		host = "0.0.0.0"
	}
	port := ctx.Config.Rest.HTTP.Port
	if port == 0 {
		port = 8080
	}
	return fmt.Sprintf("%s:%d", host, port)
}

func mongoTimeout(ctx Context) string {
	timeout := ctx.Config.Mongo.Connection.Timeout
	if timeout == "" {
		timeout = "10s"
	}
	return timeout
}

func mongoURIEnv(ctx Context) string {
	if ctx.Config.Mongo != nil && ctx.Config.Mongo.Connection.URIEnv != "" {
		return ctx.Config.Mongo.Connection.URIEnv
	}
	return "MONGO_URI"
}

func mongoLoggerSetup(ctx Context) string {
	if !ctx.Config.Rest.Logging.Enabled.Bool() {
		return ""
	}
	return `logger, err := logging.New()
	if err != nil {
		return err
	}
	defer func() { _ = logger.Sync() }()`
}

func mongoStartupLog(ctx Context) string {
	if ctx.Config.Rest.Logging.Enabled.Bool() {
		return `logger.Info("starting HTTP server", zap.String("addr", server.Addr))`
	}
	return `log.Printf("listening on %s", server.Addr)`
}

func mongoShutdownLog(ctx Context) string {
	if ctx.Config.Rest.Logging.Enabled.Bool() {
		return `logger.Error("HTTP server shutdown error", zap.Error(err))`
	}
	return `log.Printf("HTTP server shutdown error: %v", err)`
}

func mongoReadinessRouteSource(ctx Context, path string) string {
	if !ctx.Config.Rest.HTTP.Readiness.Enabled.Bool() {
		return ""
	}
	if path == "" || path == "/" {
		path = routePath(ctx.Config.Rest.HTTP.BasePath, "/ready")
	}
	return fmt.Sprintf(`router.HandleFunc(%q, func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := client.Ping(ctx, nil); err != nil {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"status\":\"ready\"}\n"))
	}).Methods(http.MethodGet)`, path)
}

func mongoMetricsRouteSource(ctx Context, path string) string {
	if !ctx.Config.Rest.Observability.Metrics.Enabled.Bool() {
		return ""
	}
	if path == "" || path == "/" {
		path = routePath(ctx.Config.Rest.HTTP.BasePath, "/metrics")
	}
	return fmt.Sprintf(`router.HandleFunc(%q, metrics.Handler).Methods(http.MethodGet)`, path)
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func mongoQuotedSlice(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, strconv.Quote(value))
	}
	return strings.Join(quoted, ", ")
}

func mongoBoolMapEntries(values []string) string {
	entries := make([]string, 0, len(values))
	for _, value := range values {
		entries = append(entries, strconv.Quote(value)+": true")
	}
	return strings.Join(entries, ", ")
}

func mongoAuthJWTSecretEnv(ctx Context) string {
	value := authFeatures(ctx.Config).JWTSecretEnv
	if value == "" {
		return "JWT_SECRET"
	}
	return value
}

func mongoAuthJWTHeader(ctx Context) string {
	value := authFeatures(ctx.Config).JWTHeader
	if value == "" {
		return "Authorization"
	}
	return value
}

func mongoAuthJWTScheme(ctx Context) string {
	value := authFeatures(ctx.Config).JWTScheme
	if value == "" {
		return "Bearer"
	}
	return value
}

func mongoAuthJWTIssuer(ctx Context) string {
	return authFeatures(ctx.Config).JWTIssuer
}

func mongoAuthJWTAudience(ctx Context) string {
	return authFeatures(ctx.Config).JWTAudience
}

func mongoAuthRoleClaim(ctx Context) string {
	value := authFeatures(ctx.Config).RoleClaim
	if value == "" {
		return "roles"
	}
	return value
}

func mongoAuthBasicUsernameEnv(ctx Context) string {
	value := authFeatures(ctx.Config).BasicUsernameEnv
	if value == "" {
		return "BASIC_AUTH_USERNAME"
	}
	return value
}

func mongoAuthBasicPasswordEnv(ctx Context) string {
	value := authFeatures(ctx.Config).BasicPasswordEnv
	if value == "" {
		return "BASIC_AUTH_PASSWORD"
	}
	return value
}

func mongoAuthBasicRealm(ctx Context) string {
	value := authFeatures(ctx.Config).BasicRealm
	if value == "" {
		return "Restricted"
	}
	return value
}

func mongoAuthBasicRoles(ctx Context) []string {
	return authFeatures(ctx.Config).BasicRoles
}

func mongoModulePlaceholder() string {
	return "{{MODULE}}"
}
