package generator

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"unicode"
)

type Options struct {
	SQLCPath    string
	HTTPGenPath string
	OutDir      string
}

type sqlcConfig struct {
	QueriesDir string
	SchemaDir  string
	DBPackage  string
	DBOut      string
}

type table struct {
	Name       string
	Singular   string
	Plural     string
	GoName     string
	GoPlural   string
	RouteBase  string
	Columns    []column
	CreateCols []column
	Endpoints  []endpoint
	Queries    querySet
}

type column struct {
	Name       string
	GoName     string
	JSONName   string
	GoType     string
	DBValue    string
	Nullable   bool
	Required   bool
	ReadOnly   bool
	NeedsSQL   bool
	NeedsTime  bool
	NeedsUUID  bool
	ValidCheck string
}

type querySet struct {
	Create    bool
	GetAll    bool
	GetByID   bool
	Delete    bool
	DeleteAll bool
}

type endpoint struct {
	TableName      string
	Name           string
	Method         string
	Path           string
	Query          string
	Result         string
	Params         []endpointParam
	BodyParams     []endpointParam
	NonBodyParams  []endpointParam
	NeedsTime      bool
	NeedsStrconv   bool
	NeedsUUID      bool
	QueryArgType   string
	QueryArgKind   string
	ReturnType     string
	ResponseType   string
	ZeroValue      string
	SampleReturn   string
	DomainResponse bool
	IsExec         bool
}

type endpointParam struct {
	Name       string
	GoName     string
	JSONName   string
	Source     string
	Type       string
	GoType     string
	Required   bool
	NeedsTime  bool
	NeedsUUID  bool
	NeedsInt   bool
	ValidCheck string
	DBExpr     string
}

type renderData struct {
	Module    string
	DBPackage string
	DBImport  string
	Tables    []table
	Table     table
	Queries   querySet
}

func Generate(opts Options) error {
	if opts.SQLCPath == "" {
		opts.SQLCPath = "sqlc.yaml"
	}
	if opts.OutDir == "" {
		opts.OutDir = "."
	}
	if opts.HTTPGenPath == "" {
		opts.HTTPGenPath = filepath.Join(opts.OutDir, "httpgen.yaml")
	}

	cfg, err := readSQLCConfig(opts.SQLCPath)
	if err != nil {
		return err
	}

	module, err := readModule(filepath.Join(opts.OutDir, "go.mod"))
	if err != nil {
		return err
	}

	tables, err := readSchemaTables(filepath.Join(opts.OutDir, cfg.SchemaDir))
	if err != nil {
		return err
	}
	if len(tables) == 0 {
		return fmt.Errorf("no CREATE TABLE statements found in %s", cfg.SchemaDir)
	}

	queryMeta, err := readQuerierMeta(filepath.Join(opts.OutDir, cfg.DBOut, "querier.go"))
	if err != nil {
		return err
	}
	paramStructs, err := readSQLCParamStructs(filepath.Join(opts.OutDir, cfg.DBOut))
	if err != nil {
		return err
	}
	httpGenPath := opts.HTTPGenPath
	if !filepath.IsAbs(httpGenPath) {
		httpGenPath = filepath.Join(opts.OutDir, httpGenPath)
	}
	configuredEndpoints, err := readHTTPGenConfig(httpGenPath, queryMeta)
	if err != nil {
		return err
	}
	endpoints := append(autoEndpoints(tables, queryMeta, paramStructs, configuredEndpoints), configuredEndpoints...)
	attachEndpoints(tables, endpoints)
	for i := range tables {
		tables[i].Queries = detectQueries(queryMeta, tables[i])
	}
	if err := cleanGeneratedAppLayers(opts.OutDir); err != nil {
		return err
	}

	data := renderData{
		Module:    module,
		DBPackage: cfg.DBPackage,
		DBImport:  module + "/" + filepath.ToSlash(cfg.DBOut),
		Tables:    tables,
	}

	files := map[string]string{
		"internal/app/common/server/http_error.go":        commonServerErrorTemplate,
		"internal/app/common/server/http_ok.go":           commonServerOKTemplate,
		"internal/app/common/slugerrors/errors.go":        slugErrorsTemplate,
		"internal/app/config/config.go":                   configTemplate,
		"internal/app/domain/error.go":                    domainErrorTemplate,
		"internal/app/transport/httpserver/sever.go":      httpServerTemplate,
		"internal/app/transport/httpserver/interfaces.go": httpServerInterfacesTemplate,
		"cmd/main.go": appMainTemplate,
		"Makefile":    makefileTemplate,
		"init_db.sh":  initDBTemplate,
	}

	for path, tmpl := range files {
		if err := renderFile(filepath.Join(opts.OutDir, path), tmpl, data); err != nil {
			return err
		}
	}

	for _, tbl := range tables {
		tableData := data
		tableData.Table = tbl
		tableData.Queries = tbl.Queries
		tableFiles := map[string]string{
			fmt.Sprintf("internal/app/domain/%s.go", tbl.Singular):                             domainModelTemplate,
			fmt.Sprintf("internal/app/services/%s.go", tbl.Singular):                           serviceTemplate,
			fmt.Sprintf("internal/app/repository/pgrepo/%s_repo.go", tbl.Singular):             repoTemplate,
			fmt.Sprintf("internal/app/transport/httpmodels/%s.go", tbl.Singular):               httpModelsTemplate,
			fmt.Sprintf("internal/app/transport/httpserver/%s_handlers.go", tbl.Singular):      httpHandlersTemplate,
			fmt.Sprintf("internal/app/transport/httpserver/%s_handlers_test.go", tbl.Singular): httpHandlersTestTemplate,
		}
		for path, tmpl := range tableFiles {
			if err := renderFile(filepath.Join(opts.OutDir, path), tmpl, tableData); err != nil {
				return err
			}
		}
	}

	return nil
}

