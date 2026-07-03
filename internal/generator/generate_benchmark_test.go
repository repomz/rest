package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func BenchmarkGenerate(b *testing.B) {
	for _, tableCount := range []int{10, 50} {
		b.Run(fmt.Sprintf("tables_%d", tableCount), func(b *testing.B) {
			projectDir, sqlcPath := createGenerateBenchmarkProject(b, tableCount)
			opts := benchmarkGenerateOptions(projectDir, sqlcPath)

			if err := Generate(opts); err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := Generate(opts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func createGenerateBenchmarkProject(b *testing.B, tableCount int) (string, string) {
	b.Helper()

	root := b.TempDir()
	sqlcDir := filepath.Join(root, "rest_sqlc")
	schemaDir := filepath.Join(sqlcDir, "schema")
	queriesDir := filepath.Join(sqlcDir, "queries")
	dbDir := filepath.Join(root, "internal", "app", "db")
	for _, dir := range []string{schemaDir, queriesDir, dbDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatal(err)
		}
	}

	writeBenchmarkFile(b, filepath.Join(root, "go.mod"), "module example.test/benchmark\n\ngo 1.25.11\n")
	sqlcConfig := `version: "2"
sql:
  - engine: postgresql
    queries: queries
    schema: schema
    gen:
      go:
        package: db
        out: ../internal/app/db
`
	sqlcPath := filepath.Join(sqlcDir, "rest_sqlc.yaml")
	writeBenchmarkFile(b, sqlcPath, sqlcConfig)

	var schema strings.Builder
	var queries strings.Builder
	var querier strings.Builder
	var params strings.Builder
	querier.WriteString("package db\n\nimport (\n\t\"context\"\n\t\"database/sql\"\n\t\"time\"\n\n\t\"github.com/google/uuid\"\n)\n\ntype Querier interface {\n")
	params.WriteString("package db\n\nimport (\n\t\"database/sql\"\n\t\"time\"\n\n\t\"github.com/google/uuid\"\n)\n\n")

	for i := 1; i <= tableCount; i++ {
		tableName := fmt.Sprintf("studies_%03d", i)
		goName := fmt.Sprintf("Study%03d", i)
		pluralName := fmt.Sprintf("Studies%03d", i)

		fmt.Fprintf(&schema, benchmarkStudySchema, tableName)
		fmt.Fprintf(&queries, benchmarkStudyQueries, goName, tableName, pluralName, tableName, goName, tableName, goName, tableName, pluralName, tableName, goName, tableName)
		fmt.Fprintf(&querier, benchmarkQuerierMethods,
			goName, goName, goName,
			pluralName, pluralName, goName,
			goName, goName,
			goName,
			pluralName,
			goName, goName, goName,
		)
		fmt.Fprintf(&params, benchmarkSQLCTypes, goName, goName, pluralName, goName)
	}
	querier.WriteString("}\n")

	writeBenchmarkFile(b, filepath.Join(schemaDir, "studies.sql"), schema.String())
	writeBenchmarkFile(b, filepath.Join(queriesDir, "studies.sql"), queries.String())
	writeBenchmarkFile(b, filepath.Join(dbDir, "querier.go"), querier.String())
	writeBenchmarkFile(b, filepath.Join(dbDir, "studies.sql.go"), params.String())

	return root, sqlcPath
}

func benchmarkGenerateOptions(projectDir, sqlcPath string) Options {
	return Options{
		SQLCPath: sqlcPath,
		OutDir:   projectDir,
		Features: FeatureOptions{
			Build: BuildFeatures{
				Configured:     true,
				Makefile:       true,
				HandlerTests:   true,
				Curl:           true,
				HTTPPort:       8080,
				Gitignore:      true,
				Env:            true,
				InitDB:         true,
				InitMigration:  true,
				MigrationsPath: "internal/sql/migrations",
				DBName:         "benchmark",
				DBUser:         "benchmark",
				DBPassword:     "benchmark",
				DBOptions:      "sslmode=disable",
			},
			HTTP: HTTPFeatures{
				CORS:             true,
				Recovery:         true,
				RequestID:        true,
				RequestIDHeader:  "X-Request-ID",
				Host:             "0.0.0.0",
				Port:             8080,
				ReadTimeout:      "30s",
				WriteTimeout:     "30s",
				IdleTimeout:      "60s",
				ShutdownTimeout:  "10s",
				GracefulShutdown: true,
				Health:           true,
				HealthPath:       "/health",
			},
			Logging: LoggingFeatures{
				Enabled:    true,
				Library:    "zap",
				Level:      "info",
				Format:     "json",
				OutputType: "stdout",
			},
			OpenAPI: OpenAPIFeatures{
				Enabled:   true,
				Output:    "docs/swagger.yaml",
				WithUI:    true,
				Title:     "Benchmark API",
				Version:   "1.0.0",
				ServerURL: "http://localhost:8080",
				UIPath:    "/swagger/index.html",
				SpecPath:  "/swagger/openapi.yaml",
			},
			Metrics: MetricsFeatures{
				Enabled:          true,
				Provider:         "prometheus",
				Path:             "/metrics",
				Namespace:        "benchmark",
				HTTPRequests:     true,
				RequestDuration:  true,
				ResponseSize:     true,
				InFlightRequests: true,
				Labels:           []string{"method", "route", "status"},
			},
			Docker: DockerFeatures{
				Enabled:            true,
				Output:             "Dockerfile",
				DockerignoreOutput: ".dockerignore",
				BuildImage:         "golang:1.25-alpine",
				RuntimeImage:       "alpine:3.21",
				Binary:             "app",
				Port:               8080,
				User:               "app",
				Healthcheck:        true,
				HealthPath:         "/health",
				HealthInterval:     "30s",
				HealthTimeout:      "3s",
				HealthRetries:      3,
			},
		},
	}
}

func writeBenchmarkFile(b *testing.B, path, content string) {
	b.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		b.Fatal(err)
	}
}

const benchmarkStudySchema = `
CREATE TABLE %s (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    study_id        TEXT NOT NULL,
    patient         TEXT NOT NULL,
    age             INTEGER,
    department      TEXT NOT NULL,
    name_operation  TEXT NOT NULL,
    study_type      TEXT NOT NULL,
    descr_operation TEXT NOT NULL,
    time_beginning  TIMESTAMP,
    time_duration   INTEGER,
    surgeon         TEXT NOT NULL,
    dicom_link      TEXT,
    deleted         BOOLEAN NOT NULL DEFAULT FALSE
);
`

const benchmarkStudyQueries = `
-- name: Create%s :one
INSERT INTO %s (study_id, patient, age, department, name_operation, study_type, descr_operation, time_beginning, time_duration, surgeon, dicom_link)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: Get%s :many
SELECT * FROM %s
WHERE deleted = false
  AND (sqlc.narg('date')::timestamp IS NULL OR time_beginning::date = sqlc.narg('date')::date)
  AND (sqlc.narg('type')::text IS NULL OR study_type = sqlc.narg('type'))
  AND (sqlc.narg('surgeon')::text IS NULL OR surgeon = sqlc.narg('surgeon'));

-- name: Get%sByID :one
SELECT * FROM %s WHERE id = $1 AND deleted = false;

-- name: SoftDelete%s :exec
UPDATE %s SET deleted = true, updated_at = NOW() WHERE id = $1;

-- name: SoftDeleteAll%s :exec
UPDATE %s SET deleted = true, updated_at = NOW() WHERE deleted = false;

-- name: Update%sDicomLink :one
UPDATE %s SET dicom_link = $2, updated_at = NOW() WHERE id = $1 AND deleted = false RETURNING *;
`

const benchmarkQuerierMethods = `
	Create%s(ctx context.Context, arg Create%sParams) (%s, error)
	Get%s(ctx context.Context, arg Get%sParams) ([]%s, error)
	Get%sByID(ctx context.Context, id uuid.UUID) (%s, error)
	SoftDelete%s(ctx context.Context, id uuid.UUID) error
	SoftDeleteAll%s(ctx context.Context) error
	Update%sDicomLink(ctx context.Context, arg Update%sDicomLinkParams) (%s, error)
`

const benchmarkSQLCTypes = `
type %s struct {
	ID uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
	StudyID string
	Patient string
	Age sql.NullInt32
	Department string
	NameOperation string
	StudyType string
	DescrOperation string
	TimeBeginning sql.NullTime
	TimeDuration sql.NullInt32
	Surgeon string
	DicomLink sql.NullString
	Deleted bool
}

type Create%sParams struct {
	StudyID string ` + "`json:\"study_id\"`" + `
	Patient string ` + "`json:\"patient\"`" + `
	Age sql.NullInt32 ` + "`json:\"age\"`" + `
	Department string ` + "`json:\"department\"`" + `
	NameOperation string ` + "`json:\"name_operation\"`" + `
	StudyType string ` + "`json:\"study_type\"`" + `
	DescrOperation string ` + "`json:\"descr_operation\"`" + `
	TimeBeginning sql.NullTime ` + "`json:\"time_beginning\"`" + `
	TimeDuration sql.NullInt32 ` + "`json:\"time_duration\"`" + `
	Surgeon string ` + "`json:\"surgeon\"`" + `
	DicomLink sql.NullString ` + "`json:\"dicom_link\"`" + `
}

type Get%sParams struct {
	Date sql.NullTime ` + "`json:\"date\"`" + `
	Type sql.NullString ` + "`json:\"type\"`" + `
	Surgeon sql.NullString ` + "`json:\"surgeon\"`" + `
}

type Update%sDicomLinkParams struct {
	ID uuid.UUID ` + "`json:\"id\"`" + `
	DicomLink sql.NullString ` + "`json:\"dicom_link\"`" + `
}
`
