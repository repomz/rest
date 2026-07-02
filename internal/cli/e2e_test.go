package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestE2EInitSQLCGenerateAndTestGeneratedProject(t *testing.T) {
	projectDir := generateE2ESQLProject(t)
	assertDoctorHealthy(t)
	for _, path := range []string{
		"cmd/main.go",
		"internal/app/domain/item.go",
		"internal/app/repository/pgrepo/item_repo.go",
		"internal/app/services/item.go",
		"internal/app/transport/httpserver/item_handlers.go",
		"docs/swagger.yaml",
	} {
		if _, err := os.Stat(filepath.Join(projectDir, path)); err != nil {
			t.Fatalf("expected generated file %s: %v", path, err)
		}
	}
	runGeneratedGoTest(t, projectDir)
}

func TestE2EInitMongoExampleGenerateAndTestGeneratedProject(t *testing.T) {
	projectDir := generateE2EMongoProject(t)
	assertDoctorHealthy(t)
	for _, path := range []string{
		"cmd/main.go",
		"internal/app/domain/document.go",
		"internal/app/repository/mongorepo/item_repo.go",
		"internal/app/services/item.go",
		"internal/app/transport/httpserver/item_handlers.go",
		"internal/app/transport/httpserver/auth_middleware.go",
		"docs/swagger.yaml",
		".env.example",
		"Dockerfile",
		".dockerignore",
	} {
		if _, err := os.Stat(filepath.Join(projectDir, path)); err != nil {
			t.Fatalf("expected generated Mongo example file %s: %v", path, err)
		}
	}
	swagger, err := os.ReadFile(filepath.Join(projectDir, "docs", "swagger.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"/items:", "/items/{id}:", "/items/by-status:", "ItemRequest:", "ItemResponse:"} {
		if !strings.Contains(string(swagger), expected) {
			t.Fatalf("Mongo example swagger missing %q:\n%s", expected, swagger)
		}
	}
	runGeneratedGoTest(t, projectDir)
}

func TestE2EGoldenSnapshots(t *testing.T) {
	sqlDir := generateE2ESQLProject(t)
	assertGoldenSnapshot(t, "sql.txt", sqlDir)
	mongoDir := generateE2EMongoProject(t)
	assertGoldenSnapshot(t, "mongo.txt", mongoDir)
}

func TestE2EGeneratesPrometheusMetricsForSQLAndMongo(t *testing.T) {
	t.Run("sql", func(t *testing.T) {
		projectDir := filepath.Join(t.TempDir(), "sql-metrics-app")
		if err := os.MkdirAll(projectDir, 0o755); err != nil {
			t.Fatal(err)
		}
		withWorkingDir(t, projectDir)
		if err := run([]string{"init"}); err != nil {
			t.Fatal(err)
		}
		patchE2ERestConfig(t, filepath.Join(projectDir, "rest_config", "rest.yaml"))
		enableMetricsForE2E(t, filepath.Join(projectDir, "rest_config", "rest.yaml"))
		writeE2ESQLCInputs(t, projectDir)
		writeE2ESQLCOutput(t, projectDir)
		if err := run([]string{"gen"}); err != nil {
			t.Fatal(err)
		}
		assertGeneratedMetrics(t, projectDir, []string{"SetDBStatsProvider", "open_connections"})
		runGeneratedGoTest(t, projectDir)
	})

	t.Run("mongo", func(t *testing.T) {
		projectDir := filepath.Join(t.TempDir(), "mongo-metrics-app")
		if err := os.MkdirAll(projectDir, 0o755); err != nil {
			t.Fatal(err)
		}
		withWorkingDir(t, projectDir)
		if err := run([]string{"init", "--example", "mongo"}); err != nil {
			t.Fatal(err)
		}
		enableMetricsForE2E(t, filepath.Join(projectDir, "rest_config", "rest.yaml"))
		if err := run([]string{"gen"}); err != nil {
			t.Fatal(err)
		}
		assertGeneratedMetrics(t, projectDir, []string{"promhttp.Handler().ServeHTTP", "requests_in_flight"})
		runGeneratedGoTest(t, projectDir)
	})
}

func TestE2EDockerBuildSmoke(t *testing.T) {
	if os.Getenv("REST_DOCKER_SMOKE") != "1" {
		t.Skip("set REST_DOCKER_SMOKE=1 to build generated Dockerfiles")
	}
	requireBinary(t, "docker")
	sqlDir := generateE2ESQLProject(t)
	runDockerBuildSmoke(t, sqlDir, "rest-generator-sql-smoke:test")
	mongoDir := generateE2EMongoProject(t)
	runDockerBuildSmoke(t, mongoDir, "rest-generator-mongo-smoke:test")
}

func generateE2ESQLProject(t *testing.T) string {
	t.Helper()
	projectDir := filepath.Join(t.TempDir(), "app")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	withWorkingDir(t, projectDir)
	if err := run([]string{"init"}); err != nil {
		t.Fatal(err)
	}
	patchE2ERestConfig(t, filepath.Join(projectDir, "rest_config", "rest.yaml"))
	writeE2ESQLCInputs(t, projectDir)
	writeE2ESQLCOutput(t, projectDir)

	if err := run([]string{"gen"}); err != nil {
		t.Fatal(err)
	}
	return projectDir
}

func generateE2EMongoProject(t *testing.T) string {
	t.Helper()
	projectDir := filepath.Join(t.TempDir(), "mongo-app")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	withWorkingDir(t, projectDir)
	if err := run([]string{"init", "--example", "mongo"}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"gen"}); err != nil {
		t.Fatal(err)
	}
	return projectDir
}