func cleanGeneratedAppLayers(outDir string) error {
	paths := []string{
		"internal/app/common",
		"internal/app/config",
		"internal/app/domain",
		"internal/app/repository",
		"internal/app/services",
		"internal/app/transport",
	}
	for _, path := range paths {
		if err := os.RemoveAll(filepath.Join(outDir, path)); err != nil {
			return err
		}
	}
	return nil
}

func readSQLCConfig(path string) (sqlcConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return sqlcConfig{}, err
	}
	cfg := sqlcConfig{DBPackage: "db"}
	lines := strings.Split(string(b), "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		switch {
		case strings.HasPrefix(line, "queries:"):
			cfg.QueriesDir = yamlValue(line)
		case strings.HasPrefix(line, "schema:"):
			cfg.SchemaDir = yamlValue(line)
		case strings.HasPrefix(line, "package:"):
			cfg.DBPackage = yamlValue(line)
		case strings.HasPrefix(line, "out:"):
			cfg.DBOut = yamlValue(line)
		}
	}
	if cfg.SchemaDir == "" || cfg.QueriesDir == "" || cfg.DBOut == "" {
		return sqlcConfig{}, fmt.Errorf("sqlc config must define queries, schema and gen.go.out")
	}
	return cfg, nil
}

func yamlValue(line string) string {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(parts[1]), `"'`)
}

func readModule(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	return "", fmt.Errorf("module declaration not found in %s", path)
}

