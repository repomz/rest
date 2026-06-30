package sqlcconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateProject(t *testing.T) {
	dir := t.TempDir()
	if err := GenerateProject(dir); err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{
		"rest_sqlc/rest_sqlc.yaml":   `- "queries"`,
		"rest_sqlc/schema/item.sql":  "CREATE TABLE items",
		"rest_sqlc/queries/item.sql": "-- name: CreateItem :one",
	}
	if _, err := os.Stat(filepath.Join(dir, "rest_sqlc.yaml")); !os.IsNotExist(err) {
		t.Fatalf("only rest_sqlc/rest_sqlc.yaml must be generated, got root file: %v", err)
	}
	for path, expected := range checks {
		content, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), expected) {
			t.Fatalf("%s does not contain %q", path, expected)
		}
	}
	if err := GenerateProject(dir); err == nil {
		t.Fatal("expected existing SQLC files to be preserved")
	}
}

func TestGenerateExample(t *testing.T) {
	dir := t.TempDir()
	if err := GenerateExample(dir); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"rest_sqlc_example/rest_sqlc.yaml",
		"rest_sqlc_example/schema/studies.sql",
		"rest_sqlc_example/queries/studies.sql",
	} {
		if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRemoveExample(t *testing.T) {
	dir := t.TempDir()
	if err := GenerateExample(dir); err != nil {
		t.Fatal(err)
	}
	if err := RemoveExample(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "rest_sqlc_example")); !os.IsNotExist(err) {
		t.Fatalf("expected rest_sqlc_example to be removed, got %v", err)
	}
}
