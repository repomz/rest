package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/repomz/rest/internal/selfupdate"
)

func TestParseGenPath(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "default", want: "rest_config"},
		{name: "custom", args: []string{"--path", "configs/rest"}, want: "configs/rest"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseGenPath(test.args)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("config dir = %q, want %q", got, test.want)
			}
		})
	}
}

func TestParseGenPathRejectsInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{"--path"}, {"-config", "rest_config"}, {"--out", "."}, {"-out", "."}} {
		if _, err := parseGenPath(args); err == nil {
			t.Fatalf("expected error for arguments %v", args)
		}
	}
}

func TestRunRejectsLegacyGenerateCommand(t *testing.T) {
	if err := run([]string{"generate"}); err == nil {
		t.Fatal("expected legacy generate command to be rejected")
	}
}

func TestParseInitOptions(t *testing.T) {
	got, err := parseInitOptions([]string{"--sqlc", "--example", "--path", "project"})
	if err != nil {
		t.Fatal(err)
	}
	if got.path != "project" || !got.withSQLC || !got.withExample {
		t.Fatalf("unexpected init options: %+v", got)
	}
	if _, err := parseInitOptions([]string{"--config", "rest_config"}); err == nil {
		t.Fatal("expected unknown argument error")
	}
}

func TestParseUpdateOptions(t *testing.T) {
	got, err := parseUpdateOptions([]string{"--version", "v0.2.0", "--force"})
	if err != nil {
		t.Fatal(err)
	}
	if got.version != "v0.2.0" || !got.force {
		t.Fatalf("unexpected update options: %+v", got)
	}
	for _, args := range [][]string{{"--version"}, {"-config", "rest_config"}, {"--path", "."}, {"--sqlc"}} {
		if _, err := parseUpdateOptions(args); err == nil {
			t.Fatalf("expected error for arguments %v", args)
		}
	}
}

func TestParseChangelogOptions(t *testing.T) {
	got, err := parseChangelogOptions([]string{"--version", "v0.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if got.version != "v0.2.0" {
		t.Fatalf("version = %q", got.version)
	}
	for _, args := range [][]string{{"--version"}, {"--force"}, {"v0.2.0"}} {
		if _, err := parseChangelogOptions(args); err == nil {
			t.Fatalf("expected error for arguments %v", args)
		}
	}
}

func TestPrintUpdateResult(t *testing.T) {
	var output bytes.Buffer
	printUpdateResult(&output, selfupdate.Result{
		PreviousVersion: "v0.1.0",
		Version:         "v0.2.0",
		ReleaseNotes:    "Features:\n\n - abc1234 [update] Add release notes.",
	})
	for _, want := range []string{
		"Updating rest\n",
		"v0.1.0 -> v0.2.0\n",
		"Features:\n\n - abc1234 [update] Add release notes.\n",
		"You can see the changelog with `rest changelog`.\n",
		"Hooray! rest has been updated!\n",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("output does not contain %q:\n%s", want, output.String())
		}
	}
}

func TestResolveVersion(t *testing.T) {
	tests := []struct {
		name          string
		linkerVersion string
		buildVersion  string
		want          string
	}{
		{name: "release linker version", linkerVersion: "v1.2.3", buildVersion: "v1.2.2", want: "v1.2.3"},
		{name: "go install module version", linkerVersion: "dev", buildVersion: "v1.2.3", want: "v1.2.3"},
		{name: "local build", linkerVersion: "dev", buildVersion: "(devel)", want: "dev"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveVersion(test.linkerVersion, test.buildVersion); got != test.want {
				t.Fatalf("version = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRunInitModes(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		want       []string
		wantAbsent []string
	}{
		{
			name:       "configs only",
			want:       []string{"rest_config/rest.yaml", "rest_config/rest_sqlc.yaml", "rest_config/mongo_rest.yaml", "rest_config/rest_mongo/rest_cheatsheet.yaml", "rest_config/rest_mongo/rest_user_example.yaml"},
			wantAbsent: []string{"rest_config/auth_rest.yaml", "rest_config/sqlc_rest.yaml", "sqlc/sqlc.yaml", "sqlc_example/schema/studies.sql", "rest_sqlc/rest_sqlc.yaml", "rest_sqlc_example/schema/studies.sql"},
		},
		{
			name:       "sqlc",
			args:       []string{"--sqlc"},
			want:       []string{"rest_config/rest.yaml", "rest_config/rest_mongo/rest_cheatsheet.yaml", "rest_config/rest_mongo/rest_user_example.yaml", "rest_sqlc/rest_sqlc.yaml", "rest_sqlc/schema/item.sql", "rest_sqlc/queries/item.sql"},
			wantAbsent: []string{"rest_config/auth_rest.yaml", "rest_config/sqlc_rest.yaml", "sqlc/sqlc.yaml", "sqlc_example/schema/studies.sql", "rest_sqlc_example/schema/studies.sql"},
		},
		{
			name:       "example",
			args:       []string{"--example"},
			want:       []string{"rest_config/rest.yaml", "rest_config/rest_mongo/rest_cheatsheet.yaml", "rest_config/rest_mongo/rest_user_example.yaml", "rest_sqlc_example/rest_sqlc.yaml", "rest_sqlc_example/schema/studies.sql", "rest_sqlc_example/queries/studies.sql"},
			wantAbsent: []string{"rest_config/auth_rest.yaml", "rest_config/sqlc_rest.yaml", "sqlc/sqlc.yaml", "sqlc_example/schema/studies.sql", "rest_sqlc/rest_sqlc.yaml", "rest_sqlc/schema/item.sql", "rest_sqlc/queries/item.sql"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			args := append([]string{}, test.args...)
			args = append(args, "--path", dir)
			if err := runInit(args); err != nil {
				t.Fatal(err)
			}
			for _, path := range test.want {
				if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
					t.Fatalf("expected %s: %v", path, err)
				}
			}
			for _, path := range test.wantAbsent {
				if _, err := os.Stat(filepath.Join(dir, path)); !os.IsNotExist(err) {
					t.Fatalf("expected %s to be absent, got %v", path, err)
				}
			}
		})
	}
}

func TestRunInitRejectsSQLCAndExampleTogether(t *testing.T) {
	if err := runInit([]string{"--sqlc", "--example", "--path", t.TempDir()}); err == nil {
		t.Fatal("expected --sqlc and --example conflict")
	}
}

func TestRunGenValidatesYAMLBeforeGeneration(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rest_config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rest.yaml"), []byte("sql: enable\nsql: disable\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runGen([]string{"--path", dir})
	if err == nil || !strings.Contains(err.Error(), "duplicate key") {
		t.Fatalf("expected duplicate YAML key error, got %v", err)
	}
}
