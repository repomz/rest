package generator

const appMainTemplate = `package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	{{- if or .Features.Auth.Enabled .Features.HTTP.GracefulShutdown }}
	"os"
	{{- end }}
	{{- if .Features.HTTP.GracefulShutdown }}
	"os/signal"
	"syscall"
	{{- end }}
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	{{- if .Features.Logging.Enabled }}
	"go.uber.org/zap"
	{{- end }}

	"{{ .Module }}/internal/app/config"
	"{{ .DBImport }}"
	{{- if .Features.Logging.Enabled }}
	"{{ .Module }}/internal/app/logging"
	{{- end }}
	"{{ .Module }}/internal/app/repository/pgrepo"
	"{{ .Module }}/internal/app/services"
	"{{ .Module }}/internal/app/transport/httpserver"
	{{- if or .Features.HTTP.Health .Features.HTTP.Readiness }}
	"{{ .Module }}/internal/app/common/server"
	{{- end }}
	{{- if or .Features.HTTP.CORS .Features.HTTP.Recovery .Features.HTTP.RequestID .Features.HTTP.SecurityHeaders .Features.HTTP.RateLimit (gt .Features.HTTP.MaxBodyBytes 0) }}
	"{{ .Module }}/internal/app/transport/middleware"
	{{- end }}
	{{- if .Features.Metrics.Enabled }}
	"{{ .Module }}/internal/app/metrics"
	{{- end }}
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func Dial(dsn string) (*sql.DB, *db.Queries, error) {
	if dsn == "" {
		return nil, nil, errors.New("no postgres DSN provided")
	}
	dbase, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("sql.Open failed: %w", err)
	}
	dbase.SetMaxOpenConns(10)
	dbase.SetMaxIdleConns(10)
	dbase.SetConnMaxIdleTime(5 * time.Minute)
	dbase.SetConnMaxLifetime(time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := dbase.PingContext(ctx); err != nil {
		_ = dbase.Close()
		return nil, nil, fmt.Errorf("postgres ping failed: %w", err)
	}
	return dbase, db.New(dbase), nil
}

func run() error {
	cfg := config.Read()
	{{- if .Features.Logging.Enabled }}
	logger, err := logging.New()
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	defer func() { _ = logger.Sync() }()
	{{- end }}
	dbase, queries, err := Dial(cfg.DB_DSN)
	if err != nil {
		return err
	}
	defer dbase.Close()

{{- range .Tables }}
	{{ .Singular }}Repo := pgrepo.New{{ .GoName }}Repo(queries)
	{{ .Singular }}Service := services.New{{ .GoName }}Service({{ .Singular }}Repo)
{{- end }}
	{{- if and .Features.Auth.Enabled (eq .Features.Auth.Strategy "jwt") }}
	tokenService := services.NewTokenService(services.TokenConfig{
		TTL: mustDuration({{ printf "%q" (defaultString .Features.Auth.JWTAccessTokenTTL "15m") }}),
		RefreshToken: {{ .Features.Auth.JWTRefreshToken }},
		Secret: os.Getenv({{ printf "%q" .Features.Auth.JWTSecretEnv }}),
		Issuer: {{ printf "%q" .Features.Auth.JWTIssuer }},
		Audience: {{ printf "%q" .Features.Auth.JWTAudience }},
	})
	{{- end }}
	{{- if and .Features.Auth.Enabled (eq .Features.Auth.Strategy "basic") }}
	basicAuthConfig := httpserver.BasicAuthConfig{
		Username: os.Getenv({{ printf "%q" .Features.Auth.BasicUsernameEnv }}),
		Password: os.Getenv({{ printf "%q" .Features.Auth.BasicPasswordEnv }}),
		Realm: {{ printf "%q" .Features.Auth.BasicRealm }},
		Roles: []string{
			{{- range .Features.Auth.BasicRoles }}{{ printf "%q" . }},{{ end }}
		},
	}
	{{- end }}
	httpServer := httpserver.NewHttpServer(
{{- range .Tables }}
		{{ .Singular }}Service,
{{- end }}
{{- if and .Features.Auth.Enabled (eq .Features.Auth.Strategy "jwt") }}
		tokenService,
{{- end }}
{{- if and .Features.Auth.Enabled (eq .Features.Auth.Strategy "basic") }}
		basicAuthConfig,
{{- end }}
	)
{{- if not (anyGeneratedEndpointTests .Tables) }}
	_ = httpServer
{{- end }}

	router := mux.NewRouter()
	apiRouter := router
	{{- if ne .Features.HTTP.BasePath "/" }}
	apiRouter = router.PathPrefix({{ printf "%q" .Features.HTTP.BasePath }}).Subrouter()
	{{- end }}
	apiRouter.HandleFunc("/", {{ authHandler .Features.Auth .Features.HTTP.BasePath "GET" "/" "func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(\"Generated API\")) }" }}).Methods(http.MethodGet)
	{{- if and .Features.Auth.Enabled (eq .Features.Auth.Strategy "jwt") }}
	apiRouter.HandleFunc("/signup", httpServer.SignUp).Methods(http.MethodPost)
	apiRouter.HandleFunc("/signin", httpServer.SignIn).Methods(http.MethodPost)
	{{- end }}
	{{- if .Features.HTTP.Health }}
	apiRouter.HandleFunc({{ printf "%q" (defaultString .Features.HTTP.HealthPath "/health") }}, {{ authHandler .Features.Auth .Features.HTTP.BasePath "GET" (defaultString .Features.HTTP.HealthPath "/health") "func(w http.ResponseWriter, _ *http.Request) { server.RespondOK(map[string]string{\"status\": \"ok\"}, w, nil) }" }}).Methods(http.MethodGet)
	{{- end }}
	{{- if .Features.HTTP.Readiness }}
	apiRouter.HandleFunc({{ printf "%q" (defaultString .Features.HTTP.ReadinessPath "/ready") }}, {{ authHandler .Features.Auth .Features.HTTP.BasePath "GET" (defaultString .Features.HTTP.ReadinessPath "/ready") "func(w http.ResponseWriter, r *http.Request) { ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second); defer cancel(); if err := dbase.PingContext(ctx); err != nil { server.InternalError(\"readiness-check-failed\", err, w, r); return }; server.RespondOK(map[string]string{\"status\": \"ready\"}, w, r) }" }}).Methods(http.MethodGet)
	{{- end }}
	{{- if .Features.Metrics.Enabled }}
	metrics.SetDBStatsProvider(dbase.Stats)
	apiRouter.HandleFunc({{ printf "%q" (defaultString .Features.Metrics.Path "/metrics") }}, {{ authHandler .Features.Auth .Features.HTTP.BasePath "GET" (defaultString .Features.Metrics.Path "/metrics") "metrics.Handler" }}).Methods(http.MethodGet)
	{{- end }}
	{{- if .Features.OpenAPI.Enabled }}
	apiRouter.HandleFunc({{ printf "%q" (defaultString .Features.OpenAPI.SpecPath "/swagger/openapi.yaml") }}, {{ authHandler .Features.Auth .Features.HTTP.BasePath "GET" (defaultString .Features.OpenAPI.SpecPath "/swagger/openapi.yaml") "httpserver.SwaggerSpec" }}).Methods(http.MethodGet)
	{{- end }}
	{{- if and .Features.OpenAPI.Enabled .Features.OpenAPI.WithUI }}
	apiRouter.HandleFunc({{ printf "%q" (defaultString .Features.OpenAPI.UIPath "/swagger/index.html") }}, {{ authHandler .Features.Auth .Features.HTTP.BasePath "GET" (defaultString .Features.OpenAPI.UIPath "/swagger/index.html") "httpserver.SwaggerUI" }}).Methods(http.MethodGet)
	{{- end }}
{{- range .Tables }}
{{- if not (and (eq $.Features.Auth.Strategy "jwt") (isAuthIdentityTable . $.Features.Auth)) }}
{{- if .Queries.GetAll }}
	apiRouter.HandleFunc("{{ .RouteBase }}", {{ authHandler $.Features.Auth $.Features.HTTP.BasePath "GET" .RouteBase (printf "httpServer.GetAll%s" .GoPlural) }}).Methods(http.MethodGet)
{{- end }}
{{- if .Queries.Create }}
	apiRouter.HandleFunc("{{ .RouteBase }}", {{ authHandler $.Features.Auth $.Features.HTTP.BasePath "POST" .RouteBase (printf "httpServer.Create%s" .GoName) }}).Methods(http.MethodPost)
{{- end }}
{{- if .Queries.DeleteAll }}
	apiRouter.HandleFunc("{{ .RouteBase }}", {{ authHandler $.Features.Auth $.Features.HTTP.BasePath "DELETE" .RouteBase (printf "httpServer.DeleteAll%s" .GoPlural) }}).Methods(http.MethodDelete)
{{- end }}
{{- range .Endpoints }}
	apiRouter.HandleFunc("{{ .Path }}", {{ authHandler $.Features.Auth $.Features.HTTP.BasePath .Method .Path (printf "httpServer.%s" .Name) }}).Methods("{{ .Method }}")
{{- end }}
{{- if .Queries.GetByID }}
	apiRouter.HandleFunc("{{ .RouteBase }}/{id}", {{ authHandler $.Features.Auth $.Features.HTTP.BasePath "GET" (printf "%s/{id}" .RouteBase) (printf "httpServer.Get%sByID" .GoName) }}).Methods(http.MethodGet)
{{- end }}
{{- if .Queries.Delete }}
	apiRouter.HandleFunc("{{ .RouteBase }}/{id}", {{ authHandler $.Features.Auth $.Features.HTTP.BasePath "DELETE" (printf "%s/{id}" .RouteBase) (printf "httpServer.Delete%s" .GoName) }}).Methods(http.MethodDelete)
{{- end }}
{{- end }}
{{ end }}

	var handler http.Handler = router
	{{- if gt .Features.HTTP.MaxBodyBytes 0 }}
	handler = middleware.MaxBodyBytes({{ .Features.HTTP.MaxBodyBytes }}, handler)
	{{- end }}
	{{- if .Features.HTTP.RateLimit }}
	handler = middleware.RateLimit({{ .Features.HTTP.RateLimitRequests }}, mustDuration({{ printf "%q" (defaultString .Features.HTTP.RateLimitWindow "1m") }}), handler)
	{{- end }}
	{{- if .Features.HTTP.CORS }}
	handler = middleware.CORS(handler)
	{{- end }}
	{{- if .Features.HTTP.SecurityHeaders }}
	handler = middleware.SecurityHeaders(handler)
	{{- end }}
	{{- if .Features.HTTP.Recovery }}
	handler = middleware.Recovery(handler)
	{{- end }}
	{{- if .Features.HTTP.RequestID }}
	handler = middleware.RequestID(handler)
	{{- end }}
	{{- if .Features.Metrics.Enabled }}
	handler = metrics.Middleware(handler)
	{{- end }}
	{{- if .Features.Logging.Enabled }}
	handler = logging.Middleware(logger, handler)
	{{- end }}
	srv := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: handler,
		ReadHeaderTimeout: mustDuration({{ printf "%q" (defaultString .Features.HTTP.ReadHeaderTimeout "5s") }}),
		ReadTimeout: mustDuration({{ printf "%q" (defaultString .Features.HTTP.ReadTimeout "30s") }}),
		WriteTimeout: mustDuration({{ printf "%q" (defaultString .Features.HTTP.WriteTimeout "30s") }}),
		IdleTimeout: mustDuration({{ printf "%q" (defaultString .Features.HTTP.IdleTimeout "60s") }}),
	}
	{{- if .Features.HTTP.GracefulShutdown }}
	stopped := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		<-sigint
		ctx, cancel := context.WithTimeout(context.Background(), mustDuration({{ printf "%q" (defaultString .Features.HTTP.ShutdownTimeout "10s") }}))
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			{{- if .Features.Logging.Enabled }}
			logger.Error("HTTP server shutdown error", zap.Error(err))
			{{- else }}
			log.Printf("HTTP server shutdown error: %v", err)
			{{- end }}
		}
		close(stopped)
	}()

	{{- if .Features.Logging.Enabled }}
	logger.Info("starting HTTP server", zap.String("addr", cfg.HTTPAddr))
	{{- else }}
	log.Printf("Starting HTTP server on %s", cfg.HTTPAddr)
	{{- end }}
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	<-stopped
	return nil
	{{- else }}
	return srv.ListenAndServe()
	{{- end }}
}

func mustDuration(value string) time.Duration {
	duration, err := time.ParseDuration(value)
	if err != nil {
		panic(err)
	}
	return duration
}
`