func readSchemaTables(schemaDir string) ([]table, error) {
	files, err := filepath.Glob(filepath.Join(schemaDir, "*.sql"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	var sql strings.Builder
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		sql.WriteByte('\n')
		sql.Write(b)
	}

	re := regexp.MustCompile(`(?is)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?("?[\w]+"?)\s*\((.*?)\);`)
	matches := re.FindAllStringSubmatch(sql.String(), -1)
	tables := make([]table, 0, len(matches))
	for _, match := range matches {
		name := cleanIdent(match[1])
		tbl := table{
			Name:      name,
			Singular:  singular(name),
			Plural:    name,
			RouteBase: "/" + name,
		}
		tbl.GoName = exported(tbl.Singular)
		tbl.GoPlural = exported(tbl.Plural)
		tbl.Columns = parseColumns(match[2])
		for _, col := range tbl.Columns {
			if !col.ReadOnly {
				tbl.CreateCols = append(tbl.CreateCols, col)
			}
		}
		tables = append(tables, tbl)
	}
	return tables, nil
}

func parseColumns(body string) []column {
	var cols []column
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, ","))
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := cleanIdent(fields[0])
		upperName := strings.ToUpper(name)
		if upperName == "PRIMARY" || upperName == "CONSTRAINT" || upperName == "FOREIGN" || upperName == "UNIQUE" || upperName == "CHECK" {
			continue
		}
		sqlType := strings.ToUpper(strings.Trim(fields[1], ","))
		nullable := !strings.Contains(strings.ToUpper(line), "NOT NULL") && !strings.Contains(strings.ToUpper(line), "PRIMARY KEY")
		col := column{
			Name:     name,
			GoName:   exported(name),
			JSONName: name,
			Nullable: nullable,
			ReadOnly: isReadOnlyColumn(name) || strings.Contains(strings.ToUpper(line), "DEFAULT"),
		}
		col.Required = !col.Nullable && !col.ReadOnly && !strings.Contains(sqlType, "BOOL")
		col.GoType, col.DBValue, col.NeedsSQL, col.NeedsTime, col.NeedsUUID, col.ValidCheck = mapSQLType(sqlType, col)
		cols = append(cols, col)
	}
	return cols
}

func mapSQLType(sqlType string, col column) (goType, dbValue string, needsSQL, needsTime, needsUUID bool, validCheck string) {
	switch {
	case strings.Contains(sqlType, "UUID"):
		goType, dbValue, needsUUID = "uuid.UUID", col.GoName, true
		validCheck = "item." + col.GoName + " != uuid.Nil"
	case strings.Contains(sqlType, "INT"):
		goType = "int32"
		validCheck = "item." + col.GoName + " != 0"
		if col.Nullable {
			dbValue, needsSQL = fmt.Sprintf("sql.NullInt32{Int32: item.%s, Valid: item.%s != 0}", col.GoName, col.GoName), true
		} else {
			dbValue = col.GoName
		}
	case strings.Contains(sqlType, "TIME") || strings.Contains(sqlType, "DATE"):
		goType, needsTime = "time.Time", true
		validCheck = "!item." + col.GoName + ".IsZero()"
		if col.Nullable {
			dbValue, needsSQL = fmt.Sprintf("sql.NullTime{Time: item.%s, Valid: !item.%s.IsZero()}", col.GoName, col.GoName), true
		} else {
			dbValue = col.GoName
		}
	case strings.Contains(sqlType, "BOOL"):
		goType, dbValue = "bool", col.GoName
		validCheck = "item." + col.GoName
	default:
		goType = "string"
		validCheck = "item." + col.GoName + ` != ""`
		if col.Nullable {
			dbValue, needsSQL = fmt.Sprintf("sql.NullString{String: item.%s, Valid: item.%s != \"\"}", col.GoName, col.GoName), true
		} else {
			dbValue = col.GoName
		}
	}
	if !col.Nullable {
		dbValue = "item." + col.GoName
	}
	return
}

func isReadOnlyColumn(name string) bool {
	switch name {
	case "id", "created_at", "updated_at", "deleted":
		return true
	default:
		return false
	}
}

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
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`(?m)^\s*([A-Z]\w+)\(ctx context\.Context(?:,\s*(\w+)\s+([^\)]+))?\)\s+(.+)$`)
	result := map[string]queryMeta{}
	for _, match := range re.FindAllStringSubmatch(string(b), -1) {
		meta := queryMeta{Name: match[1], ReturnType: strings.TrimSpace(match[4])}
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
	return result, nil
}

