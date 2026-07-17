package toolchain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	CompatibleSQLCVersion = "v1.30.0"
	sqlcModule            = "github.com/sqlc-dev/sqlc/cmd/sqlc"
)

type SQLCResult struct {
	Path      string
	Version   string
	Installed bool
}

type sqlcManager struct {
	lookPath func(string) (string, error)
	output   func(context.Context, string, ...string) ([]byte, error)
	run      func(context.Context, io.Writer, io.Writer, []string, string, ...string) error
	getenv   func(string) string
	goos     string
	pathList string
}

func EnsureSQLC(ctx context.Context, output io.Writer) (SQLCResult, error) {
	return defaultSQLCManager().ensure(ctx, output)
}

func FindSQLC(ctx context.Context) (SQLCResult, error) {
	return defaultSQLCManager().find(ctx)
}

func SQLCInstallCommand() string {
	return "go install " + sqlcModule + "@" + CompatibleSQLCVersion
}

func defaultSQLCManager() sqlcManager {
	return sqlcManager{
		lookPath: exec.LookPath,
		output: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).CombinedOutput()
		},
		run: func(ctx context.Context, stdout, stderr io.Writer, env []string, name string, args ...string) error {
			cmd := exec.CommandContext(ctx, name, args...)
			cmd.Env = env
			cmd.Stdout = stdout
			cmd.Stderr = stderr
			return cmd.Run()
		},
		getenv:   os.Getenv,
		goos:     runtime.GOOS,
		pathList: string(os.PathListSeparator),
	}
}

func (m sqlcManager) ensure(ctx context.Context, output io.Writer) (SQLCResult, error) {
	if output == nil {
		output = io.Discard
	}
	foundVersion := ""
	if path, err := m.lookPath("sqlc"); err == nil {
		version, versionErr := m.version(ctx, path)
		if versionErr == nil && version == CompatibleSQLCVersion {
			return SQLCResult{Path: path, Version: version}, nil
		}
		if versionErr == nil {
			foundVersion = version
		}
	}

	binDir, err := m.goBinDir(ctx)
	if err != nil {
		return SQLCResult{}, err
	}
	path := filepath.Join(binDir, executableName("sqlc", m.goos))
	if version, versionErr := m.version(ctx, path); versionErr == nil && version == CompatibleSQLCVersion {
		return SQLCResult{Path: path, Version: version}, nil
	}

	switch {
	case foundVersion != "":
		fmt.Fprintf(output, "Installing sqlc %s required by rest (found %s).\n", CompatibleSQLCVersion, foundVersion)
	default:
		fmt.Fprintf(output, "Installing sqlc %s required by rest.\n", CompatibleSQLCVersion)
	}

	env := append(os.Environ(), "GOBIN="+binDir)
	if err := m.run(ctx, output, output, env, "go", "install", sqlcModule+"@"+CompatibleSQLCVersion); err != nil {
		return SQLCResult{}, fmt.Errorf("install sqlc %s: %w", CompatibleSQLCVersion, err)
	}
	version, err := m.version(ctx, path)
	if err != nil {
		return SQLCResult{}, fmt.Errorf("verify installed sqlc: %w", err)
	}
	if version != CompatibleSQLCVersion {
		return SQLCResult{}, fmt.Errorf("installed sqlc version is %s, expected %s", version, CompatibleSQLCVersion)
	}
	fmt.Fprintf(output, "sqlc %s installed at %s.\n", version, path)
	return SQLCResult{Path: path, Version: version, Installed: true}, nil
}

func (m sqlcManager) find(ctx context.Context) (SQLCResult, error) {
	var fallback SQLCResult
	if path, err := m.lookPath("sqlc"); err == nil {
		if version, versionErr := m.version(ctx, path); versionErr == nil {
			result := SQLCResult{Path: path, Version: version}
			if version == CompatibleSQLCVersion {
				return result, nil
			}
			fallback = result
		}
	}
	binDir, err := m.goBinDir(ctx)
	if err == nil {
		path := filepath.Join(binDir, executableName("sqlc", m.goos))
		if version, versionErr := m.version(ctx, path); versionErr == nil {
			result := SQLCResult{Path: path, Version: version}
			if version == CompatibleSQLCVersion {
				return result, nil
			}
			if fallback.Path == "" {
				fallback = result
			}
		}
	}
	if fallback.Path != "" {
		return fallback, nil
	}
	return SQLCResult{}, fmt.Errorf("sqlc is not installed")
}

func (m sqlcManager) version(ctx context.Context, path string) (string, error) {
	raw, err := m.output(ctx, path, "version")
	if err != nil {
		return "", fmt.Errorf("%s version: %w", path, err)
	}
	for _, field := range strings.Fields(string(raw)) {
		candidate := strings.TrimSpace(field)
		if strings.HasPrefix(candidate, "v") && len(candidate) > 1 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("cannot parse sqlc version from %q", strings.TrimSpace(string(raw)))
}

func (m sqlcManager) goBinDir(ctx context.Context) (string, error) {
	if value := strings.TrimSpace(m.getenv("GOBIN")); value != "" {
		return filepath.Clean(value), nil
	}
	if raw, err := m.output(ctx, "go", "env", "GOBIN"); err == nil {
		if value := strings.TrimSpace(string(raw)); value != "" {
			return filepath.Clean(value), nil
		}
	}
	raw, err := m.output(ctx, "go", "env", "GOPATH")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("Go is required to install sqlc; install Go and rerun rest")
		}
		return "", fmt.Errorf("resolve GOPATH for sqlc installation: %w", err)
	}
	goPath := strings.TrimSpace(string(raw))
	if goPath == "" {
		return "", fmt.Errorf("go env GOPATH returned an empty path")
	}
	paths := strings.Split(goPath, m.pathList)
	return filepath.Join(paths[0], "bin"), nil
}

func executableName(name, goos string) string {
	if goos == "windows" {
		return name + ".exe"
	}
	return name
}