func TestE2EInitMongoExampleGeneratesComposeWhenEnabled(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "mongo-compose-app")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	withWorkingDir(t, projectDir)
	if err := run([]string{"init", "--example", "mongo"}); err != nil {
		t.Fatal(err)
	}
	patchFileForE2E(t, filepath.Join(projectDir, "rest_config", "rest.yaml"), map[string]string{
		"    enabled: false                    # true/enable creates docker-compose.yml for the selected backend.": "    enabled: true                     # true/enable creates docker-compose.yml for the selected backend.",
	})
	if err := run([]string{"gen"}); err != nil {
		t.Fatal(err)
	}
	compose, err := os.ReadFile(filepath.Join(projectDir, "docker-compose.yml"))
	if err != nil {
		t.Fatalf("expected docker-compose.yml: %v", err)
	}
	for _, expected := range []string{"mongo:7", "MONGO_URI: mongodb://mongo:27017", "mongo_data:"} {
		if !strings.Contains(string(compose), expected) {
			t.Fatalf("Mongo compose missing %q:\n%s", expected, compose)
		}
	}
}

func TestE2EGeneratesDeploymentGuideWhenEnabled(t *testing.T) {
	t.Run("sql", func(t *testing.T) {
		projectDir := filepath.Join(t.TempDir(), "sql-guide-app")
		if err := os.MkdirAll(projectDir, 0o755); err != nil {
			t.Fatal(err)
		}
		withWorkingDir(t, projectDir)
		if err := run([]string{"init"}); err != nil {
			t.Fatal(err)
		}
		patchE2ERestConfig(t, filepath.Join(projectDir, "rest_config", "rest.yaml"))
		patchFileForE2E(t, filepath.Join(projectDir, "rest_config", "rest.yaml"), map[string]string{
			"    enabled: false                    # Generate DEPLOYMENT.md tailored to this app.\n    output: DEPLOYMENT.md            # Practical local/prod runbook for the generated application.": "    enabled: true                    # Generate DEPLOYMENT.md tailored to this app.\n    output: DEPLOYMENT.md            # Practical local/prod runbook for the generated application.",
		})
		writeE2ESQLCInputs(t, projectDir)
		writeE2ESQLCOutput(t, projectDir)
		if err := run([]string{"gen"}); err != nil {
			t.Fatal(err)
		}
		assertDeploymentGuide(t, filepath.Join(projectDir, "DEPLOYMENT.md"), []string{"`DB_DSN`", "rest doctor", "go run ./cmd"})
	})

	t.Run("mongo", func(t *testing.T) {
		projectDir := filepath.Join(t.TempDir(), "mongo-guide-app")
		if err := os.MkdirAll(projectDir, 0o755); err != nil {
			t.Fatal(err)
		}
		withWorkingDir(t, projectDir)
		if err := run([]string{"init", "--example", "mongo"}); err != nil {
			t.Fatal(err)
		}
		patchFileForE2E(t, filepath.Join(projectDir, "rest_config", "rest.yaml"), map[string]string{
			"    enabled: false                    # Generate DEPLOYMENT.md tailored to this app.\n    output: DEPLOYMENT.md            # Practical local/prod runbook for the generated application.": "    enabled: true                    # Generate DEPLOYMENT.md tailored to this app.\n    output: DEPLOYMENT.md            # Practical local/prod runbook for the generated application.",
		})
		if err := run([]string{"gen"}); err != nil {
			t.Fatal(err)
		}
		assertDeploymentGuide(t, filepath.Join(projectDir, "DEPLOYMENT.md"), []string{"`MONGO_URI`", "MongoDB database", "rest doctor"})
	})
}

