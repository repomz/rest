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
COPY go.mod ./
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
      DB_DSN: "postgres://{{ defaultString .Features.Build.DBUser "app_user" }}:{{ defaultString .Features.Build.DBPassword "app_password" }}@postgres:5432/{{ defaultString .Features.Build.DBName "app_db" }}?{{ defaultString .Features.Build.DBOptions "sslmode=disable" }}"
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
      postgres:
        condition: service_healthy

  postgres:
    image: postgres:17-alpine
    environment:
      POSTGRES_DB: {{ defaultString .Features.Build.DBName "app_db" }}
      POSTGRES_USER: {{ defaultString .Features.Build.DBUser "app_user" }}
      POSTGRES_PASSWORD: {{ defaultString .Features.Build.DBPassword "app_password" }}
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

const gitignoreTemplate = `# rest:begin
# Build and test output
bin/
dist/
.cache/
coverage/
coverage.out
coverage.html
*.test
*.out

# Local runtime files and secrets
.env
.env.*
!.env.example
*.local

# Logs and temporary files
logs/
*.log
tmp/

# Docker/local overrides
docker-compose.override.yml

# rest generator workspace files not required by the generated runtime app
.rest/
rest_config/
rest_sqlc/
rest_sqlc_example/
{{ defaultString .Features.Build.DeploymentPath "DEPLOYMENT.md" }}

# OS/editor noise
.DS_Store
.idea/
.vscode/
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
