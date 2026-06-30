package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2EInitSQLCGenerateAndTestGeneratedProject(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "app")
	if err := run([]string{"init", "--sqlc", "--path", projectDir}); err != nil {
		t.Fatal(err)
	}
	patchE2ERestConfig(t, filepath.Join(projectDir, "rest_config", "rest.yaml"), projectDir)
	writeE2ESQLCInputs(t, projectDir)
	writeE2ESQLCOutput(t, projectDir)

	if err := run([]string{"gen", "--path", filepath.Join(projectDir, "rest_config")}); err != nil {
		t.Fatal(err)
	}
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

func patchE2ERestConfig(t *testing.T, path, projectDir string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	replacements := map[string]string{
		"module: github.com/repomz/myapp": "module: example.test/e2eapp",
		"project_path: .":                 "project_path: " + projectDir,
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
	cmd.Env = append(os.Environ(), "GOWORK=off")
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("generated project go test failed: %v\n%s", err, output.String())
	}
}
