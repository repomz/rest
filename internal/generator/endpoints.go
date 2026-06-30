package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func readQuerierMethods(path string) (map[string]bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`\n\s*([A-Z]\w+)\(`)
	methods := map[string]bool{}
	for _, match := range re.FindAllStringSubmatch(string(b), -1) {
		methods[match[1]] = true
	}
	return methods, nil
}

type queryMeta struct {
	Name       string
	ArgName    string
	ArgKind    string
	ArgType    string
	ReturnType string
}

func readQuerierMeta(path string) (map[string]queryMeta, error) {
	info, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		dir := filepath.Dir(path)
		if filepath.Base(path) != "querier.go" {
			return nil, err
		}
		return readQuerierMeta(dir)
	}
	var files []string
	if info.IsDir() {
		matches, err := filepath.Glob(filepath.Join(path, "*.go"))
		if err != nil {
			return nil, err
		}
		files = matches
		sort.Strings(files)
	} else {
		files = []string{path}
	}
	re := regexp.MustCompile(`(?m)^\s*(?:func\s+\([^)]+\)\s+)?([A-Z]\w+)\(ctx context\.Context(?:,\s*(\w+)\s+([^\)]+))?\)\s+(.+?)\s*\{?\s*$`)
	result := map[string]queryMeta{}
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		for _, match := range re.FindAllStringSubmatch(string(b), -1) {
			meta := queryMeta{Name: match[1], ReturnType: strings.TrimSpace(strings.TrimSuffix(match[4], "{"))}
			meta.ArgName = match[2]
			if match[3] == "" {
				meta.ArgKind = "none"
			} else if strings.HasPrefix(match[3], "db.") || strings.HasSuffix(match[3], "Params") {
				meta.ArgKind = "struct"
				meta.ArgType = strings.TrimPrefix(match[3], "db.")
			} else {
				meta.ArgKind = "single"
				meta.ArgType = match[3]
			}
			result[meta.Name] = meta
		}
	}
	return result, nil
}

func finalizeEndpoint(ep endpoint, queries map[string]queryMeta) endpoint {
	ep.Name = exported(ep.Name)
	if meta, ok := queries[ep.Query]; ok {
		ep.QueryArgKind = meta.ArgKind
		ep.QueryArgType = meta.ArgType
		ep.ReturnType = meta.ReturnType
	}
	for i := range ep.Params {
		param := &ep.Params[i]
		param.GoName = exported(param.Name)
		param.JSONName = param.Name
		param.GoType, param.DBExpr, param.NeedsTime, param.NeedsUUID, param.NeedsInt, param.ValidCheck = mapEndpointParam(param)
		if param.Source == "body" {
			ep.BodyParams = append(ep.BodyParams, *param)
		} else {
			ep.NonBodyParams = append(ep.NonBodyParams, *param)
		}
		ep.NeedsTime = ep.NeedsTime || param.NeedsTime
		ep.NeedsUUID = ep.NeedsUUID || param.NeedsUUID
		ep.NeedsStrconv = ep.NeedsStrconv || param.NeedsInt
	}
	return ep
}

func mapEndpointParam(param *endpointParam) (goType, dbExpr string, needsTime, needsUUID, needsInt bool, validCheck string) {
	name := "params." + param.GoName
	switch param.Type {
	case "uuid":
		return "uuid.UUID", name, false, true, false, name + " != uuid.Nil"
	case "int", "int32":
		return "int32", name, false, false, true, name + " != 0"
	case "time":
		return "time.Time", name, true, false, false, "!" + name + ".IsZero()"
	case "null_time":
		return "time.Time", fmt.Sprintf("sql.NullTime{Time: %s, Valid: !%s.IsZero()}", name, name), true, false, false, "!" + name + ".IsZero()"
	case "null_int", "null_int32":
		return "int32", fmt.Sprintf("sql.NullInt32{Int32: %s, Valid: %s != 0}", name, name), false, false, true, name + " != 0"
	case "null_string":
		return "string", fmt.Sprintf("sql.NullString{String: %s, Valid: %s != \"\"}", name, name), false, false, false, name + ` != ""`
	default:
		return "string", name, false, false, false, name + ` != ""`
	}
}

