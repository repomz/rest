package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCurlTemplateContainsEveryInstanceEndpoint(t *testing.T) {
	tbl := table{
		Name:       "studies",
		Singular:   "study",
		RouteBase:  "/studies",
		CreateCols: []column{{JSONName: "patient", GoType: "string"}},
		Queries: querySet{
			Create:    true,
			GetByID:   true,
			Delete:    true,
			DeleteAll: true,
		},
		Endpoints: []endpoint{{
			Name:   "GetStudies",
			Method: "GET",
			Path:   "/studies",
			NonBodyParams: []endpointParam{{
				Name: "surgeon", JSONName: "surgeon", Source: "query", Type: "string",
			}},
		}, {
			Name:   "UpdateStudyLink",
			Method: "PATCH",
			Path:   "/studies/{id}/link",
			BodyParams: []endpointParam{{
				JSONName: "link", GoType: "string", Source: "body",
			}},
			NonBodyParams: []endpointParam{{
				Name: "id", JSONName: "id", Source: "path", Type: "uuid",
			}},
		}},
	}
	path := filepath.Join(t.TempDir(), "curl", "study.md")
	data := renderData{Table: tbl, Features: FeatureOptions{Build: BuildFeatures{HTTPPort: 9090}}}
	if err := renderFile(path, curlTemplate, data); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, want := range []string{
		`BASE_URL="${BASE_URL:-http://localhost:9090}"`,
		`-X POST`,
		`/studies?surgeon=test_surgeon`,
		`-X PATCH`,
		`/studies/00000000-0000-0000-0000-000000000001/link`,
		`-d '{"link":"test_link"}'`,
		`-X DELETE`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated curl file does not contain %q:\n%s", want, text)
		}
	}
}
