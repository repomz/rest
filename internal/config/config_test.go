package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configtemplates "github.com/repomz/rest_generator/internal/config/templates"
	"gopkg.in/yaml.v3"
)

func TestGenerateCopiesCanonicalYAMLFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rest_config")
	if err := Generate(dir); err != nil {
		t.Fatal(err)
	}

	err := fs.WalkDir(configtemplates.Files, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".yaml" {
			return err
		}
		want, err := configtemplates.Files.ReadFile(path)
		if err != nil {
			return err
		}
		got, err := os.ReadFile(filepath.Join(dir, filepath.Base(path)))
		if err != nil {
			return err
		}
		if string(got) != string(want) {
			t.Fatalf("%s differs from the canonical template", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedConfigsKeepDocumentation(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rest_config")
	if err := Generate(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"rest.yaml", "sqlc_rest.yaml", "mongo_rest.yaml", "auth_rest.yaml"} {
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		if !strings.Contains(text, "# ==============================================================================") || !strings.Contains(text, "# ------------------------------------------------------------------------------") {
			t.Fatalf("%s must retain visual sections and documentation comments", name)
		}
	}
}

func TestGenerateForSQLCEnablesSQLC(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rest_config")
	if err := GenerateForSQLC(dir); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "sqlc_rest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "  enable: enable") {
		t.Fatalf("SQLC bootstrap config must enable SQLC:\n%s", content)
	}
}

func TestGenerateForExampleUsesExampleSQLCPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rest_config")
	if err := GenerateForExample(dir); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "sqlc_rest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "  enable: enable") || !strings.Contains(text, "  sqlc_path: ../sqlc_example/sqlc.yaml") {
		t.Fatalf("example config must enable SQLC and point to sqlc_example:\n%s", content)
	}
}

func TestFutureFeatureConfigsAreValidContracts(t *testing.T) {
	mongo := readEmbeddedYAMLMap(t, "mongo_rest.yaml")
	for _, key := range []string{"version", "driver", "connection", "generation", "models", "indexes", "methods", "hooks"} {
		requireMapKey(t, mongo, key)
	}
	if _, exists := mongo["enabled"]; exists {
		t.Fatal("mongo_rest.yaml must not duplicate the rest.yaml feature switch")
	}
	assertMongoIndexesReferenceExistingFields(t, mongo)

	auth := readEmbeddedYAMLMap(t, "auth_rest.yaml")
	for _, key := range []string{"version", "identity", "endpoints", "authentication", "password", "authorization", "cookies", "features"} {
		requireMapKey(t, auth, key)
	}
	if _, exists := auth["auth"]; exists {
		t.Fatal("auth_rest.yaml must not wrap settings in a duplicate auth.enabled section")
	}
	identity := requireMapValue(t, auth, "identity")
	if identity["provider"] == "" || identity["model"] == "" {
		t.Fatal("auth identity provider and model are required")
	}
	authentication := requireMapValue(t, auth, "authentication")
	if len(requireSliceValue(t, authentication, "strategies")) == 0 {
		t.Fatal("auth must define at least one authentication strategy")
	}
	authorization := requireMapValue(t, auth, "authorization")
	if authorization["default_policy"] != "deny" {
		t.Fatal("authorization must use deny as the safe default policy")
	}
}

