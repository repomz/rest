package toolchain

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEnsureSQLCUsesCompatibleBinaryWithoutInstalling(t *testing.T) {
	installed := false
	manager := sqlcManager{
		lookPath: func(string) (string, error) { return "/tools/sqlc", nil },
		output: func(_ context.Context, name string, args ...string) ([]byte, error) {
			if name == "/tools/sqlc" && len(args) == 1 && args[0] == "version" {
				return []byte(CompatibleSQLCVersion + "\n"), nil
			}
			return nil, errors.New("unexpected command")
		},
		run: func(context.Context, io.Writer, io.Writer, []string, string, ...string) error {
			installed = true
			return nil
		},
		getenv:   func(string) string { return "" },
		goos:     runtime.GOOS,
		pathList: string(filepath.ListSeparator),
	}

	result, err := manager.ensure(context.Background(), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if installed {
		t.Fatal("compatible sqlc was reinstalled")
	}
	if result.Path != "/tools/sqlc" || result.Version != CompatibleSQLCVersion || result.Installed {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestEnsureSQLCInstallsPinnedVersionIntoGoBin(t *testing.T) {
	binDir := t.TempDir()
	var command string
	var args []string
	installed := false
	manager := sqlcManager{
		lookPath: func(string) (string, error) { return "", errors.New("missing") },
		output: func(_ context.Context, name string, commandArgs ...string) ([]byte, error) {
			if installed && name == filepath.Join(binDir, executableName("sqlc", runtime.GOOS)) {
				return []byte(CompatibleSQLCVersion + "\n"), nil
			}
			return nil, errors.New("unexpected command")
		},
		run: func(_ context.Context, _, _ io.Writer, _ []string, name string, commandArgs ...string) error {
			command = name
			args = append([]string(nil), commandArgs...)
			installed = true
			return nil
		},
		getenv: func(name string) string {
			if name == "GOBIN" {
				return binDir
			}
			return ""
		},
		goos:     runtime.GOOS,
		pathList: string(filepath.ListSeparator),
	}
	var output bytes.Buffer

	result, err := manager.ensure(context.Background(), &output)
	if err != nil {
		t.Fatal(err)
	}
	if command != "go" || strings.Join(args, " ") != "install "+sqlcModule+"@"+CompatibleSQLCVersion {
		t.Fatalf("install command = %s %v", command, args)
	}
	if !result.Installed || result.Version != CompatibleSQLCVersion {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !strings.Contains(output.String(), "sqlc "+CompatibleSQLCVersion+" installed") {
		t.Fatalf("missing installation message:\n%s", output.String())
	}
}

func TestEnsureSQLCReusesManagedBinaryOutsidePath(t *testing.T) {
	binDir := t.TempDir()
	installCalls := 0
	manager := sqlcManager{
		lookPath: func(string) (string, error) { return "", errors.New("missing") },
		output: func(_ context.Context, name string, _ ...string) ([]byte, error) {
			if name == filepath.Join(binDir, executableName("sqlc", runtime.GOOS)) {
				return []byte(CompatibleSQLCVersion + "\n"), nil
			}
			return nil, errors.New("unexpected command")
		},
		run: func(context.Context, io.Writer, io.Writer, []string, string, ...string) error {
			installCalls++
			return nil
		},
		getenv: func(name string) string {
			if name == "GOBIN" {
				return binDir
			}
			return ""
		},
		goos:     runtime.GOOS,
		pathList: string(filepath.ListSeparator),
	}

	result, err := manager.ensure(context.Background(), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if installCalls != 0 || result.Installed || result.Path == "" {
		t.Fatalf("managed sqlc was not reused: calls=%d result=%+v", installCalls, result)
	}
}

func TestEnsureSQLCReplacesIncompatibleVersion(t *testing.T) {
	binDir := t.TempDir()
	installCalls := 0
	installed := false
	manager := sqlcManager{
		lookPath: func(string) (string, error) { return "/tools/sqlc", nil },
		output: func(_ context.Context, name string, _ ...string) ([]byte, error) {
			if name == "/tools/sqlc" {
				return []byte("v1.31.1\n"), nil
			}
			if installed && name == filepath.Join(binDir, executableName("sqlc", runtime.GOOS)) {
				return []byte(CompatibleSQLCVersion + "\n"), nil
			}
			return nil, errors.New("unexpected command")
		},
		run: func(context.Context, io.Writer, io.Writer, []string, string, ...string) error {
			installCalls++
			installed = true
			return nil
		},
		getenv: func(name string) string {
			if name == "GOBIN" {
				return binDir
			}
			return ""
		},
		goos:     runtime.GOOS,
		pathList: string(filepath.ListSeparator),
	}

	result, err := manager.ensure(context.Background(), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if installCalls != 1 || !result.Installed {
		t.Fatalf("incompatible version was not replaced: calls=%d result=%+v", installCalls, result)
	}
}

func TestSQLCInstallCommandUsesCompatibleVersion(t *testing.T) {
	want := "go install " + sqlcModule + "@" + CompatibleSQLCVersion
	if got := SQLCInstallCommand(); got != want {
		t.Fatalf("install command = %q, want %q", got, want)
	}
}

func TestCompatibleSQLCVersionReferencesStayAligned(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve repository root")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	for _, name := range []string{
		"Makefile",
		"README.md",
		filepath.Join("internal", "config", "templates", "rest.yaml"),
	} {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), CompatibleSQLCVersion) {
			t.Fatalf("%s does not reference compatible sqlc %s", name, CompatibleSQLCVersion)
		}
	}
}
