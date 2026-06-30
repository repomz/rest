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