func readSQLCParamStructs(dbOutDir string) (map[string][]endpointParam, error) {
	files, err := filepath.Glob(filepath.Join(dbOutDir, "*.sql.go"))
	if err != nil {
		return nil, err
	}
	result := map[string][]endpointParam{}
	structRe := regexp.MustCompile(`(?s)type\s+(\w+Params)\s+struct\s*\{(.*?)\}`)
	fieldRe := regexp.MustCompile(`(?m)^\s*(\w+)\s+([^` + "`" + `\n]+)(?:` + "`" + `json:"([^\"]+)"` + "`" + `)?`)
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		for _, sm := range structRe.FindAllStringSubmatch(string(b), -1) {
			for _, fm := range fieldRe.FindAllStringSubmatch(sm[2], -1) {
				jsonName := fm[3]
				if jsonName == "" {
					jsonName = lowerSnake(fm[1])
				}
				result[sm[1]] = append(result[sm[1]], endpointParam{
					Name:     jsonName,
					Type:     endpointTypeFromGo(strings.TrimSpace(fm[2])),
					Required: true,
				})
			}
		}
	}
	return result, nil
}

func readSQLCOptionalQueryParams(queriesDirs []string) (map[string]map[string]bool, error) {
	var files []string
	for _, queriesDir := range queriesDirs {
		matches, err := filepath.Glob(filepath.Join(queriesDir, "*.sql"))
		if err != nil {
			return nil, err
		}
		files = append(files, matches...)
	}
	result := map[string]map[string]bool{}
	queryRe := regexp.MustCompile(`(?m)^--\s*name:\s*(\w+)\s+:\w+\s*$`)
	nargRe := regexp.MustCompile(`sqlc\.narg\(['"](\w+)['"]\)`)
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		text := string(b)
		matches := queryRe.FindAllStringSubmatchIndex(text, -1)
		for i, match := range matches {
			end := len(text)
			if i+1 < len(matches) {
				end = matches[i+1][0]
			}
			queryName := text[match[2]:match[3]]
			for _, narg := range nargRe.FindAllStringSubmatch(text[match[1]:end], -1) {
				if result[queryName] == nil {
					result[queryName] = map[string]bool{}
				}
				result[queryName][narg[1]] = true
			}
		}
	}
	return result, nil
}

func autoEndpoints(tables []table, queries map[string]queryMeta, paramStructs map[string][]endpointParam, optionalQueryParams map[string]map[string]bool) []endpoint {
	var endpoints []endpoint
	names := make([]string, 0, len(queries))
	for name := range queries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		meta := queries[name]
		for _, tbl := range tables {
			if !queryBelongsToTable(name, tbl) {
				continue
			}
			if isStandardQuery(name, queries, tbl) {
				continue
			}
			ep := endpoint{
				TableName: tbl.Name,
				Name:      name,
				Method:    defaultHTTPMethod(name, meta.ReturnType),
				Path:      autoEndpointPath(tbl, name),
				Query:     name,
				Result:    "one",
			}
			source := "query"
			if ep.Method == "POST" || ep.Method == "PUT" || ep.Method == "PATCH" {
				source = "body"
			}
			switch meta.ArgKind {
			case "single":
				paramName := lowerSnake(meta.ArgName)
				if paramName == "" {
					paramName = "value"
				}
				ep.Params = []endpointParam{{Name: paramName, Source: source, Type: endpointTypeFromGo(meta.ArgType), Required: !optionalQueryParams[name][paramName]}}
			case "struct":
				for _, param := range paramStructs[meta.ArgType] {
					param.Required = !optionalQueryParams[name][param.Name]
					param.Source = source
					ep.Params = append(ep.Params, param)
				}
			}
			ep = applyAutoEndpointPathAndSources(ep, tbl)
			ep = finalizeEndpoint(ep, queries)
			ep = applyEndpointReturn(ep, tbl)
			endpoints = append(endpoints, ep)
		}
	}
	return endpoints
}

func endpointBelongsToTable(ep endpoint, tbl table) bool {
	return ep.TableName == tbl.Name || ep.TableName == tbl.Singular || ep.TableName == tbl.GoName
}