const makefileTemplate = `-include .env

APP_NAME ?= app
BUILD_DIR ?= ./bin
{{ if .Features.Build.InitDB }}
DB_SCRIPT ?= init_db.sh
{{ end }}
DB_NAME ?= {{ defaultString .Features.Build.DBName "app_db" }}
DB_USER ?= {{ defaultString .Features.Build.DBUser "app_user" }}
DB_PASS ?= {{ defaultString .Features.Build.DBPassword "app_password" }}
DB_DRIVER ?= postgres
MIGRATIONS_DIR ?= ./{{ defaultString .Features.Build.MigrationsPath "internal/sql/migrations" }}
DB_OPTIONS ?= {{ defaultString .Features.Build.DBOptions "sslmode=disable" }}
DB_DSN ?= postgres://$(DB_USER):$(DB_PASS)@localhost:5432/$(DB_NAME)?$(DB_OPTIONS)
HTTP_ADDR ?= {{ httpAddr .Features.HTTP.Host .Features.HTTP.Port }}
DEBUG_ERRORS ?= 0
GOCACHE ?= $(CURDIR)/.cache/go-build
GOLANGCI_LINT_VERSION ?= v1.64.8
REST ?= rest

export

.PHONY: build rest-gen run test clean{{ if .Features.Build.InitDB }} db{{ end }}{{ if .Features.Build.InitMigration }} migrate-status migrate-up migrate-down migrate-create{{ end }} install-lint lint

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd

rest-gen:
	$(REST) gen

run:
	@mkdir -p $(BUILD_DIR) && \
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd && \
	HTTP_ADDR=$(HTTP_ADDR) \
	DB_DSN=$(DB_DSN) \
	DEBUG_ERRORS=$(DEBUG_ERRORS) \
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

{{ if .Features.Build.InitDB }}
db:
	@test -f $(DB_SCRIPT) || { echo "Error: $(DB_SCRIPT) is missing"; exit 1; }
	@chmod +x $(DB_SCRIPT)
	@./$(DB_SCRIPT)
{{ end }}

{{ if .Features.Build.InitMigration }}
migrate-status:
	goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) $(DB_DSN) status

migrate-up:
	goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) $(DB_DSN) up

migrate-down:
	goose -dir $(MIGRATIONS_DIR) $(DB_DRIVER) $(DB_DSN) down

migrate-create:
	@read -p "Migration name: " name; \
	goose -dir $(MIGRATIONS_DIR) create $$name sql
{{ end }}
`