func TestE2EMongoGeneratesProjectSupportFilesWhenEnabled(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "mongo-support-app")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	withWorkingDir(t, projectDir)
	if err := run([]string{"init", "--example", "mongo"}); err != nil {
		t.Fatal(err)
	}
	patchFileForE2E(t, filepath.Join(projectDir, "rest_config", "rest.yaml"), map[string]string{
		"  makefile:\n    enabled: false":  "  makefile:\n    enabled: true",
		"  gitignore:\n    enabled: false": "  gitignore:\n    enabled: true",
		"    generate_local_env: false":    "    generate_local_env: true",
		"  ci:\n    enabled: false":        "  ci:\n    enabled: true",
		"  cd:\n    enabled: false":        "  cd:\n    enabled: true",
	})
	if err := run([]string{"gen"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"Makefile",
		".gitignore",
		".env.example",
		".env",
		filepath.Join(".github", "workflows", "ci.yaml"),
		filepath.Join(".github", "workflows", "cd.yaml"),
	} {
		if _, err := os.Stat(filepath.Join(projectDir, path)); err != nil {
			t.Fatalf("expected Mongo support file %s: %v", path, err)
		}
	}
	makefile, err := os.ReadFile(filepath.Join(projectDir, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(makefile), "MONGO_URI") || strings.Contains(string(makefile), "DB_DSN") {
		t.Fatalf("Mongo Makefile must use Mongo settings only:\n%s", makefile)
	}
}

func assertDeploymentGuide(t *testing.T, path string, expected []string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected deployment guide %s: %v", path, err)
	}
	text := string(content)
	for _, value := range expected {
		if !strings.Contains(text, value) {
			t.Fatalf("deployment guide missing %q:\n%s", value, text)
		}
	}
}