func isStandardQuery(name string, queries map[string]queryMeta, tbl table) bool {
	standard := detectQueries(queries, tbl)
	switch name {
	case "Create" + tbl.GoName:
		return standard.Create
	case "Get" + tbl.GoPlural:
		return standard.GetAll
	case "Get" + tbl.GoName + "ByID":
		return standard.GetByID
	case "SoftDelete" + tbl.GoName:
		return standard.Delete
	case "SoftDeleteAll" + tbl.GoPlural:
		return standard.DeleteAll
	default:
		return false
	}
}

func queryBelongsToTable(queryName string, tbl table) bool {
	return strings.Contains(queryName, tbl.GoName) || strings.Contains(queryName, tbl.GoPlural)
}

func autoEndpointPath(tbl table, queryName string) string {
	lowerName := strings.ToLower(queryName)
	if strings.HasPrefix(lowerName, "create") {
		return tbl.RouteBase
	}
	return tbl.RouteBase + "/" + kebab(queryName)
}

func applyAutoEndpointPathAndSources(ep endpoint, tbl table) endpoint {
	if ep.Method == "GET" || ep.Method == "DELETE" {
		if hasOptionalEndpointParams(ep.Params) {
			for i := range ep.Params {
				ep.Params[i].Source = "query"
			}
			ep.Path = tbl.RouteBase
			return ep
		}
		for i := range ep.Params {
			ep.Params[i].Source = "path"
		}
		if len(ep.Params) == 0 {
			ep.Path = tbl.RouteBase
		} else {
			ep.Path = tbl.RouteBase + pathSegmentsForEndpointParams(ep.Params)
		}
		return ep
	}

	if ep.Method == "PATCH" || ep.Method == "PUT" {
		for i := range ep.Params {
			if ep.Params[i].Name == "id" && ep.Params[i].Type == "uuid" {
				ep.Params[i].Source = "path"
			}
		}
		if hasPathEndpointParam(ep.Params) {
			ep.Path = tbl.RouteBase + "/{id}/" + kebab(strings.TrimPrefix(ep.Query, "Update"+tbl.GoName))
		}
	}
	return ep
}

func hasOptionalEndpointParams(params []endpointParam) bool {
	for _, param := range params {
		if !param.Required {
			return true
		}
	}
	return false
}

func hasPathEndpointParam(params []endpointParam) bool {
	for _, param := range params {
		if param.Source == "path" {
			return true
		}
	}
	return false
}

func pathSegmentsForEndpointParams(params []endpointParam) string {
	var b strings.Builder
	for _, param := range params {
		label := strings.TrimSuffix(param.Name, "_id")
		label = strings.ReplaceAll(label, "_", "-")
		b.WriteString("/")
		b.WriteString(label)
		b.WriteString("/{")
		b.WriteString(param.Name)
		b.WriteString("}")
	}
	return b.String()
}

func defaultHTTPMethod(queryName, returnType string) string {
	lowerName := strings.ToLower(queryName)
	switch {
	case strings.HasPrefix(lowerName, "get"), strings.HasPrefix(lowerName, "list"), strings.HasPrefix(lowerName, "find"), strings.HasPrefix(lowerName, "search"):
		return "GET"
	case strings.HasPrefix(lowerName, "delete"), strings.HasPrefix(lowerName, "remove"), strings.HasPrefix(lowerName, "softdelete"):
		return "DELETE"
	case strings.HasPrefix(lowerName, "update"), strings.HasPrefix(lowerName, "patch"):
		return "PATCH"
	default:
		return "POST"
	}
}

