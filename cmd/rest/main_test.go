package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfigDir(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "default", want: "rest_config"},
		{name: "custom", args: []string{"-config", "configs/rest"}, want: "configs/rest"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseConfigDir(test.args)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("config dir = %q, want %q", got, test.want)
			}
		})
	}
}

func TestParseConfigDirRejectsInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{"-config"}, {"-sqlc", "sqlc.yaml"}, {"-out", "."}} {
		if _, err := parseConfigDir(args); err == nil {
			t.Fatalf("expected error for arguments %v", args)
		}
	}
}

func TestParseInitOptions(t *testing.T) {
	got, err := parseInitOptions([]string{"--sqlc", "--example", "--out", "project"})
	if err != nil {
		t.Fatal(err)
	}
	if got.out != "project" || !got.withSQLC || !got.withExample {
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
	for _, args := range [][]string{{"--version"}, {"-config", "rest_config"}, {"--out", "."}, {"--sqlc"}} {
		if _, err := parseUpdateOptions(args); err == nil {
			t.Fatalf("expected error for arguments %v", args)
		}
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
			want:       []string{"rest_config/rest.yaml", "rest_config/sqlc_rest.yaml"},
			wantAbsent: []string{"sqlc/sqlc.yaml", "sqlc_example/schema/studies.sql"},
		},
		{
			name:       "sqlc",
			args:       []string{"--sqlc"},
			want:       []string{"rest_config/rest.yaml", "sqlc/sqlc.yaml", "sqlc/schema/item.sql", "sqlc/queries/item.sql"},
			wantAbsent: []string{"sqlc_example/schema/studies.sql"},
		},
		{
			name:       "example",
			args:       []string{"--example"},
			want:       []string{"rest_config/rest.yaml", "sqlc_example/sqlc.yaml", "sqlc_example/schema/studies.sql", "sqlc_example/queries/studies.sql"},
			wantAbsent: []string{"sqlc/sqlc.yaml", "sqlc/schema/item.sql", "sqlc/queries/item.sql"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			args := append([]string{}, test.args...)
			args = append(args, "--out", dir)
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
	if err := runInit([]string{"--sqlc", "--example", "--out", t.TempDir()}); err == nil {
		t.Fatal("expected --sqlc and --example conflict")
	}
}
