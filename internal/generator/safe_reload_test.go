package generator

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeReloadKeepsUserChanges(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal/app/domain/item.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	reload := newSafeReload(root, strings.NewReader("a\n"), &bytes.Buffer{})
	if err := reload.save([]string{"internal/app/domain/item.go"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("user change\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	reload = newSafeReload(root, strings.NewReader("a\n"), &out)
	preserved, err := reload.resolve([]string{"internal/app/domain/item.go"})
	if err != nil {
		t.Fatal(err)
	}
	if string(preserved["internal/app/domain/item.go"]) != "user change\n" {
		t.Fatalf("expected preserved user change, got %q", preserved["internal/app/domain/item.go"])
	}
	if !strings.Contains(out.String(), "diff --git a/internal/app/domain/item.go b/internal/app/domain/item.go") {
		t.Fatalf("expected diff output, got %q", out.String())
	}

	if err := os.WriteFile(path, []byte("new generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := reload.restore(preserved); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "user change\n" {
		t.Fatalf("expected restored user change, got %q", content)
	}

	if err := os.WriteFile(path, []byte("new generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := reload.save([]string{"internal/app/domain/item.go"}); err != nil {
		t.Fatal(err)
	}
	if err := reload.restore(preserved); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	reload = newSafeReload(root, strings.NewReader("b\n"), &out)
	preserved, err = reload.resolve([]string{"internal/app/domain/item.go"})
	if err != nil {
		t.Fatal(err)
	}
	if len(preserved) != 0 {
		t.Fatalf("expected no preserved files after overwrite answer, got %d", len(preserved))
	}
	if !strings.Contains(out.String(), "-new generated\n+user change\n") {
		t.Fatalf("expected snapshot to remain generated output, got %q", out.String())
	}
}

func TestSafeReloadOverwritesUserChanges(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal/app/domain/item.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	reload := newSafeReload(root, strings.NewReader("d\n"), &bytes.Buffer{})
	if err := reload.save([]string{"internal/app/domain/item.go"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("user change\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	preserved, err := reload.resolve([]string{"internal/app/domain/item.go"})
	if err != nil {
		t.Fatal(err)
	}
	if len(preserved) != 0 {
		t.Fatalf("expected no preserved files, got %d", len(preserved))
	}
}
