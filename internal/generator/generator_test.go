package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAutoEndpointsSkipStandardStudyQueries(t *testing.T) {
	tables := []table{{
		Name:      "studies",
		Singular:  "study",
		GoName:    "Study",
		GoPlural:  "Studies",
		RouteBase: "/studies",
	}}
	queries := map[string]queryMeta{
		"CreateStudy": {
			Name:       "CreateStudy",
			ArgKind:    "struct",
			ArgType:    "CreateStudyParams",
			ReturnType: "(Study, error)",
		},
		"GetStudies": {
			Name:       "GetStudies",
			ArgKind:    "none",
			ReturnType: "([]Study, error)",
		},
		"GetStudyByID": {
			Name:       "GetStudyByID",
			ArgName:    "id",
			ArgKind:    "single",
			ArgType:    "uuid.UUID",
			ReturnType: "(Study, error)",
		},
		"SoftDeleteStudy": {
			Name:       "SoftDeleteStudy",
			ArgName:    "id",
			ArgKind:    "single",
			ArgType:    "uuid.UUID",
			ReturnType: "error",
		},
		"SoftDeleteAllStudies": {
			Name:       "SoftDeleteAllStudies",
			ArgKind:    "none",
			ReturnType: "error",
		},
		"GetStudyByPatient": {
			Name:       "GetStudyByPatient",
			ArgName:    "patient",
			ArgKind:    "single",
			ArgType:    "string",
			ReturnType: "(Study, error)",
		},
	}

	got := autoEndpoints(tables, queries, nil, nil)

	if len(got) != 1 {
		t.Fatalf("expected only the custom query endpoint, got %d: %+v", len(got), got)
	}
	if got[0].Name != "GetStudyByPatient" {
		t.Fatalf("unexpected generated endpoint: %+v", got[0])
	}
}

func TestAutoEndpointsKeepExecCreateQuery(t *testing.T) {
	tables := []table{{
		Name:      "agent_records",
		Singular:  "agent_record",
		GoName:    "AgentRecord",
		GoPlural:  "AgentRecords",
		RouteBase: "/agent_records",
	}}
	queries := map[string]queryMeta{
		"CreateAgentRecord": {
			Name:       "CreateAgentRecord",
			ArgKind:    "struct",
			ArgType:    "CreateAgentRecordParams",
			ReturnType: "error",
		},
	}
	params := map[string][]endpointParam{
		"CreateAgentRecordParams": {
			{Name: "agent_id", Type: "int32", Required: true},
			{Name: "status", Type: "string", Required: true},
		},
	}

	got := autoEndpoints(tables, queries, params, nil)

	if len(got) != 1 || got[0].Name != "CreateAgentRecord" || !got[0].IsExec {
		t.Fatalf("expected CreateAgentRecord exec endpoint, got %+v", got)
	}
}

func TestAutoEndpointsUseQueryParamsForSQLCNarg(t *testing.T) {
	tables := []table{{
		Name:      "studies",
		Singular:  "study",
		GoName:    "Study",
		GoPlural:  "Studies",
		RouteBase: "/studies",
	}}
	queries := map[string]queryMeta{
		"GetStudies": {
			Name:       "GetStudies",
			ArgKind:    "struct",
			ArgType:    "GetStudiesParams",
			ReturnType: "([]Study, error)",
		},
	}
	params := map[string][]endpointParam{
		"GetStudiesParams": {
			{Name: "date", Type: "null_time"},
			{Name: "type", Type: "null_string"},
			{Name: "surgeon", Type: "null_string"},
		},
	}
	optional := map[string]map[string]bool{
		"GetStudies": {
			"date":    true,
			"type":    true,
			"surgeon": true,
		},
	}

	got := autoEndpoints(tables, queries, params, optional)

	if len(got) != 1 || got[0].Path != "/studies" {
		t.Fatalf("unexpected filter endpoint: %+v", got)
	}
	for _, param := range got[0].Params {
		if param.Required || param.Source != "query" {
			t.Fatalf("expected optional query parameter, got %+v", param)
		}
	}
}

func TestReadSQLCOptionalQueryParams(t *testing.T) {
	dir := t.TempDir()
	queries := `-- name: GetStudiesByDate :many
SELECT * FROM studies WHERE time_beginning::date = $1;

-- name: GetStudiesByFilter :many
SELECT * FROM studies
WHERE (sqlc.narg('surgeon')::text IS NULL OR surgeon = sqlc.narg('surgeon'))
  AND (sqlc.narg("study_type")::text IS NULL OR study_type = sqlc.narg("study_type"));
`
	if err := os.WriteFile(filepath.Join(dir, "studies.sql"), []byte(queries), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readSQLCOptionalQueryParams([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(got["GetStudiesByDate"]) != 0 {
		t.Fatalf("regular sqlc arg must remain required: %+v", got)
	}
	if !got["GetStudiesByFilter"]["surgeon"] || !got["GetStudiesByFilter"]["study_type"] {
		t.Fatalf("sqlc.narg parameters were not detected: %+v", got)
	}
}

func TestReadSQLCConfigResolvesPathsFromConfigDirectory(t *testing.T) {
	projectDir := t.TempDir()
	configDir := filepath.Join(projectDir, "sqlc")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `version: "2"
sql:
  - engine: postgresql
    queries: queries
    schema: schema
    gen:
      go:
        package: db
        out: ../internal/app/db
`
	configPath := filepath.Join(configDir, "sqlc.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := readSQLCConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.QueriesDirs) != 1 || cfg.QueriesDirs[0] != filepath.Join(configDir, "queries") {
		t.Fatalf("unexpected queries paths: %v", cfg.QueriesDirs)
	}
	if len(cfg.SchemaDirs) != 1 || cfg.SchemaDirs[0] != filepath.Join(configDir, "schema") {
		t.Fatalf("unexpected schema paths: %v", cfg.SchemaDirs)
	}
	if cfg.DBOut != filepath.Join(projectDir, "internal", "app", "db") {
		t.Fatalf("unexpected DB output path: %s", cfg.DBOut)
	}
}
