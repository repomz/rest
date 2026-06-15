package generator

const appMainTemplate = `package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"

	"{{ .Module }}/internal/app/config"
	"{{ .DBImport }}"
	{{- if .Features.Logging.Enabled }}
	"{{ .Module }}/internal/app/logging"
	{{- end }}
	"{{ .Module }}/internal/app/repository/pgrepo"
	"{{ .Module }}/internal/app/services"
	"{{ .Module }}/internal/app/transport/httpserver"
	{{- if .Features.HTTP.Health }}
	"{{ .Module }}/internal/app/common/server"
	{{- end }}
	{{- if or .Features.HTTP.CORS .Features.HTTP.Recovery .Features.HTTP.RequestID (gt .Features.HTTP.MaxBodyBytes 0) }}
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
	dbase.SetMaxIdleConns(10)
	dbase.SetMaxOpenConns(10)
	dbase.SetConnMaxLifetime(time.Minute)
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
	httpServer := httpserver.NewHttpServer(
{{- range .Tables }}
		{{ .Singular }}Service,
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
	apiRouter.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Generated API"))
	}).Methods(http.MethodGet)
	{{- if .Features.HTTP.Health }}
	apiRouter.HandleFunc({{ printf "%q" (defaultString .Features.HTTP.HealthPath "/health") }}, func(w http.ResponseWriter, _ *http.Request) {
		server.RespondOK(map[string]string{"status": "ok"}, w, nil)
	}).Methods(http.MethodGet)
	{{- end }}
	{{- if .Features.Metrics.Enabled }}
	metrics.SetDBStatsProvider(dbase.Stats)
	apiRouter.HandleFunc({{ printf "%q" (defaultString .Features.Metrics.Path "/metrics") }}, metrics.Handler).Methods(http.MethodGet)
	{{- end }}
	{{- if .Features.OpenAPI.Enabled }}
	apiRouter.HandleFunc({{ printf "%q" (defaultString .Features.OpenAPI.SpecPath "/swagger/openapi.yaml") }}, httpserver.SwaggerSpec).Methods(http.MethodGet)
	{{- end }}
	{{- if and .Features.OpenAPI.Enabled .Features.OpenAPI.WithUI }}
	apiRouter.HandleFunc({{ printf "%q" (defaultString .Features.OpenAPI.UIPath "/swagger/index.html") }}, httpserver.SwaggerUI).Methods(http.MethodGet)
	{{- end }}
{{- range .Tables }}
{{- if .Queries.GetAll }}
	apiRouter.HandleFunc("{{ .RouteBase }}", httpServer.GetAll{{ .GoPlural }}).Methods(http.MethodGet)
{{- end }}
{{- if .Queries.Create }}
	apiRouter.HandleFunc("{{ .RouteBase }}", httpServer.Create{{ .GoName }}).Methods(http.MethodPost)
{{- end }}
{{- if .Queries.DeleteAll }}
	apiRouter.HandleFunc("{{ .RouteBase }}", httpServer.DeleteAll{{ .GoPlural }}).Methods(http.MethodDelete)
{{- end }}
{{- range .Endpoints }}
	apiRouter.HandleFunc("{{ .Path }}", httpServer.{{ .Name }}).Methods("{{ .Method }}")
{{- end }}
{{- if .Queries.GetByID }}
	apiRouter.HandleFunc("{{ .RouteBase }}/{id}", httpServer.Get{{ .GoName }}ByID).Methods(http.MethodGet)
{{- end }}
{{- if .Queries.Delete }}
	apiRouter.HandleFunc("{{ .RouteBase }}/{id}", httpServer.Delete{{ .GoName }}).Methods(http.MethodDelete)
{{- end }}
{{ end }}

	var handler http.Handler = router
	{{- if gt .Features.HTTP.MaxBodyBytes 0 }}
	handler = middleware.MaxBodyBytes({{ .Features.HTTP.MaxBodyBytes }}, handler)
	{{- end }}
	{{- if .Features.HTTP.CORS }}
	handler = middleware.CORS(handler)
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
	stopped := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		<-sigint
		ctx, cancel := context.WithTimeout(context.Background(), mustDuration({{ printf "%q" (defaultString .Features.HTTP.ShutdownTimeout "10s") }}))
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
		close(stopped)
	}()

	log.Printf("Starting HTTP server on %s", cfg.HTTPAddr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	<-stopped
	return nil
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
GOLANGCI_LINT_VERSION ?= latest
REST ?= rest
REST_CONFIG ?= {{ defaultString .Features.Build.ConfigPath "rest_config" }}

export

.PHONY: build rest-generate run test clean{{ if .Features.Build.InitDB }} db{{ end }}{{ if .Features.Build.InitMigration }} migrate-status migrate-up migrate-down migrate-create{{ end }} install-lint lint

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd

rest-generate:
	$(REST) app generate -config $(REST_CONFIG)

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
	@test -f $(DB_SCRIPT) || { echo "Ошибка: $(DB_SCRIPT) отсутствует"; exit 1; }
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
	@read -p "Название миграции: " name; \
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

: "${DB_NAME:={{ defaultString .Features.Build.DBName "app_db" }}}"
: "${DB_USER:={{ defaultString .Features.Build.DBUser "app_user" }}}"
: "${DB_PASS:={{ defaultString .Features.Build.DBPassword "app_password" }}}"
: "${DB_ADMIN_DB:=postgres}"
: "${USE_SUDO_POSTGRES:=0}"

if [[ "$USE_SUDO_POSTGRES" == "1" ]]; then
	PSQL_ADMIN=(sudo -u postgres psql -d "$DB_ADMIN_DB")
else
	PSQL_ADMIN=(psql -d "$DB_ADMIN_DB")
fi

sql_literal() {
	printf "'%s'" "${1//\'/\'\'}"
}

echo "Настройка базы данных '$DB_NAME' и пользователя '$DB_USER'..."

if [[ $("${PSQL_ADMIN[@]}" -tAc "SELECT 1 FROM pg_roles WHERE rolname = $(sql_literal "$DB_USER")" | tr -d '[:space:]') != "1" ]]; then
	"${PSQL_ADMIN[@]}" -v ON_ERROR_STOP=1 -c "CREATE USER \"$DB_USER\" WITH ENCRYPTED PASSWORD $(sql_literal "$DB_PASS");"
	echo "Пользователь '$DB_USER' создан."
else
	"${PSQL_ADMIN[@]}" -v ON_ERROR_STOP=1 -c "ALTER USER \"$DB_USER\" WITH ENCRYPTED PASSWORD $(sql_literal "$DB_PASS");"
	echo "Пользователь '$DB_USER' уже существует, пароль обновлен."
fi

if [[ $("${PSQL_ADMIN[@]}" -tAc "SELECT 1 FROM pg_database WHERE datname = $(sql_literal "$DB_NAME")" | tr -d '[:space:]') != "1" ]]; then
	"${PSQL_ADMIN[@]}" -v ON_ERROR_STOP=1 -c "CREATE DATABASE \"$DB_NAME\" OWNER \"$DB_USER\";"
	echo "База данных '$DB_NAME' создана."
else
	echo "База данных '$DB_NAME' уже существует."
fi

"${PSQL_ADMIN[@]}" -v ON_ERROR_STOP=1 <<SQL
GRANT CONNECT ON DATABASE "$DB_NAME" TO "$DB_USER";
ALTER DATABASE "$DB_NAME" OWNER TO "$DB_USER";
SQL

if [[ "$USE_SUDO_POSTGRES" == "1" ]]; then
	PSQL_TARGET=(sudo -u postgres psql -d "$DB_NAME")
else
	PSQL_TARGET=(psql -d "$DB_NAME")
fi

"${PSQL_TARGET[@]}" -v ON_ERROR_STOP=1 <<SQL
REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE, CREATE ON SCHEMA public TO "$DB_USER";
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO "$DB_USER";
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO "$DB_USER";
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO "$DB_USER";
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO "$DB_USER";
SQL

echo "Готово: база '$DB_NAME' доступна пользователю '$DB_USER'."
`
