package main

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
		"    enabled: false                    # true/enable создаёт docker-compose.yml рядом с Dockerfile.": "    enabled: true                     # true/enable создаёт docker-compose.yml рядом с Dockerfile.",
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
