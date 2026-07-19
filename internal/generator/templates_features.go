package generator

const metricsTemplate = `package metrics

import (
{{- if ne .Features.Build.Backend "mongo" }}
	"database/sql"
{{- end }}
	"net/http"
{{- if and (or .Features.Metrics.HTTPRequests .Features.Metrics.RequestDuration .Features.Metrics.ResponseSize) (contains .Features.Metrics.Labels "status") }}
	"strconv"
{{- end }}
{{- if .Features.Metrics.RequestDuration }}
	"time"
{{- end }}

{{- if and (or .Features.Metrics.HTTPRequests .Features.Metrics.RequestDuration .Features.Metrics.ResponseSize) (contains .Features.Metrics.Labels "route") }}
	"github.com/gorilla/mux"
{{- end }}
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = {{ printf "%q" (defaultString .Features.Metrics.Namespace "app") }}

{{- if or .Features.Metrics.HTTPRequests .Features.Metrics.RequestDuration .Features.Metrics.ResponseSize }}
var labelNames = []string{
{{- if contains .Features.Metrics.Labels "method" }}
	"method",
{{- end }}
{{- if contains .Features.Metrics.Labels "route" }}
	"route",
{{- end }}
{{- if contains .Features.Metrics.Labels "status" }}
	"status",
{{- end }}
}
{{- end }}

var (
{{- if .Features.Metrics.HTTPRequests }}
	requestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "http",
		Name: "requests_total",
		Help: "Total number of HTTP requests processed by the application.",
	}, labelNames)
{{- end }}
{{- if .Features.Metrics.RequestDuration }}
	requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: "http",
		Name: "request_duration_seconds",
		Help: "HTTP request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, labelNames)
{{- end }}
{{- if .Features.Metrics.ResponseSize }}
	responseSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: "http",
		Name: "response_size_bytes",
		Help: "HTTP response size in bytes.",
		Buckets: []float64{100, 500, 1000, 2500, 5000, 10000, 50000, 100000, 500000, 1000000},
	}, labelNames)
{{- end }}
{{- if .Features.Metrics.InFlightRequests }}
	inFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "http",
		Name: "requests_in_flight",
		Help: "Current number of in-flight HTTP requests.",
	})
{{- end }}
{{- if ne .Features.Build.Backend "mongo" }}
	dbStatsProvider func() sql.DBStats
	dbOpenConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "db",
		Name: "open_connections",
		Help: "Number of established database connections.",
	})
	dbConnectionsInUse = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "db",
		Name: "connections_in_use",
		Help: "Number of database connections currently in use.",
	})
	dbIdleConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "db",
		Name: "idle_connections",
		Help: "Number of idle database connections.",
	})
{{- end }}
)

func init() {
{{- if .Features.Metrics.HTTPRequests }}
	prometheus.MustRegister(requestsTotal)
{{- end }}
{{- if .Features.Metrics.RequestDuration }}
	prometheus.MustRegister(requestDuration)
{{- end }}
{{- if .Features.Metrics.ResponseSize }}
	prometheus.MustRegister(responseSize)
{{- end }}
{{- if .Features.Metrics.InFlightRequests }}
	prometheus.MustRegister(inFlight)
{{- end }}
{{- if ne .Features.Build.Backend "mongo" }}
	prometheus.MustRegister(dbOpenConnections, dbConnectionsInUse, dbIdleConnections)
{{- end }}
}

{{- if ne .Features.Build.Backend "mongo" }}
func SetDBStatsProvider(provider func() sql.DBStats) {
	dbStatsProvider = provider
}
{{- end }}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		{{- if .Features.Metrics.InFlightRequests }}
		inFlight.Inc()
		defer inFlight.Dec()
		{{- end }}
		{{- if .Features.Metrics.RequestDuration }}
		started := time.Now()
		{{- end }}
		recorder := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		{{- if or .Features.Metrics.HTTPRequests .Features.Metrics.RequestDuration .Features.Metrics.ResponseSize }}
		labels := labelValues(r, recorder.status)
		{{- end }}
		{{- if .Features.Metrics.HTTPRequests }}
		requestsTotal.WithLabelValues(labels...).Inc()
		{{- end }}
		{{- if .Features.Metrics.RequestDuration }}
		requestDuration.WithLabelValues(labels...).Observe(time.Since(started).Seconds())
		{{- end }}
		{{- if .Features.Metrics.ResponseSize }}
		responseSize.WithLabelValues(labels...).Observe(float64(recorder.bytes))
		{{- end }}
	})
}

func Handler(w http.ResponseWriter, r *http.Request) {
	{{- if ne .Features.Build.Backend "mongo" }}
	updateDBStats()
	{{- end }}
	promhttp.Handler().ServeHTTP(w, r)
}

{{- if ne .Features.Build.Backend "mongo" }}
func updateDBStats() {
	if dbStatsProvider == nil {
		return
	}
	stats := dbStatsProvider()
	dbOpenConnections.Set(float64(stats.OpenConnections))
	dbConnectionsInUse.Set(float64(stats.InUse))
	dbIdleConnections.Set(float64(stats.Idle))
}
{{- end }}

{{- if or .Features.Metrics.HTTPRequests .Features.Metrics.RequestDuration .Features.Metrics.ResponseSize }}
func labelValues(r *http.Request, status int) []string {
	values := make([]string, 0, len(labelNames))
{{- if contains .Features.Metrics.Labels "method" }}
	values = append(values, r.Method)
{{- end }}
{{- if contains .Features.Metrics.Labels "route" }}
	values = append(values, routePattern(r))
{{- end }}
{{- if contains .Features.Metrics.Labels "status" }}
	values = append(values, strconv.Itoa(status))
{{- end }}
	return values
}

{{- if contains .Features.Metrics.Labels "route" }}
func routePattern(r *http.Request) string {
	if current := mux.CurrentRoute(r); current != nil {
		if template, err := current.GetPathTemplate(); err == nil {
			return template
		}
	}
	return r.URL.Path
}
{{- end }}
{{- end }}

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes int
}

func (w *responseRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseRecorder) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	w.bytes += n
	return n, err
}
`

