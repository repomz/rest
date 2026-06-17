package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBuildOpenAPISpecDescribesGeneratedApplication(t *testing.T) {
	tables := []table{{
		Name: "studies", Singular: "study", GoName: "Study", GoPlural: "Studies", RouteBase: "/studies",
		Columns: []column{
			{JSONName: "id", GoType: "uuid.UUID"},
			{JSONName: "patient", GoType: "string"},
			{JSONName: "created_at", GoType: "time.Time"},
		},
		CreateCols: []column{
			{JSONName: "patient", GoType: "string", Required: true},
			{JSONName: "surgeon", GoType: "string"},
		},
		Queries: querySet{GetAll: true, Create: true, GetByID: true, Delete: true},
		Endpoints: []endpoint{{
			Name: "SearchStudies", Method: "GET", Path: "/studies/search", Result: "many", DomainResponse: true,
			Params: []endpointParam{
				{Name: "surgeon", JSONName: "surgeon", Source: "query", GoType: "string"},
				{Name: "date", JSONName: "date", Source: "query", Type: "null_time", GoType: "time.Time"},
			},
		}, {
			Name: "UpdateStudyLink", Method: "PATCH", Path: "/studies/{id}/link", Result: "one", DomainResponse: true,
			Params: []endpointParam{
				{Name: "id", JSONName: "id", Source: "path", GoType: "uuid.UUID", Required: true},
				{Name: "link", JSONName: "link", Source: "body", GoType: "string", Required: true},
			},
			BodyParams: []endpointParam{{Name: "link", JSONName: "link", Source: "body", GoType: "string", Required: true}},
		}},
	}}
	features := FeatureOptions{Build: BuildFeatures{HTTPPort: 9090}, OpenAPI: OpenAPIFeatures{Enabled: true, WithUI: true}}

	spec := buildOpenAPISpec("example.test/app", tables, features)
	if strings.Count(spec, "  /studies:\n") != 1 {
		t.Fatalf("collection path must be emitted once:\n%s", spec)
	}
	var document map[string]interface{}
	if err := yaml.Unmarshal([]byte(spec), &document); err != nil {
		t.Fatalf("generated OpenAPI is not valid YAML: %v", err)
	}

	paths := mapValue(t, document, "paths")
	for _, path := range []string{"/", "/swagger/openapi.yaml", "/swagger/index.html", "/studies", "/studies/{id}", "/studies/search", "/studies/{id}/link"} {
		if _, ok := paths[path]; !ok {
			t.Fatalf("missing generated path %s", path)
		}
	}

	post := mapValue(t, mapValue(t, paths, "/studies"), "post")
	requestBody := mapValue(t, post, "requestBody")
	jsonBody := mapValue(t, mapValue(t, requestBody, "content"), "application/json")
	if got := mapValue(t, jsonBody, "schema")["$ref"]; got != "#/components/schemas/StudyRequest" {
		t.Fatalf("unexpected create request schema: %v", got)
	}
	responses := mapValue(t, post, "responses")
	for _, status := range []string{"200", "400", "500"} {
		if _, ok := responses[status]; !ok {
			t.Fatalf("create operation misses response %s", status)
		}
	}

	getByID := mapValue(t, mapValue(t, paths, "/studies/{id}"), "get")
	getResponses := mapValue(t, getByID, "responses")
	for _, status := range []string{"200", "400", "404", "500"} {
		if _, ok := getResponses[status]; !ok {
			t.Fatalf("get by ID misses response %s", status)
		}
	}

	schemas := mapValue(t, mapValue(t, document, "components"), "schemas")
	for _, schema := range []string{"ErrorResponse", "SuccessResponse", "DeletedResponse", "StudyRequest", "StudyResponse", "UpdateStudyLinkRequest"} {
		if _, ok := schemas[schema]; !ok {
			t.Fatalf("missing component schema %s", schema)
		}
	}
	studyRequest := mapValue(t, schemas, "StudyRequest")
	required := sliceValue(t, studyRequest, "required")
	if len(required) != 1 || required[0] != "patient" {
		t.Fatalf("unexpected required create fields: %v", required)
	}
	studyProperties := mapValue(t, mapValue(t, schemas, "StudyResponse"), "properties")
	if mapValue(t, studyProperties, "id")["format"] != "uuid" {
		t.Fatal("UUID response field must use uuid format")
	}
	if mapValue(t, studyProperties, "created_at")["format"] != "date-time" {
		t.Fatal("time response field must use date-time format")
	}

	servers := sliceMapsValue(t, document, "servers")
	if servers[0]["url"] != "http://localhost:9090" {
		t.Fatalf("unexpected server URL: %v", servers[0]["url"])
	}
	search := mapValue(t, mapValue(t, paths, "/studies/search"), "get")
	parameters := sliceMapsValue(t, search, "parameters")
	if mapValue(t, parameters[1], "schema")["format"] != "date" {
		t.Fatal("HTTP date query parameter must use date format")
	}
	assertOpenAPIRefsResolve(t, document, schemas)
}