func TestRestConfigUsesOnlySupportedOptionalFeatures(t *testing.T) {
	rest := readEmbeddedYAMLMap(t, "rest.yaml")
	for _, removed := range []string{"docker-compose", "docker_compose"} {
		if _, exists := rest[removed]; exists {
			t.Fatalf("removed feature %q must not remain in rest.yaml", removed)
		}
	}
	observability := requireMapValue(t, rest, "observability")
	if _, exists := observability["tracing"]; exists {
		t.Fatal("removed tracing feature must not remain in rest.yaml")
	}
	if _, exists := rest["safe_reload"]; !exists {
		t.Fatal("rest.yaml must define the safe_reload switch")
	}
	if _, exists := rest["auto_sqlc"]; !exists {
		t.Fatal("rest.yaml must define the auto_sqlc switch")
	}
	http := requireMapValue(t, rest, "http")
	if _, exists := http["database_pool"]; exists {
		t.Fatal("database/sql pooling is an implementation detail and must not be an HTTP feature switch")
	}
	gracefulShutdown := requireMapValue(t, http, "graceful_shutdown")
	if gracefulShutdown["enabled"] != true && gracefulShutdown["enabled"] != "enable" && gracefulShutdown["enabled"] != "enabled" {
		t.Fatal("http.graceful_shutdown.enabled must be present and enabled")
	}
	features := requireMapValue(t, rest, "features")
	for _, removed := range []string{"safe_app_reload", "safe_config_reload"} {
		if _, exists := features[removed]; exists {
			t.Fatalf("removed feature %q must not remain in rest.yaml", removed)
		}
	}
	for _, key := range []string{"makefile", "gitignore", "env", "init_db", "ci", "cd"} {
		section := requireMapValue(t, features, key)
		if _, exists := section["enabled"]; !exists {
			t.Fatalf("feature %q must have an explicit enabled switch", key)
		}
	}
	docker := requireMapValue(t, rest, "docker")
	if docker["enabled"] != true || docker["output"] == "" {
		t.Fatal("Docker contract must define enabled and output")
	}
	metrics := requireMapValue(t, observability, "metrics")
	if _, exists := metrics["enabled"]; !exists {
		t.Fatal("metrics must have an explicit enabled switch")
	}
	collect := requireMapValue(t, metrics, "collect")
	if _, exists := collect["database_pool"]; exists {
		t.Fatal("database pool reuse is not a configurable feature")
	}
}

func readEmbeddedYAMLMap(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	content, err := configtemplates.Files.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]interface{}
	if err := yaml.Unmarshal(content, &result); err != nil {
		t.Fatalf("%s is not valid YAML: %v", path, err)
	}
	return result
}

func requireMapKey(t *testing.T, values map[string]interface{}, key string) {
	t.Helper()
	if _, ok := values[key]; !ok {
		t.Fatalf("required config section %q is missing", key)
	}
}

func requireMapValue(t *testing.T, values map[string]interface{}, key string) map[string]interface{} {
	t.Helper()
	value, ok := values[key].(map[string]interface{})
	if !ok {
		t.Fatalf("config value %q is not an object: %#v", key, values[key])
	}
	return value
}

func requireSliceValue(t *testing.T, values map[string]interface{}, key string) []interface{} {
	t.Helper()
	value, ok := values[key].([]interface{})
	if !ok {
		t.Fatalf("config value %q is not an array: %#v", key, values[key])
	}
	return value
}

func assertMongoIndexesReferenceExistingFields(t *testing.T, mongo map[string]interface{}) {
	t.Helper()
	modelFields := map[string]map[string]bool{}
	for _, rawModel := range requireSliceValue(t, mongo, "models") {
		model, ok := rawModel.(map[string]interface{})
		if !ok {
			t.Fatalf("Mongo model is not an object: %#v", rawModel)
		}
		name, _ := model["name"].(string)
		fields := map[string]bool{}
		for _, rawField := range requireSliceValue(t, model, "fields") {
			field, ok := rawField.(map[string]interface{})
			if !ok {
				t.Fatalf("Mongo field is not an object: %#v", rawField)
			}
			fieldName, _ := field["name"].(string)
			fields[fieldName] = true
		}
		modelFields[name] = fields
	}
	for _, rawIndex := range requireSliceValue(t, mongo, "indexes") {
		index, ok := rawIndex.(map[string]interface{})
		if !ok {
			t.Fatalf("Mongo index is not an object: %#v", rawIndex)
		}
		modelName, _ := index["model"].(string)
		fields, exists := modelFields[modelName]
		if !exists {
			t.Fatalf("Mongo index references unknown model %q", modelName)
		}
		for _, rawKey := range requireSliceValue(t, index, "keys") {
			key, ok := rawKey.(map[string]interface{})
			if !ok {
				t.Fatalf("Mongo index key is not an object: %#v", rawKey)
			}
			fieldName, _ := key["field"].(string)
			if !fields[fieldName] {
				t.Fatalf("Mongo index for %s references unknown field %q", modelName, fieldName)
			}
		}
	}
}