const dockerfileTemplate = `FROM {{ .Features.Docker.BuildImage }} AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED={{ if .Features.Docker.CGOEnabled }}1{{ else }}0{{ end }} go build -trimpath -ldflags="-s -w" -o /out/{{ .Features.Docker.Binary }} ./cmd

FROM {{ .Features.Docker.RuntimeImage }}
RUN addgroup -S {{ .Features.Docker.User }} && adduser -S {{ .Features.Docker.User }} -G {{ .Features.Docker.User }}
WORKDIR /app
COPY --from=build /out/{{ .Features.Docker.Binary }} /app/{{ .Features.Docker.Binary }}
USER {{ .Features.Docker.User }}
EXPOSE {{ .Features.Docker.Port }}
{{- if .Features.Docker.Healthcheck }}
HEALTHCHECK --interval={{ .Features.Docker.HealthInterval }} --timeout={{ .Features.Docker.HealthTimeout }} --retries={{ .Features.Docker.HealthRetries }} CMD wget -qO- http://127.0.0.1:{{ .Features.Docker.Port }}{{ .Features.Docker.HealthPath }} || exit 1
{{- end }}
ENTRYPOINT ["/app/{{ .Features.Docker.Binary }}"]
`

const dockerignoreTemplate = `.git
.gitignore
.env
bin
logs
curl
docs
*.md
`

const dockerComposeTemplate = `services:
  app:
    build:
      context: .
      dockerfile: {{ defaultString .Features.Docker.Output "Dockerfile" }}
    environment:
      HTTP_ADDR: 0.0.0.0:{{ .Features.Docker.Port }}
      DB_DSN: {{ printf "%q" (postgresDSN .Features.Build "postgres:5432") }}
{{- if and .Features.Auth.Enabled (eq .Features.Auth.Strategy "jwt") }}
      {{ .Features.Auth.JWTSecretEnv }}: change-me
{{- end }}
{{- if and .Features.Auth.Enabled (eq .Features.Auth.Strategy "basic") }}
      {{ .Features.Auth.BasicUsernameEnv }}: admin
      {{ .Features.Auth.BasicPasswordEnv }}: change-me
{{- end }}
    ports:
      - "{{ .Features.Docker.Port }}:{{ .Features.Docker.Port }}"
    depends_on:
      migrate:
        condition: service_completed_successfully

  migrate:
    image: postgres:17-alpine
    environment:
      PGHOST: postgres
      PGPORT: "5432"
      PGDATABASE: {{ printf "%q" (defaultString .Features.Build.DBName "app_db") }}
      PGUSER: {{ printf "%q" (defaultString .Features.Build.DBUser "app_user") }}
      PGPASSWORD: {{ printf "%q" (defaultString .Features.Build.DBPassword "app_password") }}
      MIGRATIONS_DIR: /migrations
    volumes:
      - ./docker/migrate.sh:/usr/local/bin/rest-migrate.sh:ro
      - ./{{ defaultString .Features.Build.MigrationsPath "internal/sql/migrations" }}:/migrations:ro
    depends_on:
      postgres:
        condition: service_healthy
    entrypoint: ["/bin/sh", "/usr/local/bin/rest-migrate.sh"]
    restart: "no"

  postgres:
    image: postgres:17-alpine
    environment:
      POSTGRES_DB: {{ printf "%q" (defaultString .Features.Build.DBName "app_db") }}
      POSTGRES_USER: {{ printf "%q" (defaultString .Features.Build.DBUser "app_user") }}
      POSTGRES_PASSWORD: {{ printf "%q" (defaultString .Features.Build.DBPassword "app_password") }}
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U {{ defaultString .Features.Build.DBUser "app_user" }} -d {{ defaultString .Features.Build.DBName "app_db" }}"]
      interval: 5s
      timeout: 3s
      retries: 10

volumes:
  postgres_data:
`

