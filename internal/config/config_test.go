package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configtemplates "github.com/repomz/rest/internal/config/templates"
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
		if filepath.Base(path) == "auth_rest.yaml" {
			if _, err := os.Stat(filepath.Join(dir, "auth_rest.yaml")); !os.IsNotExist(err) {
				t.Fatalf("auth_rest.yaml must not be created by rest init")
			}
			return nil
		}
		want, err := configtemplates.Files.ReadFile(path)
		if err != nil {
			return err
		}
		got, err := os.ReadFile(filepath.Join(dir, filepath.ToSlash(path)))
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
	for _, name := range []string{"rest.yaml", "rest_sqlc.yaml", "mongo_rest.yaml", "rest_mongo/rest_cheatsheet.yaml", "rest_mongo/rest_user_example.yaml"} {
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
	content, err := os.ReadFile(filepath.Join(dir, "rest_sqlc.yaml"))
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
	content, err := os.ReadFile(filepath.Join(dir, "rest_sqlc.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "  enable: enable") || !strings.Contains(text, "  sqlc_path: ../rest_sqlc_example/rest_sqlc.yaml") {
		t.Fatalf("example config must enable SQLC and point to rest_sqlc_example:\n%s", content)
	}
}

func TestGenerateForMongoExampleEnablesMongoAndWritesActiveContract(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rest_config")
	if err := GenerateForMongoExample(dir); err != nil {
		t.Fatal(err)
	}
	rest, err := os.ReadFile(filepath.Join(dir, "rest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(rest)
	for _, expected := range []string{"sql: disable", "auto_sqlc: disable", "mongo: enable", "  env:\n    enabled: true"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("mongo example rest.yaml missing %q:\n%s", expected, text)
		}
	}
	contract, err := os.ReadFile(filepath.Join(dir, "rest_mongo", "item.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contract), "collection: items") {
		t.Fatalf("mongo example contract is not active item contract:\n%s", contract)
	}
}

func TestFutureFeatureConfigsAreValidContracts(t *testing.T) {
	mongo := readEmbeddedYAMLMap(t, "mongo_rest.yaml")
	for _, key := range []string{"version", "connection", "engine", "mongo"} {
		requireMapKey(t, mongo, key)
	}
	mongoSettings := requireMapValue(t, mongo, "mongo")
	requireMapKey(t, mongoSettings, "models_path")
	if mongoSettings["models_path"] != "rest_mongo" {
		t.Fatal("mongo.models_path must point to rest_mongo directory")
	}
	if _, exists := mongoSettings["enable"]; exists {
		t.Fatal("mongo_rest.yaml must not duplicate rest.yaml mongo switch inside mongo.enable")
	}
	for _, removed := range []string{"driver", "hooks", "models", "indexes", "methods", "generation"} {
		if _, exists := mongo[removed]; exists {
			t.Fatalf("mongo_rest.yaml should not expose premature %q settings", removed)
		}
	}
	if _, exists := mongo["enabled"]; exists {
		t.Fatal("mongo_rest.yaml must not duplicate the rest.yaml feature switch")
	}

	cheatsheet := readEmbeddedYAMLMap(t, "rest_mongo/rest_cheatsheet.yaml")
	requireMapKey(t, cheatsheet, "version")
	if _, exists := cheatsheet["models"]; exists {
		t.Fatal("rest_mongo/rest_cheatsheet.yaml must be documentation only, not an active model")
	}

	mongoExamples := readEmbeddedYAMLMap(t, "rest_mongo/rest_user_example.yaml")
	for _, key := range []string{"version", "models", "indexes", "methods"} {
		requireMapKey(t, mongoExamples, key)
	}
	assertMongoIndexesReferenceExistingFields(t, mongoExamples)
	assertMongoExamplesShowSupportedMethods(t, mongoExamples)

	auth := readEmbeddedYAMLMap(t, "auth_rest.yaml")
	for _, key := range []string{"version", "identity", "endpoints", "authentication", "authorization"} {
		requireMapKey(t, auth, key)
	}
	if _, exists := auth["auth"]; exists {
		t.Fatal("auth_rest.yaml must not wrap settings in a duplicate auth.enabled section")
	}
	identity := requireMapValue(t, auth, "identity")
	for _, key := range []string{"model", "table", "id_field", "username_field", "password_field", "roles_field", "claims_model"} {
		requireMapKey(t, identity, key)
	}
	authentication := requireMapValue(t, auth, "authentication")
	if authentication["strategy"] != "jwt" {
		t.Fatal("auth must define the supported JWT strategy")
	}
	jwt := requireMapValue(t, authentication, "jwt")
	for _, key := range []string{"algorithm", "signing_key_env", "verification_key_file_env", "access_token_ttl", "refresh_token", "refresh_token_storage", "leeway", "header_name", "token_prefix"} {
		requireMapKey(t, jwt, key)
	}
	basic := requireMapValue(t, authentication, "basic")
	for _, key := range []string{"username_env", "password_env", "realm", "roles"} {
		requireMapKey(t, basic, key)
	}
	authorization := requireMapValue(t, auth, "authorization")
	if authorization["default_policy"] != "deny" {
		t.Fatal("authorization must use deny as the safe default policy")
	}
}

func TestGenerateAuthWritesEndpointPolicies(t *testing.T) {
	dir := t.TempDir()
	endpoints := []GeneratedEndpoint{
		{Name: "CreateUser", Method: "POST", Path: "/api/users"},
		{Name: "Health", Method: "GET", Path: "/api/health", Public: true},
	}
	if err := GenerateAuth(dir, endpoints); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "auth_rest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, expected := range []string{`name: "CreateUser"`, `path: "/api/users"`, "require_auth: true", `name: "Health"`, "public: true"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("generated auth config missing %q:\n%s", expected, text)
		}
	}
	for _, comment := range []string{
		"# Active strategy: jwt or basic.",
		"# Environment variable containing the HS256 signing key.",
		"# Realm returned in the WWW-Authenticate challenge.",
		"# Stable generated handler name; informational only.",
		"# Allowed roles; [] permits any authenticated user.",
	} {
		if !strings.Contains(text, comment) {
			t.Fatalf("generated auth config missing field comment %q:\n%s", comment, text)
		}
	}
}

func TestSyncAuthPreservesPoliciesAndAddsNewEndpoints(t *testing.T) {
	dir := t.TempDir()
	current := defaultAuth()
	current.Endpoints = []AuthEndpoint{{
		Name: "ListUsers", Method: "GET", Path: "/users",
		RequireAuth: true, Roles: []string{"admin"},
	}}
	changed, err := SyncAuth(dir, &current, []GeneratedEndpoint{
		{Name: "ListAllUsers", Method: "GET", Path: "/users"},
		{Name: "CreateUser", Method: "POST", Path: "/users"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected endpoint inventory to change")
	}
	var loaded Auth
	if err := readYAML(filepath.Join(dir, "auth_rest.yaml"), &loaded); err != nil {
		t.Fatal(err)
	}
	if len(loaded.Endpoints) != 2 {
		t.Fatalf("endpoints = %+v", loaded.Endpoints)
	}
	if loaded.Endpoints[0].Name != "ListAllUsers" || len(loaded.Endpoints[0].Roles) != 1 || loaded.Endpoints[0].Roles[0] != "admin" {
		t.Fatalf("existing policy was not preserved: %+v", loaded.Endpoints[0])
	}
	if !loaded.Endpoints[1].RequireAuth || loaded.Endpoints[1].Public {
		t.Fatalf("new endpoint must be authenticated by default: %+v", loaded.Endpoints[1])
	}
}

func TestRestConfigUsesOnlySupportedOptionalFeatures(t *testing.T) {
	rest := readEmbeddedYAMLMap(t, "rest.yaml")
	if _, exists := rest["docker-compose"]; exists {
		t.Fatal("docker compose settings must live under docker.compose, not docker-compose")
	}
	if _, exists := rest["docker_compose"]; exists {
		t.Fatal("docker compose settings must live under docker.compose, not docker_compose")
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
	if _, exists := rest["environment"]; exists {
		t.Fatal("top-level environment must not remain in rest.yaml until it affects generation")
	}
	if _, exists := rest["language"]; exists {
		t.Fatal("top-level language must not remain in rest.yaml while only Go generation is supported")
	}
	if module, _ := rest["module"].(string); module == "" {
		t.Fatal("rest.yaml must define the Go module path")
	}
	http := requireMapValue(t, rest, "http")
	if _, exists := http["database_pool"]; exists {
		t.Fatal("database/sql pooling is an implementation detail and must not be an HTTP feature switch")
	}
	gracefulShutdown := requireMapValue(t, http, "graceful_shutdown")
	if gracefulShutdown["enabled"] != true && gracefulShutdown["enabled"] != "enable" && gracefulShutdown["enabled"] != "enabled" {
		t.Fatal("http.graceful_shutdown.enabled must be present and enabled")
	}
	readiness := requireMapValue(t, http, "readiness")
	if readiness["enabled"] != true || readiness["path"] != "/ready" {
		t.Fatalf("http.readiness must be enabled at /ready, got %+v", readiness)
	}
	features := requireMapValue(t, rest, "features")
	for _, removed := range []string{"safe_app_reload", "safe_config_reload"} {
		if _, exists := features[removed]; exists {
			t.Fatalf("removed feature %q must not remain in rest.yaml", removed)
		}
	}
	for _, key := range []string{"makefile", "gitignore", "env", "init_db", "deployment_guide", "ci", "cd"} {
		section := requireMapValue(t, features, key)
		if _, exists := section["enabled"]; !exists {
			t.Fatalf("feature %q must have an explicit enabled switch", key)
		}
		if _, exists := section["output"]; !exists {
			t.Fatalf("feature %q must define output", key)
		}
	}
	docker := requireMapValue(t, rest, "docker")
	if docker["enabled"] != true || docker["output"] == "" {
		t.Fatal("Docker contract must define enabled and output")
	}
	compose := requireMapValue(t, docker, "compose")
	if _, exists := compose["enabled"]; !exists {
		t.Fatal("docker.compose must define an enabled switch")
	}
	if compose["output"] == "" {
		t.Fatal("docker.compose must define output")
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
		if model["timestamps"] == true {
			fields["created_at"] = true
			fields["updated_at"] = true
		}
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

func assertMongoExamplesShowSupportedMethods(t *testing.T, mongo map[string]interface{}) {
	t.Helper()
	seen := map[string]bool{}
	for _, rawMethod := range requireSliceValue(t, mongo, "methods") {
		method, ok := rawMethod.(map[string]interface{})
		if !ok {
			t.Fatalf("Mongo method is not an object: %#v", rawMethod)
		}
		operation, _ := method["operation"].(string)
		seen[operation] = true
	}
	for _, operation := range []string{"find_one", "find_many", "update_one", "delete_one", "aggregate"} {
		if !seen[operation] {
			t.Fatalf("rest_mongo/rest_user_example.yaml must show %s method", operation)
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
	sqlConfig := "db_connection:\n  usere_password: legacy-secret\nsqlc:\n  sqlc_path: rest_sqlc.yaml\n"
	if err := os.WriteFile(filepath.Join(dir, "rest_sqlc.yaml"), []byte(sqlConfig), 0o644); err != nil {
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

func TestValidateYAMLTreeRejectsSyntaxErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rest.yaml"), []byte("http:\n  port: [8080\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateYAMLTree(dir)
	if err == nil || !strings.Contains(err.Error(), "invalid YAML") {
		t.Fatalf("expected invalid YAML error, got %v", err)
	}
}

func TestValidateYAMLTreeRejectsDuplicateKeys(t *testing.T) {
	dir := t.TempDir()
	modelsDir := filepath.Join(dir, "rest_mongo")
	if err := os.Mkdir(modelsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelsDir, "item.yaml"), []byte("models:\n  - name: Item\n    name: Duplicate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateYAMLTree(dir)
	if err == nil || !strings.Contains(err.Error(), "duplicate key") {
		t.Fatalf("expected duplicate key error, got %v", err)
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
	if bundle.SQL.SQLC.Enabled.Bool() {
		t.Fatalf("unexpected canonical SQLC switches: %+v", bundle.SQL.SQLC)
	}
	if !bundle.Rest.HTTP.Middleware.Recovery.Enabled.Bool() || !bundle.Rest.HTTP.Middleware.CORS.Enabled.Bool() || !bundle.Rest.HTTP.Middleware.SecurityHeaders.Enabled.Bool() || !bundle.Rest.HTTP.Middleware.RateLimit.Enabled.Bool() {
		t.Fatal("HTTP middleware switches were not loaded")
	}
	if bundle.Rest.HTTP.Middleware.CORS.AllowCredentials {
		t.Fatal("canonical CORS must not allow credentials by default")
	}
	if len(bundle.Rest.HTTP.Middleware.CORS.AllowOrigins) == 0 || bundle.Rest.HTTP.Middleware.CORS.AllowOrigins[0] == "*" {
		t.Fatalf("canonical CORS must use explicit origins, got %v", bundle.Rest.HTTP.Middleware.CORS.AllowOrigins)
	}
	if bundle.Rest.HTTP.Middleware.SecurityHeaders.ContentTypeOptions != "nosniff" {
		t.Fatalf("security headers were not loaded: %+v", bundle.Rest.HTTP.Middleware.SecurityHeaders)
	}
	if bundle.Rest.HTTP.Middleware.RateLimit.RequestsPerWindow < 1 || bundle.Rest.HTTP.Middleware.RateLimit.Window == "" {
		t.Fatalf("rate limit was not loaded: %+v", bundle.Rest.HTTP.Middleware.RateLimit)
	}
	if !bundle.Rest.Logging.Enabled.Bool() || !bundle.Rest.OpenAPI.Enabled.Bool() {
		t.Fatal("logging and OpenAPI must be enabled by the canonical config")
	}
	if !bundle.Rest.Testing.HandlerTests.Bool() || !bundle.Rest.Testing.Curl.Bool() {
		t.Fatal("testing switches were not loaded")
	}
	if bundle.Rest.Testing.IntegrationTests.Bool() {
		t.Fatal("integration tests must remain disabled until the feature is implemented")
	}
	if bundle.Rest.Features.Makefile.Enabled.Bool() {
		t.Fatal("makefile must remain disabled in the canonical config")
	}
}
