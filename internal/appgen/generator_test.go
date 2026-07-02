package appgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/repomz/rest/internal/config"
)

func TestValidateConfigRequiresGracefulShutdown(t *testing.T) {
	bundle := minimalBundle()
	bundle.Rest.HTTP.GracefulShutdown.Enabled = config.Enabled(false)

	err := validateConfig(bundle)
	if err == nil || !strings.Contains(err.Error(), "http.graceful_shutdown.enabled") {
		t.Fatalf("expected graceful shutdown validation error, got %v", err)
	}
}

func TestResolveSQLCPathUsesConfigDir(t *testing.T) {
	got := resolveSQLCPath("/project/rest_config", "../rest_sqlc/rest_sqlc.yaml")
	want := "/project/rest_sqlc/rest_sqlc.yaml"
	if got != want {
		t.Fatalf("sqlc path = %q, want %q", got, want)
	}
}

func TestDiscoverMongoAuthEndpointsUsesActiveEntityFiles(t *testing.T) {
	configDir := t.TempDir()
	modelsDir := filepath.Join(configDir, "rest_mongo")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelsDir, "rest_user_example.yaml"), []byte(`
version: "0.1.0"
models:
  - name: User
    collection: users
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelsDir, "item.yaml"), []byte(`
version: "0.1.0"
models:
  - name: Item
    collection: items
  - name: Dimensions
    embedded: true
methods:
  - model: Item
    name: FindFeaturedItems
    operation: find_many
    http:
      method: GET
      path: /items/featured
  - model: Item
    name: DeleteExpiredItems
    operation: delete_one
    http:
      path: /items/expired
`), 0o644); err != nil {
		t.Fatal(err)
	}
	bundle := minimalBundle()
	bundle.Dir = configDir
	bundle.Rest.Mongo = config.Enabled(true)
	bundle.Mongo = &config.Mongo{
		Mongo: config.MongoSettings{ModelsPath: "rest_mongo"},
	}
	endpoints, err := discoverMongoAuthEndpoints(NewContext(bundle))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, endpoint := range endpoints {
		got[endpoint.Method+" "+endpoint.Path+" "+endpoint.Name] = true
	}
	for _, want := range []string{
		"GET /items GetAllItems",
		"POST /items CreateItem",
		"GET /items/{id} GetItemByID",
		"PATCH /items/{id} UpdateItem",
		"DELETE /items/{id} DeleteItem",
		"GET /items/featured FindFeaturedItems",
		"DELETE /items/expired DeleteExpiredItems",
	} {
		if !got[want] {
			t.Fatalf("missing Mongo auth endpoint %q in %#v", want, endpoints)
		}
	}
	for key := range got {
		if strings.Contains(key, "users") {
			t.Fatalf("system rest_*.yaml file must not produce auth endpoints, got %s", key)
		}
	}
}

func TestGenerationFingerprintIncludesMongoContracts(t *testing.T) {
	configDir := t.TempDir()
	projectDir := t.TempDir()
	modelsDir := filepath.Join(configDir, "rest_mongo")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "rest.yaml"), []byte("mongo: enable\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "mongo_rest.yaml"), []byte("mongo:\n  models_path: rest_mongo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	contractPath := filepath.Join(modelsDir, "item.yaml")
	if err := os.WriteFile(contractPath, []byte("models: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bundle := minimalBundle()
	bundle.Dir = configDir
	bundle.Rest.ProjectPath = projectDir
	bundle.Rest.Mongo = config.Enabled(true)
	bundle.Mongo = &config.Mongo{
		Mongo: config.MongoSettings{ModelsPath: "rest_mongo"},
	}
	first, err := generationFingerprint(NewContext(bundle))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(contractPath, []byte("models:\n  - name: Item\n    collection: items\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := generationFingerprint(NewContext(bundle))
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("Mongo contract changes must change generation fingerprint")
	}
}

func TestValidateReferencedYAMLInputsRejectsDuplicateSQLCKeys(t *testing.T) {
	configDir := t.TempDir()
	sqlcDir := filepath.Join(t.TempDir(), "rest_sqlc")
	if err := os.MkdirAll(sqlcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sqlcDir, "rest_sqlc.yaml"), []byte("version: \"2\"\nversion: duplicate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bundle := minimalBundle()
	bundle.Dir = configDir
	bundle.Rest.SQL = config.Enabled(true)
	bundle.SQL = &config.SQL{
		SQLC: config.SQLC{
			Path: filepath.Join(sqlcDir, "rest_sqlc.yaml"),
		},
	}
	err := validateReferencedYAMLInputs(NewContext(bundle))
	if err == nil || !strings.Contains(err.Error(), "duplicate key") {
		t.Fatalf("expected duplicate SQLC YAML key error, got %v", err)
	}
}

func TestGenerateMongoOnlyOpenAPI(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "rest_config")
	projectDir := filepath.Join(root, "app")
	if err := config.Generate(configDir); err != nil {
		t.Fatal(err)
	}
	patchFile(t, filepath.Join(configDir, "rest.yaml"), map[string]string{
		"module: github.com/repomz/myapp": "module: example.test/mongoapi",
		"project_path: .":                 "project_path: " + projectDir,
		"sql: enable":                     "sql: disable",
		"mongo: disable":                  "mongo: enable",
	})
	writeFile(t, filepath.Join(configDir, "rest_mongo", "item.yaml"), `
version: "0.1.0"
models:
  - name: Item
    collection: items
    fields:
      - name: id
        type: object_id
        json: id
        primary: true
        generated: true
      - name: title
        type: string
        required: true
methods:
  - model: Item
    name: FindByTitle
    operation: find_one
    http:
      method: GET
      path: /items/by-title
    parameters:
      - name: title
        type: string
        source: query
        required: true
`)
	if err := New(DefaultRegistry()...).Generate(configDir); err != nil {
		t.Fatal(err)
	}
	swagger, err := os.ReadFile(filepath.Join(projectDir, "docs", "swagger.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(swagger)
	for _, expected := range []string{"/items:", "/items/{id}:", "/items/by-title:", "ItemRequest:", "ItemResponse:"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("Mongo OpenAPI output missing %q:\n%s", expected, text)
		}
	}
}

func TestListEndpointsShowsMongoSourcesAndPendingAuth(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "rest_config")
	if err := config.GenerateForMongoExample(configDir); err != nil {
		t.Fatal(err)
	}
	patchFile(t, filepath.Join(configDir, "rest.yaml"), map[string]string{
		"auth: disable": "auth: enable",
	})
	endpoints, err := ListEndpoints(configDir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]EndpointInfo{}
	for _, endpoint := range endpoints {
		got[endpoint.Method+" "+endpoint.Path] = endpoint
	}
	if got["GET /health"].Access != "public" || got["GET /health"].Source != "system" {
		t.Fatalf("health endpoint = %+v", got["GET /health"])
	}
	if got["GET /ready"].Access != "public" || got["GET /ready"].Source != "system" {
		t.Fatalf("readiness endpoint = %+v", got["GET /ready"])
	}
	create := got["POST /items"]
	if create.Name != "CreateItem" || create.Source != "mongo" || create.Access != "pending" {
		t.Fatalf("create endpoint = %+v", create)
	}
	if got["GET /items/by-status"].Name != "FindItemsByStatus" || got["GET /items/by-status"].Source != "mongo" {
		t.Fatalf("custom endpoint = %+v", got["GET /items/by-status"])
	}
}

func TestApplyEndpointAccessUsesConfiguredRoles(t *testing.T) {
	bundle := minimalBundle()
	bundle.Rest.Auth = config.Enabled(true)
	bundle.Auth = &config.Auth{
		Authorization: config.AuthAuthorization{DefaultPolicy: "deny"},
		Endpoints: []config.AuthEndpoint{{
			Name: "CreateItem", Method: "POST", Path: "/items",
			RequireAuth: true, Roles: []string{"writer", "admin"},
		}},
	}
	info := EndpointInfo{Name: "CreateItem", Method: "POST", Path: "/items"}
	applyEndpointAccess(&info, bundle, false)
	if info.Access != "auth" || strings.Join(info.Roles, ",") != "admin,writer" {
		t.Fatalf("endpoint access = %+v", info)
	}
}

func minimalBundle() config.Bundle {
	return config.Bundle{
		Rest: config.Rest{
			Language: "go",
			HTTP: config.HTTP{
				Framework:        "std",
				Port:             8080,
				BasePath:         "/",
				GracefulShutdown: config.GeneratedSwitch{Enabled: config.Enabled(true)},
				Health:           config.Health{Path: "/health"},
				Middleware: config.Middleware{
					CORS: config.CORS{MaxAge: "12h"},
				},
			},
			Logging: config.Logging{
				Enabled: config.Enabled(false),
				Output:  config.LoggingOutput{Type: "stdout"},
			},
			OpenAPI: config.OpenAPI{
				SpecPath: "/swagger/openapi.yaml",
				UIPath:   "/swagger/index.html",
			},
			Observability: config.Observability{
				Metrics: config.Metrics{Path: "/metrics"},
			},
		},
	}
}

func patchFile(t *testing.T, path string, replacements map[string]string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimPrefix(content, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}