func TestTestingSupportsLegacyHandlerTestKey(t *testing.T) {
	dir := t.TempDir()
	rest := `module: example.test/app
project_path: ./app
sql: false
testing:
  testing.T: enabled
  curl: disabled
`
	if err := os.WriteFile(filepath.Join(dir, "rest.yaml"), []byte(rest), 0o644); err != nil {
		t.Fatal(err)
	}
	bundle, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !bundle.Rest.Testing.HandlerTests.Bool() {
		t.Fatal("legacy testing.T key must remain supported")
	}
	if strings.Contains(rest, "handler_tests") {
		t.Fatal("test fixture must exercise only the legacy key")
	}
}

func TestGenerateDoesNotOverwriteEditedConfig(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rest_config")
	if err := Generate(dir); err != nil {
		t.Fatal(err)
	}
	if err := Generate(dir); err == nil {
		t.Fatal("expected an error when config files already exist")
	}
}

func TestGenerateConflictDoesNotCreatePartialConfigSet(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rest_config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rest.yaml"), []byte("edited: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Generate(dir); err == nil {
		t.Fatal("expected existing config conflict")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "rest.yaml" {
		t.Fatalf("config generation left a partial set: %#v", entries)
	}
}

func TestSQLConfigSupportsLegacyPasswordKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rest.yaml"), []byte("sql: enabled\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sqlConfig := "db_connection:\n  usere_password: legacy-secret\nsqlc:\n  sqlc_path: sqlc.yaml\n"
	if err := os.WriteFile(filepath.Join(dir, "sqlc_rest.yaml"), []byte(sqlConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	bundle, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.SQL.Connection.UserPassword != "legacy-secret" {
		t.Fatalf("legacy password key was not normalized: %+v", bundle.SQL.Connection)
	}
}

func TestLoadFeatureSwitches(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rest_config")
	if err := Generate(dir); err != nil {
		t.Fatal(err)
	}
	bundle, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !bundle.Rest.SQL.Bool() || !bundle.Rest.AutoSQLC.Bool() || bundle.Rest.Mongo.Bool() || bundle.Rest.Auth.Bool() {
		t.Fatalf("unexpected data feature switches: %+v", bundle.Rest)
	}
	if bundle.SQL == nil || bundle.SQL.SQLC.Path == "" {
		t.Fatalf("sql config was not loaded: %+v", bundle.SQL)
	}
	if bundle.SQL.SQLC.Enabled.Bool() || !bundle.SQL.SQLC.Example.Bool() {
		t.Fatalf("unexpected canonical SQLC switches: %+v", bundle.SQL.SQLC)
	}
	if !bundle.Rest.HTTP.Middleware.Recovery.Enabled.Bool() || !bundle.Rest.HTTP.Middleware.CORS.Enabled.Bool() {
		t.Fatal("HTTP middleware switches were not loaded")
	}
	if !bundle.Rest.Logging.Enabled.Bool() || !bundle.Rest.OpenAPI.Enabled.Bool() {
		t.Fatal("logging and OpenAPI must be enabled by the canonical config")
	}
	if !bundle.Rest.Testing.HandlerTests.Bool() || !bundle.Rest.Testing.Curl.Bool() {
		t.Fatal("testing switches were not loaded")
	}
	if bundle.Rest.Features.Makefile.Enabled.Bool() {
		t.Fatal("makefile must remain disabled in the canonical config")
	}
}