func patchE2ERestConfig(t *testing.T, path string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	replacements := map[string]string{
		"module: github.com/repomz/myapp": "module: example.test/e2eapp",
		"auto_sqlc: enable":               "auto_sqlc: disable",
	}
	text := string(content)
	for old, replacement := range replacements {
		if !strings.Contains(text, old) {
			t.Fatalf("%s does not contain %q", path, old)
		}
		text = strings.Replace(text, old, replacement, 1)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func patchFileForE2E(t *testing.T, path string, replacements map[string]string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for old, replacement := range replacements {
		if !strings.Contains(text, old) {
			t.Fatalf("%s does not contain %q", path, old)
		}
		text = strings.Replace(text, old, replacement, 1)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func enableMetricsForE2E(t *testing.T, path string) {
	t.Helper()
	patchFileForE2E(t, path, map[string]string{
		"    enabled: false                    # Generate a Prometheus metrics endpoint using the official Go client.": "    enabled: true                     # Generate a Prometheus metrics endpoint using the official Go client.",
	})
}

func assertGeneratedMetrics(t *testing.T, projectDir string, expectedSource []string) {
	t.Helper()
	metricsPath := filepath.Join(projectDir, "internal", "app", "metrics", "metrics.go")
	content, err := os.ReadFile(metricsPath)
	if err != nil {
		t.Fatalf("expected generated metrics source: %v", err)
	}
	text := string(content)
	for _, expected := range expectedSource {
		if !strings.Contains(text, expected) {
			t.Fatalf("generated metrics source missing %q:\n%s", expected, text)
		}
	}
	goMod, err := os.ReadFile(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(goMod), "github.com/prometheus/client_golang") {
		t.Fatalf("generated go.mod must include Prometheus client:\n%s", goMod)
	}
}

func writeE2ESQLCInputs(t *testing.T, projectDir string) {
	t.Helper()
	writeE2EFile(t, filepath.Join(projectDir, "rest_sqlc", "schema", "item.sql"), `
CREATE TABLE items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted BOOLEAN NOT NULL DEFAULT false
);
`)
	writeE2EFile(t, filepath.Join(projectDir, "rest_sqlc", "queries", "item.sql"), `
-- name: CreateItem :one
INSERT INTO items (name) VALUES ($1) RETURNING *;

-- name: GetItems :many
SELECT * FROM items WHERE deleted = false ORDER BY created_at DESC;

-- name: GetItemByID :one
SELECT * FROM items WHERE id = $1 AND deleted = false;

-- name: SoftDeleteItem :exec
UPDATE items SET deleted = true WHERE id = $1;
`)
}

func writeE2ESQLCOutput(t *testing.T, projectDir string) {
	t.Helper()
	dbDir := filepath.Join(projectDir, "internal", "app", "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeE2EFile(t, filepath.Join(dbDir, "db.go"), `package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type DBTX interface{}

type Queries struct {
	db DBTX
}

func New(db DBTX) *Queries {
	return &Queries{db: db}
}

type Item struct {
	ID        uuid.UUID
	Name      string
	CreatedAt time.Time
	Deleted   bool
}

func (q *Queries) CreateItem(ctx context.Context, arg CreateItemParams) (Item, error) {
	return Item{ID: uuid.New(), Name: arg.Name, CreatedAt: time.Now()}, nil
}

func (q *Queries) GetItems(ctx context.Context) ([]Item, error) {
	return []Item{}, nil
}

func (q *Queries) GetItemByID(ctx context.Context, id uuid.UUID) (Item, error) {
	return Item{ID: id, Name: "item", CreatedAt: time.Now()}, nil
}

func (q *Queries) SoftDeleteItem(ctx context.Context, id uuid.UUID) error {
	return nil
}

var _ = sql.ErrNoRows
`)
	writeE2EFile(t, filepath.Join(dbDir, "item.sql.go"), `package db

type CreateItemParams struct {
	Name string `+"`json:\"name\"`"+`
}
`)
}

func writeE2EFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimPrefix(content, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runGeneratedGoTest(t *testing.T, projectDir string) {
	t.Helper()
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOMODCACHE="+filepath.Join(os.TempDir(), "rest-go-mod"))
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("generated project go test failed: %v\n%s", err, output.String())
	}
}

func assertDoctorHealthy(t *testing.T) {
	t.Helper()
	report := runDoctorChecks("rest_config")
	if report.Errors() == 0 {
		return
	}
	var output bytes.Buffer
	report.Print(&output)
	t.Fatalf("expected generated project to pass rest doctor without errors:\n%s", output.String())
}

func assertGoldenSnapshot(t *testing.T, name, projectDir string) {
	t.Helper()
	got := generatedSnapshot(t, projectDir)
	path := goldenSnapshotPath(t, name)
	if os.Getenv("REST_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden snapshot %s: %v; run REST_UPDATE_GOLDEN=1 make golden to create it", path, err)
	}
	if string(want) != got {
		t.Fatalf("golden snapshot %s mismatch\n--- want\n%s\n--- got\n%s", name, string(want), got)
	}
}

func goldenSnapshotPath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve e2e test path")
	}
	return filepath.Join(filepath.Dir(file), "testdata", "golden", name)
}

func generatedSnapshot(t *testing.T, projectDir string) string {
	t.Helper()
	var lines []string
	for _, root := range []string{"cmd", "internal", "docs", "curl"} {
		walkSnapshotRoot(t, projectDir, root, &lines)
	}
	for _, path := range []string{"Dockerfile", ".dockerignore", ".env.example", "Makefile", "go.mod", "go.sum"} {
		appendSnapshotFile(t, projectDir, path, &lines)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n") + "\n"
}

func walkSnapshotRoot(t *testing.T, projectDir, root string, lines *[]string) {
	t.Helper()
	absRoot := filepath.Join(projectDir, root)
	if _, err := os.Stat(absRoot); os.IsNotExist(err) {
		return
	}
	if err := filepath.WalkDir(absRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(projectDir, path)
		if err != nil {
			return err
		}
		appendSnapshotFile(t, projectDir, filepath.ToSlash(rel), lines)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func appendSnapshotFile(t *testing.T, projectDir, path string, lines *[]string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(path)))
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	*lines = append(*lines, filepath.ToSlash(path)+" "+hex.EncodeToString(sum[:]))
}

func runDockerBuildSmoke(t *testing.T, projectDir, tag string) {
	t.Helper()
	cmd := exec.Command("docker", "build", "--pull=false", "-t", tag, ".")
	cmd.Dir = projectDir
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("docker build failed for %s: %v\n%s", tag, err, output.String())
	}
	t.Cleanup(func() {
		_ = exec.Command("docker", "rmi", "-f", tag).Run()
	})
}