const initDBTemplate = `#!/usr/bin/env bash
set -euo pipefail

if [[ -f .env ]]; then
	set -a
	source .env
	set +a
fi

{{ if eq .Features.Build.Backend "mongo" }}
: "${MONGO_DB:={{ defaultString .Features.Mongo.Database "app_db" }}}"
: "${MONGO_USER:={{ defaultString .Features.Mongo.User "app_user" }}}"
: "${MONGO_PASS:={{ defaultString .Features.Mongo.Password "app_password" }}}"
: "${MONGO_RUNTIME:=docker}"
: "${MONGO_IMAGE:=mongo:7}"
: "${MONGO_HOST:=127.0.0.1}"
: "${MONGO_PORT:=27017}"
: "${MONGO_ADMIN_USER:=rest_admin}"
: "${MONGO_ADMIN_PASS:=rest_admin_password}"
: "${MONGO_ADMIN_URI:=mongodb://$MONGO_ADMIN_USER:$MONGO_ADMIN_PASS@$MONGO_HOST:$MONGO_PORT/admin?authSource=admin}"
: "${MONGO_CONTAINER:=rest-${MONGO_DB//[^a-zA-Z0-9_.-]/-}-mongo}"
: "${MONGO_VOLUME:=${MONGO_CONTAINER}-data}"

configure_mongo_user_eval='
const databaseName = process.env.REST_APP_DB;
const username = process.env.REST_APP_USER;
const password = process.env.REST_APP_PASS;
const applicationDB = db.getSiblingDB(databaseName);
const roles = [{ role: "readWrite", db: databaseName }];
if (applicationDB.getUser(username)) {
	applicationDB.updateUser(username, { pwd: password, roles: roles });
} else {
	applicationDB.createUser({ user: username, pwd: password, roles: roles });
}
applicationDB.runCommand({ ping: 1 });
'

if [[ "$MONGO_RUNTIME" == "docker" ]]; then
	command -v docker >/dev/null 2>&1 || {
		echo "Error: Docker is required for the default MongoDB setup." >&2
		echo "Install/start Docker, or set MONGO_RUNTIME=local and MONGO_ADMIN_URI." >&2
		exit 1
	}
	docker info >/dev/null 2>&1 || {
		echo "Error: the Docker daemon is unavailable." >&2
		exit 1
	}

	if docker container inspect "$MONGO_CONTAINER" >/dev/null 2>&1; then
		docker start "$MONGO_CONTAINER" >/dev/null
	else
		echo "Starting MongoDB container '$MONGO_CONTAINER'..."
		if ! docker run -d \
			--name "$MONGO_CONTAINER" \
			-p "$MONGO_PORT:27017" \
			-v "$MONGO_VOLUME:/data/db" \
			-e "MONGO_INITDB_ROOT_USERNAME=$MONGO_ADMIN_USER" \
			-e "MONGO_INITDB_ROOT_PASSWORD=$MONGO_ADMIN_PASS" \
			"$MONGO_IMAGE" >/dev/null; then
			echo "Error: MongoDB container could not start. Check whether port $MONGO_PORT is already in use." >&2
			exit 1
		fi
	fi

	echo "Waiting for MongoDB..."
	ready=0
	for _ in {1..60}; do
		if docker exec "$MONGO_CONTAINER" mongosh \
			--quiet \
			--username "$MONGO_ADMIN_USER" \
			--password "$MONGO_ADMIN_PASS" \
			--authenticationDatabase admin \
			--eval 'quit(db.runCommand({ ping: 1 }).ok ? 0 : 1)' >/dev/null 2>&1; then
			ready=1
			break
		fi
		sleep 1
	done
	if [[ "$ready" != "1" ]]; then
		echo "Error: MongoDB did not become ready within 60 seconds." >&2
		docker logs "$MONGO_CONTAINER" >&2 || true
		exit 1
	fi

	docker exec \
		-e "REST_APP_DB=$MONGO_DB" \
		-e "REST_APP_USER=$MONGO_USER" \
		-e "REST_APP_PASS=$MONGO_PASS" \
		"$MONGO_CONTAINER" mongosh \
		--quiet \
		--username "$MONGO_ADMIN_USER" \
		--password "$MONGO_ADMIN_PASS" \
		--authenticationDatabase admin \
		--eval "$configure_mongo_user_eval" >/dev/null

	docker exec \
		-e "REST_APP_DB=$MONGO_DB" \
		-e "REST_APP_USER=$MONGO_USER" \
		-e "REST_APP_PASS=$MONGO_PASS" \
		"$MONGO_CONTAINER" mongosh \
		--quiet \
		--username "$MONGO_USER" \
		--password "$MONGO_PASS" \
		--authenticationDatabase "$MONGO_DB" \
		"$MONGO_DB" \
		--eval 'quit(db.runCommand({ ping: 1 }).ok ? 0 : 1)' >/dev/null
else
	command -v mongosh >/dev/null 2>&1 || {
		echo "Error: mongosh is required when MONGO_RUNTIME=local." >&2
		exit 1
	}
	REST_APP_DB="$MONGO_DB" REST_APP_USER="$MONGO_USER" REST_APP_PASS="$MONGO_PASS" \
		mongosh "$MONGO_ADMIN_URI" --quiet --eval "$configure_mongo_user_eval" >/dev/null
	mongosh "mongodb://$MONGO_USER:$MONGO_PASS@$MONGO_HOST:$MONGO_PORT/$MONGO_DB?authSource=$MONGO_DB" \
		--quiet --eval 'quit(db.runCommand({ ping: 1 }).ok ? 0 : 1)' >/dev/null
fi

echo "Done: MongoDB database '$MONGO_DB' is ready for user '$MONGO_USER'."
echo "Application URI: mongodb://$MONGO_USER:***@$MONGO_HOST:$MONGO_PORT/$MONGO_DB?authSource=$MONGO_DB"
{{ else }}
: "${DB_NAME:={{ defaultString .Features.Build.DBName "app_db" }}}"
: "${DB_USER:={{ defaultString .Features.Build.DBUser "app_user" }}}"
: "${DB_PASS:={{ defaultString .Features.Build.DBPassword "app_password" }}}"
: "${DB_RUNTIME:=docker}"
: "${DB_IMAGE:=postgres:17-alpine}"
: "${DB_HOST:=127.0.0.1}"
: "${DB_PORT:=5432}"
: "${DB_ADMIN_USER:=rest_admin}"
: "${DB_ADMIN_PASS:=rest_admin_password}"
: "${DB_ADMIN_DSN:=postgres://$DB_ADMIN_USER:$DB_ADMIN_PASS@$DB_HOST:$DB_PORT/postgres?sslmode=disable}"
: "${DB_DSN:=postgres://$DB_USER:$DB_PASS@$DB_HOST:$DB_PORT/$DB_NAME?{{ defaultString .Features.Build.DBOptions "sslmode=disable" }}}"
: "${DB_CONTAINER:=rest-${DB_NAME//[^a-zA-Z0-9_.-]/-}-postgres}"
: "${DB_VOLUME:=${DB_CONTAINER}-data}"
: "${MIGRATIONS_DIR:=./{{ defaultString .Features.Build.MigrationsPath "internal/sql/migrations" }}}"

if [[ ! "$DB_NAME" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || [[ ! "$DB_USER" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
	echo "Error: DB_NAME and DB_USER must contain only letters, digits, and underscores and may not start with a digit." >&2
	exit 1
fi

sql_literal() {
	printf "'%s'" "${1//\'/\'\'}"
}

if [[ "$DB_RUNTIME" == "docker" ]]; then
	command -v docker >/dev/null 2>&1 || {
		echo "Error: Docker is required for the default PostgreSQL setup." >&2
		echo "Install/start Docker, or set DB_RUNTIME=local and DB_ADMIN_DSN." >&2
		exit 1
	}
	docker info >/dev/null 2>&1 || {
		echo "Error: the Docker daemon is unavailable." >&2
		exit 1
	}

	if docker container inspect "$DB_CONTAINER" >/dev/null 2>&1; then
		docker start "$DB_CONTAINER" >/dev/null
	else
		echo "Starting PostgreSQL container '$DB_CONTAINER'..."
		if ! docker run -d \
			--name "$DB_CONTAINER" \
			-p "$DB_PORT:5432" \
			-v "$DB_VOLUME:/var/lib/postgresql/data" \
			-e "POSTGRES_DB=postgres" \
			-e "POSTGRES_USER=$DB_ADMIN_USER" \
			-e "POSTGRES_PASSWORD=$DB_ADMIN_PASS" \
			"$DB_IMAGE" >/dev/null; then
			echo "Error: PostgreSQL container could not start. Check whether port $DB_PORT is already in use." >&2
			exit 1
		fi
	fi

	echo "Waiting for PostgreSQL..."
	ready=0
	for _ in {1..60}; do
		if docker exec -e "PGPASSWORD=$DB_ADMIN_PASS" "$DB_CONTAINER" \
			pg_isready -U "$DB_ADMIN_USER" -d postgres >/dev/null 2>&1; then
			ready=1
			break
		fi
		sleep 1
	done
	if [[ "$ready" != "1" ]]; then
		echo "Error: PostgreSQL did not become ready within 60 seconds." >&2
		docker logs "$DB_CONTAINER" >&2 || true
		exit 1
	fi

	PSQL_ADMIN=(docker exec -i -e "PGPASSWORD=$DB_ADMIN_PASS" "$DB_CONTAINER" psql -U "$DB_ADMIN_USER" -d postgres)
else
	command -v psql >/dev/null 2>&1 || {
		echo "Error: psql is required when DB_RUNTIME=local." >&2
		exit 1
	}
	PSQL_ADMIN=(psql "$DB_ADMIN_DSN")
fi

echo "Configuring PostgreSQL database '$DB_NAME' and user '$DB_USER'..."
if [[ $("${PSQL_ADMIN[@]}" -tAc "SELECT 1 FROM pg_roles WHERE rolname = $(sql_literal "$DB_USER")" | tr -d '[:space:]') != "1" ]]; then
	"${PSQL_ADMIN[@]}" -v ON_ERROR_STOP=1 -c "CREATE USER \"$DB_USER\" WITH ENCRYPTED PASSWORD $(sql_literal "$DB_PASS");"
else
	"${PSQL_ADMIN[@]}" -v ON_ERROR_STOP=1 -c "ALTER USER \"$DB_USER\" WITH ENCRYPTED PASSWORD $(sql_literal "$DB_PASS");"
fi

if [[ $("${PSQL_ADMIN[@]}" -tAc "SELECT 1 FROM pg_database WHERE datname = $(sql_literal "$DB_NAME")" | tr -d '[:space:]') != "1" ]]; then
	"${PSQL_ADMIN[@]}" -v ON_ERROR_STOP=1 -c "CREATE DATABASE \"$DB_NAME\" OWNER \"$DB_USER\";"
fi
"${PSQL_ADMIN[@]}" -v ON_ERROR_STOP=1 -c "GRANT CONNECT ON DATABASE \"$DB_NAME\" TO \"$DB_USER\";"
"${PSQL_ADMIN[@]}" -v ON_ERROR_STOP=1 -c "ALTER DATABASE \"$DB_NAME\" OWNER TO \"$DB_USER\";"

if [[ "$DB_RUNTIME" == "docker" ]]; then
	PSQL_APP=(docker exec -i -e "PGPASSWORD=$DB_PASS" "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME")
else
	PSQL_APP=(psql "$DB_DSN")
fi

"${PSQL_APP[@]}" -v ON_ERROR_STOP=1 <<SQL
REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE, CREATE ON SCHEMA public TO "$DB_USER";
CREATE TABLE IF NOT EXISTS public.rest_schema_migrations (
	version text PRIMARY KEY,
	applied_at timestamptz NOT NULL DEFAULT now()
);
SQL

if [[ -d "$MIGRATIONS_DIR" ]]; then
	shopt -s nullglob
	for migration in "$MIGRATIONS_DIR"/*.sql; do
		version=$(basename "$migration")
		applied=$("${PSQL_APP[@]}" -tAc "SELECT 1 FROM public.rest_schema_migrations WHERE version = $(sql_literal "$version")" | tr -d '[:space:]')
		if [[ "$applied" == "1" ]]; then
			continue
		fi
		echo "Applying migration '$version'..."
		{
			printf 'BEGIN;\n'
			awk '/^--[[:space:]]+\+goose[[:space:]]+Up/{up=1; next} /^--[[:space:]]+\+goose[[:space:]]+Down/{up=0} up' "$migration"
			printf '\nINSERT INTO public.rest_schema_migrations (version) VALUES (%s);\n' "$(sql_literal "$version")"
			printf 'COMMIT;\n'
		} | "${PSQL_APP[@]}" -v ON_ERROR_STOP=1
	done
fi

"${PSQL_APP[@]}" -tAc "SELECT 1" >/dev/null
echo "Done: PostgreSQL database '$DB_NAME' is ready for user '$DB_USER'."
echo "Application DSN: postgres://$DB_USER:***@$DB_HOST:$DB_PORT/$DB_NAME?{{ defaultString .Features.Build.DBOptions "sslmode=disable" }}"
{{ end }}
`

const ciWorkflowTemplate = `name: CI

on:
  pull_request:
  push:
    branches:
      - main
      - master

jobs:
  verify:
    runs-on: ubuntu-24.04
    strategy:
      fail-fast: false
      matrix:
        go-version:
          - "1.25.11"
          - "stable"
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{"{{"}} matrix.go-version {{"}}"}}
          cache: true

      - name: Format check
        run: test -z "$(gofmt -l .)"

      - name: Vet
        run: go vet ./...

      - name: Test
        run: go test -race ./...

      - name: Vulnerability scan
        run: go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...

      - name: Build
        run: go build ./cmd
`

const cdWorkflowTemplate = `name: CD

on:
  push:
    tags:
      - "v*"
  workflow_dispatch:

jobs:
  docker:
    runs-on: ubuntu-24.04
    permissions:
      contents: read
      packages: write
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to GitHub Container Registry
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{"{{"}} github.actor {{"}}"}}
          password: ${{"{{"}} secrets.GITHUB_TOKEN {{"}}"}}

      - name: Build and push
        uses: docker/build-push-action@v6
        with:
          context: .
          push: true
          tags: ghcr.io/${{"{{"}} github.repository {{"}}"}}:${{"{{"}} github.ref_name {{"}}"}}
`