func TestSwaggerRoutesAreServedByGeneratedHandlers(t *testing.T) {
	data := renderData{
		Features: FeatureOptions{
			HTTP:    HTTPFeatures{BasePath: "/"},
			OpenAPI: OpenAPIFeatures{Enabled: true, WithUI: true, SpecPath: "/swagger/openapi.yaml", UIPath: "/swagger/index.html"},
		},
		OpenAPI: "openapi: 3.0.3\n",
	}
	appMain, err := renderTemplateForTest(t, "main.go", appMainTemplate, data)
	if err != nil {
		t.Fatal(err)
	}
	swagger, err := renderTemplateForTest(t, "swagger.go", swaggerTemplate, data)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`HandleFunc("/swagger/openapi.yaml", httpserver.SwaggerSpec)`, `HandleFunc("/swagger/index.html", httpserver.SwaggerUI)`} {
		if !strings.Contains(string(appMain), expected) {
			t.Fatalf("main route %q not generated:\n%s", expected, appMain)
		}
	}
	for _, expected := range []string{"func SwaggerSpec", "func SwaggerUI", "SwaggerUIBundle"} {
		if !strings.Contains(string(swagger), expected) {
			t.Fatalf("swagger handler %q not generated:\n%s", expected, swagger)
		}
	}
}

func renderTemplateForTest(t *testing.T, name, tmpl string, data renderData) ([]byte, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := renderFile(path, tmpl, data); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func assertOpenAPIRefsResolve(t *testing.T, value interface{}, schemas map[string]interface{}) {
	t.Helper()
	switch value := value.(type) {
	case map[string]interface{}:
		for key, child := range value {
			if key == "$ref" {
				ref, ok := child.(string)
				if !ok || !strings.HasPrefix(ref, "#/components/schemas/") {
					t.Fatalf("unsupported OpenAPI reference: %#v", child)
				}
				name := strings.TrimPrefix(ref, "#/components/schemas/")
				if _, ok := schemas[name]; !ok {
					t.Fatalf("OpenAPI reference points to missing schema %s", name)
				}
				continue
			}
			assertOpenAPIRefsResolve(t, child, schemas)
		}
	case []interface{}:
		for _, child := range value {
			assertOpenAPIRefsResolve(t, child, schemas)
		}
	}
}

func mapValue(t *testing.T, parent map[string]interface{}, key string) map[string]interface{} {
	t.Helper()
	value, ok := parent[key].(map[string]interface{})
	if !ok {
		t.Fatalf("%s is not an object: %#v", key, parent[key])
	}
	return value
}

func sliceValue(t *testing.T, parent map[string]interface{}, key string) []interface{} {
	t.Helper()
	value, ok := parent[key].([]interface{})
	if !ok {
		t.Fatalf("%s is not an array: %#v", key, parent[key])
	}
	return value
}

func sliceMapsValue(t *testing.T, parent map[string]interface{}, key string) []map[string]interface{} {
	t.Helper()
	values := sliceValue(t, parent, key)
	result := make([]map[string]interface{}, 0, len(values))
	for _, value := range values {
		item, ok := value.(map[string]interface{})
		if !ok {
			t.Fatalf("%s contains a non-object value: %#v", key, value)
		}
		result = append(result, item)
	}
	return result
}
