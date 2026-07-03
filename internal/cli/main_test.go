package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/repomz/rest/internal/appgen"
	"github.com/repomz/rest/internal/selfupdate"
)

func TestRunRejectsLegacyGenerateCommand(t *testing.T) {
	if err := run([]string{"generate"}); err == nil {
		t.Fatal("expected legacy generate command to be rejected")
	}
}

func TestFormatErrorAddsUsefulHints(t *testing.T) {
	got := FormatError(os.ErrNotExist)
	if got != os.ErrNotExist.Error() {
		t.Fatalf("unexpected hint for generic error: %q", got)
	}

	got = FormatError(&execError{text: `sqlc: executable file not found`})
	for _, want := range []string{
		"Install sqlc",
		"rest doctor",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted error missing %q:\n%s", want, got)
		}
	}
}

type execError struct {
	text string
}

func (e *execError) Error() string {
	return e.text
}

func TestParseInitOptions(t *testing.T) {
	got, err := parseInitOptions([]string{"--example", "sql"})
	if err != nil {
		t.Fatal(err)
	}
	if got.example != "sql" {
		t.Fatalf("unexpected init options: %+v", got)
	}
	for _, args := range [][]string{{"--config", "rest_config"}, {"--path", "."}, {"--sqlc"}, {"--example"}, {"--example", "bad"}} {
		if _, err := parseInitOptions(args); err == nil {
			t.Fatalf("expected unknown argument error for %v", args)
		}
	}
}

func TestParseUpdateOptions(t *testing.T) {
	got, err := parseUpdateOptions([]string{"--version", "v0.2.0", "--force", "--check"})
	if err != nil {
		t.Fatal(err)
	}
	if got.version != "v0.2.0" || !got.force || !got.check {
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
		t.Fatalf("version = %q, want %q", got.version, "v0.2.0")
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
		Checksum:        strings.Repeat("a", 64),
	})
	for _, want := range []string{
		"Updating rest\n",
		"v0.1.0 -> v0.2.0\n",
		"Verified SHA-256: " + strings.Repeat("a", 64) + "\n",
		"Features:\n\n - abc1234 [update] Add release notes.\n",
		"You can see the changelog with `rest changelog`.\n",
		"Hooray! rest has been updated!\n",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("output does not contain %q:\n%s", want, output.String())
		}
	}
}

func TestPrintUpdateCheckResult(t *testing.T) {
	var output bytes.Buffer
	printUpdateCheckResult(&output, selfupdate.Result{
		PreviousVersion: "v0.1.0",
		Version:         "v0.2.0",
		Available:       true,
	})
	if !strings.Contains(output.String(), "New rest version available: v0.2.0") {
		t.Fatalf("unexpected check output:\n%s", output.String())
	}
	output.Reset()
	printUpdateCheckResult(&output, selfupdate.Result{Version: "v0.2.0"})
	if !strings.Contains(output.String(), "rest is already up to date (v0.2.0)") {
		t.Fatalf("unexpected check output:\n%s", output.String())
	}
}

func TestConfirmInitUpdate(t *testing.T) {
	for _, test := range []struct {
		input string
		want  bool
	}{
		{input: "y\n", want: true},
		{input: "yes\n", want: true},
		{input: "\n", want: false},
		{input: "no\n", want: false},
	} {
		var output bytes.Buffer
		got := confirmInitUpdate(strings.NewReader(test.input), &output, selfupdate.Result{
			PreviousVersion: "v0.1.0",
			Version:         "v0.2.0",
		})
		if got != test.want {
			t.Fatalf("confirmInitUpdate(%q) = %v, want %v", test.input, got, test.want)
		}
		if !strings.Contains(output.String(), "A newer rest version is available: v0.2.0") {
			t.Fatalf("unexpected prompt output:\n%s", output.String())
		}
	}
}