const dockerMigrateTemplate = `#!/bin/sh
set -eu

: "${PGHOST:=postgres}"
: "${PGPORT:=5432}"
: "${PGDATABASE:=app_db}"
: "${PGUSER:=app_user}"
: "${MIGRATIONS_DIR:=/migrations}"

psql_app() {
	psql \
		--host "$PGHOST" \
		--port "$PGPORT" \
		--username "$PGUSER" \
		--dbname "$PGDATABASE" \
		-v ON_ERROR_STOP=1 \
		"$@"
}

sql_literal() {
	escaped=$(printf '%s' "$1" | sed "s/'/''/g")
	printf "'%s'" "$escaped"
}

echo "Waiting for PostgreSQL at $PGHOST:$PGPORT..."
ready=0
attempt=1
while [ "$attempt" -le 60 ]; do
	if pg_isready --host "$PGHOST" --port "$PGPORT" --username "$PGUSER" --dbname "$PGDATABASE" >/dev/null 2>&1; then
		ready=1
		break
	fi
	attempt=$((attempt + 1))
	sleep 1
done
if [ "$ready" != "1" ]; then
	echo "Error: PostgreSQL did not become ready within 60 seconds." >&2
	exit 1
fi

if [ ! -d "$MIGRATIONS_DIR" ]; then
	echo "Error: migrations directory '$MIGRATIONS_DIR' does not exist." >&2
	exit 1
fi

psql_app <<'SQL'
CREATE TABLE IF NOT EXISTS public.rest_schema_migrations (
	version text PRIMARY KEY,
	applied_at timestamptz NOT NULL DEFAULT now()
);
SQL

found=0
for migration in "$MIGRATIONS_DIR"/*.sql; do
	if [ ! -f "$migration" ]; then
		continue
	fi
	found=1
	version=$(basename "$migration")
	version_literal=$(sql_literal "$version")
	applied=$(psql_app -tAc "SELECT 1 FROM public.rest_schema_migrations WHERE version = $version_literal" | tr -d '[:space:]')
	if [ "$applied" = "1" ]; then
		echo "Already applied: $version"
		continue
	fi

	echo "Applying migration: $version"
	{
		printf 'BEGIN;\n'
		awk '
			/^--[[:space:]]+\+goose[[:space:]]+Up/ { up=1; next }
			/^--[[:space:]]+\+goose[[:space:]]+Down/ { up=0 }
			up { print }
		' "$migration"
		printf '\nINSERT INTO public.rest_schema_migrations (version) VALUES (%s);\n' "$version_literal"
		printf 'COMMIT;\n'
	} | psql_app
done

if [ "$found" != "1" ]; then
	echo "Error: no SQL migrations found in '$MIGRATIONS_DIR'." >&2
	echo "Enable rest_sqlc.yaml init_migration or add a migration before starting Compose." >&2
	exit 1
fi

psql_app -tAc "SELECT 1" >/dev/null
echo "PostgreSQL migrations are up to date."
`

const gitignoreTemplate = `# rest:begin
# Go build output and local binaries
bin/
dist/
build/
out/
.cache/
/app
/rest
*.exe
*.exe~
*.dll
*.so
*.dylib
*.a
*.test
*.out

# Test, coverage, benchmark, and profiling output
coverage/
coverage.*
coverage.out
coverage.html
*.cover
*.coverprofile
*.prof
*.pprof
*.trace

# Local Go workspace
go.work
go.work.sum

# Local runtime configuration and secrets
.env
.env.*
!.env.example
*.local

# Logs, process files, and temporary data
logs/
*.log
*.pid
*.pid.lock
tmp/
temp/

# Docker and local orchestration overrides
docker-compose.override.yml
docker-compose.*.override.yml
compose.override.yml
compose.*.override.yml

# rest generator workspace files not required by the generated runtime app
/.rest/
/.rest-generator/
/rest_config/
/rest_sqlc/
/rest_sqlc_example/
/rest_mongo/
/auth_rest.yaml
/mongo_rest.yaml
/rest.yaml
/rest_sqlc.yaml
/{{ defaultString .Features.Build.DeploymentPath "DEPLOYMENT.md" }}

# IDE and editor state
.idea/
.vscode/
.fleet/
.zed/
*.swp
*.swo
*~

# OS metadata
.DS_Store
._*
Thumbs.db
Desktop.ini
# rest:end
`

const envExampleTemplate = `# Code generated by rest.
HTTP_ADDR={{ httpAddr .Features.HTTP.Host .Features.HTTP.Port }}
DB_NAME={{ defaultString .Features.Build.DBName "app_db" }}
DB_USER={{ defaultString .Features.Build.DBUser "app_user" }}
DB_PASS={{ defaultString .Features.Build.DBPassword "app_password" }}
DB_DRIVER=postgres
DB_OPTIONS={{ defaultString .Features.Build.DBOptions "sslmode=disable" }}
DB_DSN=postgres://{{ defaultString .Features.Build.DBUser "app_user" }}:{{ defaultString .Features.Build.DBPassword "app_password" }}@localhost:5432/{{ defaultString .Features.Build.DBName "app_db" }}?{{ defaultString .Features.Build.DBOptions "sslmode=disable" }}
MIGRATIONS_DIR=./{{ defaultString .Features.Build.MigrationsPath "internal/sql/migrations" }}
DEBUG_ERRORS=0
`
