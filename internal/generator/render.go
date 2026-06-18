package generator

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"
)

func renderFile(path, tmpl string, data renderData) error {
	var buf bytes.Buffer
	funcs := template.FuncMap{
		"lower": strings.ToLower,
		"hasImport": func(cols []column, name string) bool {
			for _, col := range cols {
				switch name {
				case "sql":
					if col.NeedsSQL {
						return true
					}
				case "time":
					if col.NeedsTime {
						return true
					}
				case "uuid":
					if col.NeedsUUID {
						return true
					}
				}
			}
			return false
		},
		"hasRequired": func(cols []column) bool {
			for _, col := range cols {
				if col.Required {
					return true
				}
			}
			return false
		},
		"tableNeedsFmt": func(tbl table) bool {
			for _, col := range tbl.CreateCols {
				if col.Required {
					return true
				}
			}
			for _, ep := range tbl.Endpoints {
				for _, param := range ep.Params {
					if param.Required {
						return true
					}
				}
			}
			return false
		},
		"endpointNeedsBody": func(ep endpoint) bool {
			return len(ep.BodyParams) > 0
		},
		"tableEndpointNeedsTime": func(tbl table) bool {
			for _, ep := range tbl.Endpoints {
				for _, param := range ep.Params {
					if param.NeedsTime {
						return true
					}
				}
			}
			return false
		},
		"tableHasStandardMethods": func(tbl table) bool {
			return tbl.Queries.Create || tbl.Queries.GetAll || tbl.Queries.GetByID || tbl.Queries.Delete || tbl.Queries.DeleteAll
		},
		"tableHasServiceMethods": func(tbl table) bool {
			return tbl.Queries.Create || tbl.Queries.GetAll || tbl.Queries.GetByID || tbl.Queries.Delete || tbl.Queries.DeleteAll || len(tbl.Endpoints) > 0
		},
		"tableHasHandlers": func(tbl table) bool {
			return tbl.Queries.Create || tbl.Queries.GetAll || tbl.Queries.GetByID || tbl.Queries.Delete || tbl.Queries.DeleteAll || len(tbl.Endpoints) > 0
		},
		"tableHandlersNeedJSON": func(tbl table) bool {
			if tbl.Queries.Create {
				return true
			}
			for _, ep := range tbl.Endpoints {
				if len(ep.BodyParams) > 0 {
					return true
				}
			}
			return false
		},
		"tableHandlersNeedErrors": func(tbl table) bool {
			if tbl.Queries.GetByID {
				return true
			}
			for _, ep := range tbl.Endpoints {
				if ep.Result == "one" && ep.DomainResponse {
					return true
				}
			}
			return false
		},
		"tableHandlersNeedUUID": func(tbl table) bool {
			if tbl.Queries.GetByID || tbl.Queries.Delete {
				return true
			}
			for _, ep := range tbl.Endpoints {
				for _, param := range ep.Params {
					if param.NeedsUUID {
						return true
					}
				}
			}
			return false
		},
		"tableHandlersNeedMux": func(tbl table) bool {
			if tbl.Queries.GetByID || tbl.Queries.Delete {
				return true
			}
			for _, ep := range tbl.Endpoints {
				for _, param := range ep.Params {
					if param.Source == "path" {
						return true
					}
				}
			}
			return false
		},
		"tableEndpointNeedsStrconv": func(tbl table) bool {
			for _, ep := range tbl.Endpoints {
				if ep.NeedsStrconv {
					return true
				}
			}
			return false
		},
		"tableServiceNeedsTime": func(tbl table) bool {
			for _, ep := range tbl.Endpoints {
				if strings.Contains(ep.ResponseType, "time.Time") {
					return true
				}
			}
			return false
		},
		"tableTestNeedsTime": func(tbl table) bool {
			for _, col := range tbl.Columns {
				if col.NeedsTime {
					return true
				}
			}
			for _, ep := range tbl.Endpoints {
				if strings.Contains(ep.ResponseType, "time.Time") {
					return true
				}
			}
			return false
		},
		"tableServiceNeedsUUID": func(tbl table) bool {
			if tbl.Queries.GetByID || tbl.Queries.Delete {
				return true
			}
			for _, ep := range tbl.Endpoints {
				if strings.Contains(ep.ResponseType, "uuid.UUID") {
					return true
				}
			}
			return false
		},
		"tableRepoNeedsTime": func(tbl table) bool {
			for _, ep := range tbl.Endpoints {
				if strings.Contains(ep.ResponseType, "time.Time") {
					return true
				}
			}
			return false
		},
		"tableRepoNeedsUUID": func(tbl table) bool {
			if tbl.Queries.GetByID || tbl.Queries.Delete {
				return true
			}
			for _, ep := range tbl.Endpoints {
				if strings.Contains(ep.ResponseType, "uuid.UUID") {
					return true
				}
			}
			return false
		},
		"tableRepoNeedsErrors": func(tbl table) bool {
			if tbl.Queries.GetByID {
				return true
			}
			for _, ep := range tbl.Endpoints {
				if ep.Result == "one" && ep.DomainResponse {
					return true
				}
			}
			return false
		},
		"tableRepoNeedsSQL": func(tbl table) bool {
			if tbl.Queries.GetByID {
				return true
			}
			for _, col := range tbl.CreateCols {
				if tbl.Queries.Create && col.NeedsSQL {
					return true
				}
			}
			for _, ep := range tbl.Endpoints {
				if ep.Result == "one" && ep.DomainResponse {
					return true
				}
				for _, param := range ep.Params {
					if strings.Contains(param.DBExpr, "sql.") {
						return true
					}
				}
			}
			return false
		},
		"anyServiceNeedsTime": func(tables []table) bool {
			for _, tbl := range tables {
				for _, ep := range tbl.Endpoints {
					if strings.Contains(ep.ResponseType, "time.Time") {
						return true
					}
				}
			}
			return false
		},
		"anyServiceNeedsUUID": func(tables []table) bool {
			for _, tbl := range tables {
				if tbl.Queries.GetByID || tbl.Queries.Delete {
					return true
				}
				for _, ep := range tbl.Endpoints {
					if strings.Contains(ep.ResponseType, "uuid.UUID") {
						return true
					}
				}
			}
			return false
		},
		"anyServiceNeedsImports": func(tables []table) bool {
			for _, tbl := range tables {
				if tbl.Queries.Create || tbl.Queries.GetAll || tbl.Queries.GetByID || tbl.Queries.Delete || tbl.Queries.DeleteAll || len(tbl.Endpoints) > 0 {
					return true
				}
			}
			return false
		},
		"anyGeneratedEndpointTests": func(tables []table) bool {
			for _, tbl := range tables {
				if tbl.Queries.Create || tbl.Queries.GetAll || tbl.Queries.GetByID || tbl.Queries.Delete || tbl.Queries.DeleteAll || len(tbl.Endpoints) > 0 {
					return true
				}
			}
			return false
		},
		"tablesNeedUUID": func(tables []table) bool {
			for _, tbl := range tables {
				for _, col := range tbl.Columns {
					if col.NeedsUUID {
						return true
					}
				}
				for _, ep := range tbl.Endpoints {
					if ep.NeedsUUID {
						return true
					}
				}
			}
			return false
		},
		"tablesNeedTime": func(tables []table) bool {
			for _, tbl := range tables {
				for _, col := range tbl.Columns {
					if col.NeedsTime {
						return true
					}
				}
				for _, ep := range tbl.Endpoints {
					if ep.NeedsTime {
						return true
					}
				}
			}
			return false
		},
		"endpointReturn":           endpointReturn,
		"toDomainValue":            toDomainValue,
		"handlerBodyParamReads":    handlerBodyParamReads,
		"handlerNonBodyParamReads": handlerNonBodyParamReads,
		"handlerParamRead":         handlerParamRead,
		"repoQueryArg":             repoQueryArg,
		"sampleDomain":             sampleDomain,
		"createJSONBody":           createJSONBody,
		"endpointJSONBody":         endpointJSONBody,
		"testURL":                  testURL,
		"httpPort": func(port int) int {
			if port == 0 {
				return 8080
			}
			return port
		},
		"httpAddr": func(host string, port int) string {
			if port == 0 {
				port = 8080
			}
			if host == "" {
				return fmt.Sprintf(":%d", port)
			}
			return fmt.Sprintf("%s:%d", host, port)
		},
		"defaultString": func(value, fallback string) string {
			if value == "" {
				return fallback
			}
			return value
		},
		"intDefault": func(value, fallback int) int {
			if value == 0 {
				return fallback
			}
			return value
		},
		"join": strings.Join,
		"contains": func(values []string, value string) bool {
			for _, item := range values {
				if item == value {
					return true
				}
			}
			return false
		},
		"durationSeconds": func(value string) int64 {
			duration, err := time.ParseDuration(value)
			if err != nil {
				return 0
			}
			return int64(duration.Seconds())
		},
		"routePath": applicationPath,
	}
	if err := template.Must(template.New(filepath.Base(path)).Funcs(funcs).Parse(tmpl)).Execute(&buf, data); err != nil {
		return err
	}
	out := buf.Bytes()
	if strings.HasSuffix(path, ".go") {
		formatted, err := format.Source(out)
		if err != nil {
			return fmt.Errorf("format %s: %w\n%s", path, err, out)
		}
		out = formatted
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if strings.HasSuffix(path, ".sh") {
		mode = 0o755
	} else if filepath.Base(path) == ".env" {
		mode = 0o600
	}
	return os.WriteFile(path, out, mode)
}

func tidyGeneratedGo(src []byte) []byte {
	text := string(src)
	assignmentGap := regexp.MustCompile(`(?m)^(\s*params\.[A-Za-z0-9_]+ = .+)\n[ \t]*\n(\s*params\.[A-Za-z0-9_]+ = .+)$`)
	for assignmentGap.MatchString(text) {
		text = assignmentGap.ReplaceAllString(text, "$1\n$2")
	}
	return []byte(text)
}

func normalizeGoImports(src []byte) []byte {
	text := string(src)
	re := regexp.MustCompile(`(?s)import \(
(.*?)
\)`)
	return []byte(re.ReplaceAllStringFunc(text, func(block string) string {
		match := re.FindStringSubmatch(block)
		if len(match) != 2 {
			return block
		}
		groups := make([][]string, 3)
		seen := map[string]bool{}
		for _, raw := range strings.Split(match[1], "\n") {
			line := strings.TrimSpace(raw)
			if line == "" || seen[line] {
				continue
			}
			seen[line] = true
			importPath := importPathFromLine(line)
			group := 0
			if strings.Contains(importPath, ".") {
				group = 1
			}
			if strings.HasPrefix(importPath, "github.com/repomz/rest_generator/") {
				group = 2
			}
			groups[group] = append(groups[group], line)
		}
		var lines []string
		for _, group := range groups {
			if len(group) == 0 {
				continue
			}
			sort.Strings(group)
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			for _, line := range group {
				lines = append(lines, "\t"+line)
			}
		}
		return "import (\n" + strings.Join(lines, "\n") + "\n)"
	}))
}

func importPathFromLine(line string) string {
	start := strings.Index(line, "\"")
	end := strings.LastIndex(line, "\"")
	if start < 0 || end <= start {
		return line
	}
	return line[start+1 : end]
}

func endpointReturn(ep endpoint) string {
	if ep.IsExec {
		return "error"
	}
	return "(" + ep.ResponseType + ", error)"
}

func toDomainValue(col column) string {
	if !col.Nullable {
		return "item." + col.GoName
	}
	switch col.GoType {
	case "string":
		return "item." + col.GoName + ".String"
	case "int32":
		return "item." + col.GoName + ".Int32"
	case "time.Time":
		return "item." + col.GoName + ".Time"
	default:
		return "item." + col.GoName
	}
}

func handlerBodyParamReads(ep endpoint) string {
	lines := make([]string, 0, len(ep.BodyParams))
	for _, param := range ep.BodyParams {
		lines = append(lines, fmt.Sprintf("params.%s = body.%s", param.GoName, param.GoName))
	}
	return strings.Join(lines, "\n")
}

func handlerNonBodyParamReads(ep endpoint) string {
	blocks := make([]string, 0, len(ep.NonBodyParams))
	for _, param := range ep.NonBodyParams {
		blocks = append(blocks, handlerParamRead(param))
	}
	return strings.Join(blocks, "\n")
}

func handlerParamRead(param endpointParam) string {
	var b strings.Builder
	rawName := "raw" + param.GoName
	switch param.Source {
	case "path":
		fmt.Fprintf(&b, "%s := mux.Vars(r)[%q]\n", rawName, param.JSONName)
	case "query":
		fmt.Fprintf(&b, "%s := r.URL.Query().Get(%q)\n", rawName, param.JSONName)
	case "body":
		fmt.Fprintf(&b, "params.%s = body.%s\n", param.GoName, param.GoName)
		return b.String()
	default:
		fmt.Fprintf(&b, "%s := r.URL.Query().Get(%q)\n", rawName, param.JSONName)
	}
	if param.Required {
		fmt.Fprintf(&b, "if %s == \"\" {\nserver.BadRequest(\"missing-%s\", domain.ErrRequired, w, r)\nreturn\n}\n", rawName, param.JSONName)
	} else {
		fmt.Fprintf(&b, "if %s != \"\" {\n", rawName)
	}
	switch param.Type {
	case "uuid":
		fmt.Fprintf(&b, "value, err := uuid.Parse(%s)\nif err != nil {\nserver.BadRequest(\"invalid-%s\", err, w, r)\nreturn\n}\nparams.%s = value\n", rawName, param.JSONName, param.GoName)
	case "int", "int32", "null_int", "null_int32":
		fmt.Fprintf(&b, "value, err := strconv.Atoi(%s)\nif err != nil {\nserver.BadRequest(\"invalid-%s\", err, w, r)\nreturn\n}\nparams.%s = int32(value)\n", rawName, param.JSONName, param.GoName)
	case "time", "null_time":
		fmt.Fprintf(&b, "value, err := time.Parse(\"2006-01-02\", %s)\nif err != nil {\nserver.BadRequest(\"invalid-%s\", err, w, r)\nreturn\n}\nparams.%s = value\n", rawName, param.JSONName, param.GoName)
	default:
		fmt.Fprintf(&b, "params.%s = %s\n", param.GoName, rawName)
	}
	if !param.Required {
		b.WriteString("}\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func repoQueryArg(ep endpoint) string {
	switch ep.QueryArgKind {
	case "none":
		return ""
	case "single":
		if len(ep.Params) == 0 {
			return ""
		}
		return ", " + ep.Params[0].DBExpr
	case "struct":
		var b strings.Builder
		fmt.Fprintf(&b, ", db.%s{\n", ep.QueryArgType)
		for _, param := range ep.Params {
			fmt.Fprintf(&b, "%s: %s,\n", param.GoName, param.DBExpr)
		}
		b.WriteString("}")
		return b.String()
	default:
		return ""
	}
}

func sampleDomain(tbl table) string {
	var b strings.Builder
	fmt.Fprintf(&b, "domain.%s{\n", tbl.GoName)
	for _, col := range tbl.Columns {
		fmt.Fprintf(&b, "%s: %s,\n", col.GoName, sampleGoValue(col.GoType, col.JSONName))
	}
	b.WriteString("}")
	return b.String()
}

func createJSONBody(cols []column) string {
	parts := make([]string, 0, len(cols))
	for _, col := range cols {
		parts = append(parts, fmt.Sprintf("%q:%s", col.JSONName, sampleJSONValue(col.GoType, col.JSONName)))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func endpointJSONBody(ep endpoint) string {
	if len(ep.BodyParams) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ep.BodyParams))
	for _, param := range ep.BodyParams {
		parts = append(parts, fmt.Sprintf("%q:%s", param.JSONName, sampleJSONValue(param.GoType, param.JSONName)))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func testURL(ep endpoint) string {
	path := ep.Path
	query := make([]string, 0)
	for _, param := range ep.NonBodyParams {
		value := sampleRawValue(param.Type, param.JSONName)
		if param.Source == "path" {
			path = strings.ReplaceAll(path, "{"+param.JSONName+"}", value)
			continue
		}
		query = append(query, param.JSONName+"="+value)
	}
	if len(query) > 0 {
		path += "?" + strings.Join(query, "&")
	}
	return path
}

func sampleGoValue(goType, name string) string {
	switch goType {
	case "uuid.UUID":
		return "uuid.MustParse(\"00000000-0000-0000-0000-000000000001\")"
	case "time.Time":
		return "time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)"
	case "int32":
		return "1"
	case "bool":
		return "false"
	default:
		return fmt.Sprintf("%q", "test_"+name)
	}
}

func sampleJSONValue(goType, name string) string {
	switch goType {
	case "uuid.UUID":
		return `"00000000-0000-0000-0000-000000000001"`
	case "time.Time":
		return `"2026-01-02T00:00:00Z"`
	case "int32":
		return "1"
	case "bool":
		return "false"
	default:
		return fmt.Sprintf("%q", "test_"+name)
	}
}

func sampleRawValue(paramType, name string) string {
	switch paramType {
	case "uuid":
		return "00000000-0000-0000-0000-000000000001"
	case "time", "null_time":
		return "2026-01-02"
	case "int", "int32", "null_int", "null_int32":
		return "1"
	default:
		return "test_" + name
	}
}