func TestPrintWelcomeBanner(t *testing.T) {
	var output bytes.Buffer
	printWelcomeBanner(&output, false)
	text := output.String()
	for _, want := range []string{"Give yourself a little", "____  _____ ____ _____", "Write queries.", "Get an application.", "Add business logic."} {
		if !strings.Contains(text, want) {
			t.Fatalf("welcome banner does not contain %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "\x1b[") {
		t.Fatalf("non-colored welcome banner must not contain ANSI escapes:\n%q", text)
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
			name: "default",
			want: []string{
				"rest_config/rest.yaml",
				"rest_config/rest_sqlc.yaml",
				"rest_config/mongo_rest.yaml",
				"rest_config/rest_mongo/rest_cheatsheet.yaml",
				"rest_config/rest_mongo/rest_user_example.yaml",
				"rest_sqlc/rest_sqlc.yaml",
				"rest_sqlc/schema/item.sql",
				"rest_sqlc/queries/item.sql",
			},
			wantAbsent: []string{
				"rest_config/auth_rest.yaml",
				"rest_config/sqlc_rest.yaml",
				"sqlc/sqlc.yaml",
				"sqlc_example/schema/studies.sql",
				"rest_sqlc_example/schema/studies.sql",
			},
		},
		{
			name: "sql example",
			args: []string{"--example", "sql"},
			want: []string{
				"rest_config/rest.yaml",
				"rest_config/rest_mongo/rest_cheatsheet.yaml",
				"rest_config/rest_mongo/rest_user_example.yaml",
				"rest_sqlc_example/rest_sqlc.yaml",
				"rest_sqlc_example/schema/studies.sql",
				"rest_sqlc_example/queries/studies.sql",
			},
			wantAbsent: []string{
				"rest_config/auth_rest.yaml",
				"rest_config/sqlc_rest.yaml",
				"sqlc/sqlc.yaml",
				"sqlc_example/schema/studies.sql",
				"rest_sqlc/rest_sqlc.yaml",
				"rest_sqlc/schema/item.sql",
				"rest_sqlc/queries/item.sql",
			},
		},
		{
			name: "mongo example",
			args: []string{"--example", "mongo"},
			want: []string{
				"rest_config/rest.yaml",
				"rest_config/rest_sqlc.yaml",
				"rest_config/mongo_rest.yaml",
				"rest_config/rest_mongo/rest_cheatsheet.yaml",
				"rest_config/rest_mongo/rest_user_example.yaml",
				"rest_config/rest_mongo/item.yaml",
			},
			wantAbsent: []string{
				"rest_config/auth_rest.yaml",
				"rest_sqlc/rest_sqlc.yaml",
				"rest_sqlc_example/rest_sqlc.yaml",
				"sqlc/sqlc.yaml",
				"sqlc_example/schema/studies.sql",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			withWorkingDir(t, dir)
			if err := runInit(test.args); err != nil {
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

func TestRunInitUsesExistingSQLCConfig(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test/existing\n\ngo 1.25.11\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sqlc.yaml"), []byte("version: \"2\"\nsql: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runInit(nil); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "rest_config", "rest_sqlc.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "  sqlc_path: ../sqlc.yaml") {
		t.Fatalf("rest_sqlc.yaml must point to existing sqlc.yaml:\n%s", content)
	}
	if _, err := os.Stat(filepath.Join(dir, "rest_sqlc", "rest_sqlc.yaml")); !os.IsNotExist(err) {
		t.Fatalf("rest_sqlc skeleton must not be generated when existing sqlc.yaml is used, got %v", err)
	}
	goMod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(goMod), "module example.test/existing") {
		t.Fatalf("existing go.mod was modified unexpectedly:\n%s", goMod)
	}
}

func TestRunInitRejectsRemovedArguments(t *testing.T) {
	for _, args := range [][]string{{"--sqlc"}, {"--path", "."}} {
		if err := runInit(args); err == nil {
			t.Fatalf("expected removed init argument to be rejected: %v", args)
		}
	}
}

func TestRunGenValidatesYAMLBeforeGeneration(t *testing.T) {
	root := t.TempDir()
	withWorkingDir(t, root)
	dir := filepath.Join(root, "rest_config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rest.yaml"), []byte("sql: enable\nsql: disable\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runGen(nil)
	if err == nil || !strings.Contains(err.Error(), "duplicate key") {
		t.Fatalf("expected duplicate YAML key error, got %v", err)
	}
}

func TestRunGenRejectsRemovedPathArgument(t *testing.T) {
	if err := runGen([]string{"--path", "rest_config"}); err == nil {
		t.Fatal("expected removed gen path argument to be rejected")
	}
}

func TestRunListRejectsUnknownArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"routes"}, {"endpoints", "--json"}} {
		if err := runList(args); err == nil {
			t.Fatalf("expected list arguments to be rejected: %v", args)
		}
	}
}

func TestPrintEndpointList(t *testing.T) {
	var output bytes.Buffer
	printEndpointList(&output, []appgen.EndpointInfo{
		{Name: "Health", Method: "GET", Path: "/health", Source: "system", Access: "public"},
		{Name: "CreateItem", Method: "POST", Path: "/items", Source: "mongo", Access: "auth", Roles: []string{"admin"}},
	})
	text := output.String()
	for _, want := range []string{
		"METHOD", "PATH", "NAME", "SOURCE", "ACCESS", "ROLES",
		"GET", "/health", "Health", "system", "public",
		"POST", "/items", "CreateItem", "mongo", "auth", "admin",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("endpoint list output does not contain %q:\n%s", want, text)
		}
	}
}

func TestRunDoctorRejectsArguments(t *testing.T) {
	if err := runDoctor([]string{"--json"}); err == nil {
		t.Fatal("expected doctor to reject arguments")
	}
}

func TestRunDoctorReportsMissingConfig(t *testing.T) {
	root := t.TempDir()
	withWorkingDir(t, root)
	report := runDoctorChecks("rest_config")
	if report.Errors() == 0 {
		t.Fatalf("expected missing rest_config to be an error: %+v", report.Checks)
	}
	var output bytes.Buffer
	report.Print(&output)
	for _, want := range []string{"REST Doctor", "rest_config directory is missing", "Summary:"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("doctor output does not contain %q:\n%s", want, output.String())
		}
	}
}

func TestRunDoctorAfterMongoExampleInit(t *testing.T) {
	root := t.TempDir()
	withWorkingDir(t, root)
	if err := runInit([]string{"--example", "mongo"}); err != nil {
		t.Fatal(err)
	}
	report := runDoctorChecks("rest_config")
	if report.Errors() != 0 {
		var output bytes.Buffer
		report.Print(&output)
		t.Fatalf("expected mongo example init to be doctor-valid before generation:\n%s", output.String())
	}
}

func withWorkingDir(t *testing.T, dir string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})
}