func applyEndpointReturn(ep endpoint, tbl table) endpoint {
	valueType, ok := queryValueReturnType(ep.ReturnType)
	if !ok {
		ep.Result = "exec"
		ep.IsExec = true
		ep.ResponseType = "error"
		ep.ZeroValue = ""
		ep.SampleReturn = "nil"
		return ep
	}
	ep.Result = "one"
	if strings.HasPrefix(valueType, "[]") {
		ep.Result = "many"
	}
	scalarType := strings.TrimPrefix(valueType, "[]")
	ep.NeedsTime = ep.NeedsTime || scalarType == "time.Time"
	ep.NeedsUUID = ep.NeedsUUID || scalarType == "uuid.UUID"
	if scalarType == tbl.GoName {
		ep.DomainResponse = true
		if ep.Result == "many" {
			ep.ResponseType = "[]domain." + tbl.GoName
			ep.ZeroValue = "nil"
			ep.SampleReturn = "[]domain." + tbl.GoName + "{sample" + tbl.GoName + "()}, nil"
		} else {
			ep.ResponseType = "domain." + tbl.GoName
			ep.ZeroValue = "domain." + tbl.GoName + "{}"
			ep.SampleReturn = "sample" + tbl.GoName + "(), nil"
		}
		return ep
	}
	ep.ResponseType = valueType
	if ep.Result == "many" {
		ep.ZeroValue = "nil"
		ep.SampleReturn = sampleEndpointReturn(valueType) + ", nil"
	} else {
		ep.ZeroValue = zeroValue(valueType)
		ep.SampleReturn = sampleEndpointReturn(valueType) + ", nil"
	}
	return ep
}

func queryValueReturnType(returnType string) (string, bool) {
	returnType = strings.TrimSpace(returnType)
	if returnType == "error" {
		return "", false
	}
	returnType = strings.TrimPrefix(strings.TrimSuffix(returnType, ")"), "(")
	parts := strings.Split(returnType, ",")
	if len(parts) < 2 || strings.TrimSpace(parts[len(parts)-1]) != "error" {
		return "", false
	}
	return strings.TrimSpace(strings.Join(parts[:len(parts)-1], ",")), true
}

func endpointTypeFromGo(goType string) string {
	goType = strings.TrimSpace(strings.TrimPrefix(goType, "db."))
	switch goType {
	case "uuid.UUID":
		return "uuid"
	case "time.Time":
		return "time"
	case "int", "int32", "int64":
		return "int32"
	case "sql.NullString":
		return "null_string"
	case "sql.NullInt32", "sql.NullInt64":
		return "null_int32"
	case "sql.NullTime":
		return "null_time"
	default:
		return "string"
	}
}

func sampleEndpointReturn(goType string) string {
	if strings.HasPrefix(goType, "[]") {
		inner := strings.TrimPrefix(goType, "[]")
		return "[]" + inner + "{" + sampleGoValue(inner, "value") + "}"
	}
	return sampleGoValue(goType, "value")
}

func zeroValue(goType string) string {
	switch goType {
	case "string":
		return "\"\""
	case "int", "int32", "int64":
		return "0"
	case "bool":
		return "false"
	case "time.Time":
		return "time.Time{}"
	case "uuid.UUID":
		return "uuid.Nil"
	default:
		return goType + "{}"
	}
}

func attachEndpoints(tables []table, endpoints []endpoint) {
	for _, ep := range endpoints {
		for i := range tables {
			if endpointBelongsToTable(ep, tables[i]) {
				tables[i].Endpoints = append(tables[i].Endpoints, ep)
			}
		}
	}
}

func detectQueries(queries map[string]queryMeta, tbl table) querySet {
	create := false
	if meta, ok := queries["Create"+tbl.GoName]; ok {
		create = strings.Contains(meta.ReturnType, tbl.GoName) && strings.Contains(meta.ReturnType, "error")
	}
	return querySet{
		Create:    create,
		GetAll:    returnsManyDomainRows(queries["Get"+tbl.GoPlural], tbl) && queries["Get"+tbl.GoPlural].ArgKind == "none",
		GetByID:   returnsOneDomainRow(queries["Get"+tbl.GoName+"ByID"], tbl),
		Delete:    returnsOnlyError(queries["SoftDelete"+tbl.GoName]),
		DeleteAll: returnsOnlyError(queries["SoftDeleteAll"+tbl.GoPlural]),
	}
}

func returnsManyDomainRows(meta queryMeta, tbl table) bool {
	return strings.Contains(meta.ReturnType, "[]"+tbl.GoName) && strings.Contains(meta.ReturnType, "error")
}

func returnsOneDomainRow(meta queryMeta, tbl table) bool {
	return strings.Contains(meta.ReturnType, "("+tbl.GoName+", error)")
}

func returnsOnlyError(meta queryMeta) bool {
	return meta.ReturnType == "error"
}