func readHTTPGenConfig(path string, queries map[string]queryMeta) ([]endpoint, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var endpoints []endpoint
	var current *endpoint
	var currentParam *endpointParam
	inEndpoints := false
	inParams := false

	for _, raw := range strings.Split(string(b), "\n") {
		if i := strings.Index(raw, "#"); i >= 0 {
			raw = raw[:i]
		}
		if strings.TrimSpace(raw) == "" {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		line := strings.TrimSpace(raw)
		if line == "endpoints:" {
			inEndpoints = true
			continue
		}
		if !inEndpoints {
			continue
		}
		if strings.HasPrefix(line, "- ") && indent == 2 {
			if current != nil {
				endpoints = append(endpoints, finalizeEndpoint(*current, queries))
			}
			current = &endpoint{Method: "GET", Result: "many"}
			currentParam = nil
			inParams = false
			applyEndpointKV(current, strings.TrimSpace(strings.TrimPrefix(line, "- ")))
			continue
		}
		if current == nil {
			continue
		}
		if line == "params:" {
			inParams = true
			currentParam = nil
			continue
		}
		if inParams && strings.HasPrefix(line, "- ") {
			param := endpointParam{}
			applyParamKV(&param, strings.TrimSpace(strings.TrimPrefix(line, "- ")))
			current.Params = append(current.Params, param)
			currentParam = &current.Params[len(current.Params)-1]
			continue
		}
		if inParams && currentParam != nil {
			applyParamKV(currentParam, line)
			continue
		}
		applyEndpointKV(current, line)
	}
	if current != nil {
		endpoints = append(endpoints, finalizeEndpoint(*current, queries))
	}
	return endpoints, nil
}

func applyEndpointKV(ep *endpoint, line string) {
	key, value, ok := splitYAMLKV(line)
	if !ok {
		return
	}
	switch key {
	case "table":
		ep.TableName = value
	case "name":
		ep.Name = value
	case "method":
		ep.Method = strings.ToUpper(value)
	case "path":
		ep.Path = value
	case "query":
		ep.Query = value
	case "result":
		ep.Result = value
	}
}

func applyParamKV(param *endpointParam, line string) {
	key, value, ok := splitYAMLKV(line)
	if !ok {
		return
	}
	switch key {
	case "name":
		param.Name = value
	case "source":
		param.Source = value
	case "type":
		param.Type = value
	case "required":
		param.Required = value == "true"
	}
}

func splitYAMLKV(line string) (string, string, bool) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
	return key, value, true
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

func autoEndpoints(tables []table, queries map[string]queryMeta, paramStructs map[string][]endpointParam, configured []endpoint) []endpoint {
	configuredQueries := map[string]bool{}
	for _, ep := range configured {
		configuredQueries[ep.Query] = true
	}
	var endpoints []endpoint
	names := make([]string, 0, len(queries))
	for name := range queries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if configuredQueries[name] {
			continue
		}
		meta := queries[name]
		for _, tbl := range tables {
			if !queryBelongsToTable(name, tbl) {
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
				ep.Params = []endpointParam{{Name: paramName, Source: source, Type: endpointTypeFromGo(meta.ArgType), Required: true}}
			case "struct":
				for _, param := range paramStructs[meta.ArgType] {
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
	if ep.Method != "GET" && ep.Method != "DELETE" {
		return ep
	}
	byParams := byClauseParamNames(ep.Query)
	if len(byParams) == 0 {
		ep.Path = tbl.RouteBase
		return ep
	}
	paramSet := map[string]bool{}
	for _, name := range byParams {
		paramSet[name] = true
	}
	for i := range ep.Params {
		if paramSet[ep.Params[i].Name] {
			ep.Params[i].Source = "path"
		}
	}
	ep.Path = tbl.RouteBase + pathSegmentsForParams(byParams)
	return ep
}

func byClauseParamNames(queryName string) []string {
	words := splitNameWords(queryName)
	for i, word := range words {
		if strings.EqualFold(word, "by") && i+1 < len(words) {
			return paramNamesFromWords(words[i+1:])
		}
	}
	return nil
}

func paramNamesFromWords(words []string) []string {
	var result []string
	var current []string
	flush := func() {
		if len(current) == 0 {
			return
		}
		for i := range current {
			current[i] = strings.ToLower(current[i])
		}
		result = append(result, strings.Join(current, "_"))
		current = nil
	}
	for _, word := range words {
		if strings.EqualFold(word, "and") || strings.EqualFold(word, "or") {
			flush()
			continue
		}
		current = append(current, word)
	}
	flush()
	return result
}

func pathSegmentsForParams(params []string) string {
	var b strings.Builder
	for _, param := range params {
		label := strings.TrimSuffix(param, "_id")
		label = strings.ReplaceAll(label, "_", "-")
		b.WriteString("/")
		b.WriteString(label)
		b.WriteString("/{")
		b.WriteString(param)
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
			if ep.TableName == tables[i].Name || ep.TableName == tables[i].Singular || ep.TableName == tables[i].GoName {
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
		GetAll:    returnsManyDomainRows(queries["Get"+tbl.GoPlural], tbl),
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
	if filepath.Base(path) == "init_db.sh" {
		mode = 0o755
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

func kebab(s string) string {
	return strings.ReplaceAll(lowerSnake(s), "_", "-")
}

func lowerSnake(s string) string {
	words := splitNameWords(s)
	for i := range words {
		words[i] = strings.ToLower(words[i])
	}
	return strings.Join(words, "_")
}

func splitNameWords(s string) []string {
	s = normalizeNameAcronymBoundaries(s)
	var words []string
	var current []rune
	runes := []rune(s)
	for i, r := range runes {
		if r == '_' || r == '-' || r == ' ' {
			if len(current) > 0 {
				words = append(words, string(current))
				current = nil
			}
			continue
		}
		if len(current) > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			var next rune
			if i+1 < len(runes) {
				next = runes[i+1]
			}
			if !unicode.IsUpper(prev) || (next != 0 && unicode.IsLower(next)) {
				words = append(words, string(current))
				current = nil
			}
		}
		current = append(current, r)
	}
	if len(current) > 0 {
		words = append(words, string(current))
	}
	return words
}

func normalizeNameAcronymBoundaries(s string) string {
	acronyms := []string{"UUID", "HTTP", "SQL", "URL", "API", "DB", "ID"}
	for _, acronym := range acronyms {
		s = strings.ReplaceAll(s, acronym+"and", acronym+"And")
		s = strings.ReplaceAll(s, acronym+"or", acronym+"Or")
	}
	return s
}

func cleanIdent(s string) string {
	return strings.Trim(s, "\"")
}

func singular(s string) string {
	if strings.HasSuffix(s, "ies") {
		return strings.TrimSuffix(s, "ies") + "y"
	}
	if strings.HasSuffix(s, "s") {
		return strings.TrimSuffix(s, "s")
	}
	return s
}

func exported(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		if hasUpper(part) {
			runes := []rune(part)
			runes[0] = unicode.ToUpper(runes[0])
			b.WriteString(string(runes))
			continue
		}
		lower := strings.ToLower(part)
		if initialism, ok := commonInitialisms[lower]; ok {
			b.WriteString(initialism)
			continue
		}
		runes := []rune(lower)
		runes[0] = unicode.ToUpper(runes[0])
		b.WriteString(string(runes))
	}
	return b.String()
}

func hasUpper(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

var commonInitialisms = map[string]string{
	"api":   "API",
	"db":    "DB",
	"dns":   "DNS",
	"html":  "HTML",
	"http":  "HTTP",
	"https": "HTTPS",
	"id":    "ID",
	"ip":    "IP",
	"json":  "JSON",
	"sql":   "SQL",
	"uid":   "UID",
	"url":   "URL",
	"uuid":  "UUID",
	"xml":   "XML",
}
