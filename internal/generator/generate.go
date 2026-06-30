package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Generate(opts Options) error {
	if opts.SQLCPath == "" {
		opts.SQLCPath = "rest_sqlc/rest_sqlc.yaml"
	}
	if opts.OutDir == "" {
		opts.OutDir = "."
	}
	outDir, err := filepath.Abs(opts.OutDir)
	if err != nil {
		return err
	}
	cfg, err := readSQLCConfig(opts.SQLCPath)
	if err != nil {
		return err
	}

	module, err := readModule(filepath.Join(outDir, "go.mod"))
	if err != nil {
		return err
	}

	tables, err := readSchemaTables(cfg.SchemaDirs)
	if err != nil {
		return err
	}
	if len(tables) == 0 {
		return fmt.Errorf("no CREATE TABLE statements found in %s", strings.Join(cfg.SchemaDirs, ", "))
	}

	queryMeta, err := readQuerierMeta(filepath.Join(cfg.DBOut, "querier.go"))
	if err != nil {
		return err
	}
	paramStructs, err := readSQLCParamStructs(cfg.DBOut)
	if err != nil {
		return err
	}
	optionalQueryParams, err := readSQLCOptionalQueryParams(cfg.QueriesDirs)
	if err != nil {
		return err
	}
	endpoints := autoEndpoints(tables, queryMeta, paramStructs, optionalQueryParams)
	attachEndpoints(tables, endpoints)
	for i := range tables {
		tables[i].Queries = detectQueries(queryMeta, tables[i])
		tables[i].CreateArg = detectCreateArg(tables[i], paramStructs)
	}
	completeAuthFeatures(&opts.Features.Auth, tables)
	if err := validateAuthGeneration(opts.Features.Auth, tables); err != nil {
		return err
	}
	dbOut, err := filepath.Rel(outDir, cfg.DBOut)
	if err != nil || dbOut == ".." || filepath.IsAbs(dbOut) || len(dbOut) >= 3 && dbOut[:3] == ".."+string(filepath.Separator) {
		return fmt.Errorf("sqlc gen.go.out must be inside project_path: %s", cfg.DBOut)
	}

	data := renderData{
		Module:    module,
		DBPackage: cfg.DBPackage,
		DBImport:  module + "/" + filepath.ToSlash(dbOut),
		Tables:    tables,
		Features:  opts.Features,
	}
	if opts.Features.OpenAPI.Enabled {
		data.OpenAPI = buildOpenAPISpec(module, tables, opts.Features)
	}

	files := map[string]string{
		"internal/app/common/server/http_error.go":        commonServerErrorTemplate,
		"internal/app/common/server/http_ok.go":           commonServerOKTemplate,
		"internal/app/common/slugerrors/errors.go":        slugErrorsTemplate,
		"internal/app/config/config.go":                   configTemplate,
		"internal/app/domain/error.go":                    domainErrorTemplate,
		"internal/app/transport/httpserver/server.go":     httpServerTemplate,
		"internal/app/transport/httpserver/interfaces.go": httpServerInterfacesTemplate,
		"cmd/main.go": appMainTemplate,
	}
	if !opts.Features.Build.Configured || opts.Features.Build.Makefile {
		files[defaultPath(opts.Features.Build.MakefilePath, "Makefile")] = makefileTemplate
	}
	if opts.Features.Build.InitDB {
		files[defaultPath(opts.Features.Build.InitDBPath, "init_db.sh")] = initDBTemplate
	}
	if opts.Features.Build.Env {
		files[defaultPath(opts.Features.Build.EnvPath, ".env.example")] = envExampleTemplate
		if opts.Features.Build.GenerateLocalEnv {
			files[".env"] = envExampleTemplate
		}
	}
	if opts.Features.Docker.Enabled || opts.Features.Docker.Compose {
		files[defaultPath(opts.Features.Docker.Output, "Dockerfile")] = dockerfileTemplate
		files[defaultPath(opts.Features.Docker.DockerignoreOutput, ".dockerignore")] = dockerignoreTemplate
		if opts.Features.Docker.Compose {
			files[defaultPath(opts.Features.Docker.ComposeOutput, "docker-compose.yml")] = dockerComposeTemplate
		}
	}
	if opts.Features.HTTP.CORS || opts.Features.HTTP.Recovery || opts.Features.HTTP.RequestID || opts.Features.HTTP.MaxBodyBytes > 0 {
		files["internal/app/transport/middleware/http.go"] = httpMiddlewareTemplate
	}
	if opts.Features.Auth.Enabled {
		files["internal/app/transport/httpserver/auth_middleware.go"] = authMiddlewareTemplate
		files["internal/app/transport/httpserver/password_helpers.go"] = passwordHelpersTemplate
		if strings.EqualFold(opts.Features.Auth.Strategy, "jwt") {
			files["internal/app/services/token_service.go"] = tokenServiceTemplate
		}
	}
	if opts.Features.Logging.Enabled {
		files["internal/app/logging/logger.go"] = loggingTemplate
	}
	if opts.Features.OpenAPI.Enabled {
		files["internal/app/transport/httpserver/swagger.go"] = swaggerTemplate
	}
	if opts.Features.Metrics.Enabled {
		files["internal/app/metrics/metrics.go"] = metricsTemplate
	}
	if opts.Features.Build.CI {
		files[defaultPath(opts.Features.Build.CIPath, ".github/workflows/ci.yaml")] = ciWorkflowTemplate
	}
	if opts.Features.Build.CD {
		files[defaultPath(opts.Features.Build.CDPath, ".github/workflows/cd.yaml")] = cdWorkflowTemplate
	}

	var generatedFiles []string
	for path := range files {
		generatedFiles = append(generatedFiles, path)
	}
	if opts.Features.Build.Gitignore {
		generatedFiles = append(generatedFiles, defaultPath(opts.Features.Build.GitignorePath, ".gitignore"))
	}
	migrationPath := ""
	if opts.Features.Build.Configured {
		migrationPath = filepath.Join(outDir, defaultPath(opts.Features.Build.MigrationsPath, "internal/sql/migrations"), "00001_rest_init.sql")
		if opts.Features.Build.InitMigration {
			if rel, ok, err := generatedRelPath(outDir, migrationPath); err != nil {
				return err
			} else if ok {
				generatedFiles = append(generatedFiles, rel)
			}
		}
	}
	openAPIOutput := ""
	if opts.Features.OpenAPI.Enabled {
		openAPIOutput = opts.Features.OpenAPI.Output
		if openAPIOutput == "" {
			openAPIOutput = "docs/swagger.yaml"
		}
		if !filepath.IsAbs(openAPIOutput) {
			openAPIOutput = filepath.Join(outDir, openAPIOutput)
		}
		if rel, ok, err := generatedRelPath(outDir, openAPIOutput); err != nil {
			return err
		} else if ok {
			generatedFiles = append(generatedFiles, rel)
		}
	}
	tableFilesByName := map[string]map[string]string{}
	for _, tbl := range tables {
		tableFiles := map[string]string{
			fmt.Sprintf("internal/app/domain/%s.go", tbl.Singular):                        domainModelTemplate,
			fmt.Sprintf("internal/app/services/%s.go", tbl.Singular):                      serviceTemplate,
			fmt.Sprintf("internal/app/repository/pgrepo/%s_repo.go", tbl.Singular):        repoTemplate,
			fmt.Sprintf("internal/app/transport/httpmodels/%s.go", tbl.Singular):          httpModelsTemplate,
			fmt.Sprintf("internal/app/transport/httpserver/%s_handlers.go", tbl.Singular): httpHandlersTemplate,
		}
		if !opts.Features.Build.Configured || opts.Features.Build.HandlerTests {
			tableFiles[fmt.Sprintf("internal/app/transport/httpserver/%s_handlers_test.go", tbl.Singular)] = httpHandlersTestTemplate
		}
		if opts.Features.Auth.Enabled && strings.EqualFold(opts.Features.Auth.Strategy, "jwt") && isAuthIdentityTable(tbl, opts.Features.Auth) {
			delete(tableFiles, fmt.Sprintf("internal/app/transport/httpserver/%s_handlers.go", tbl.Singular))
			delete(tableFiles, fmt.Sprintf("internal/app/transport/httpserver/%s_handlers_test.go", tbl.Singular))
			tableFiles["internal/app/transport/httpserver/auth_handlers.go"] = authHandlersTemplate
		}
		if opts.Features.Build.Curl {
			tableFiles[fmt.Sprintf("curl/%s.md", tbl.Singular)] = curlTemplate
		}
		tableFilesByName[tbl.Name] = tableFiles
		for path := range tableFiles {
			generatedFiles = append(generatedFiles, path)
		}
	}

	var preserved map[string][]byte
	reload := newSafeReload(outDir, opts.Stdin, opts.Stdout)
	if opts.Features.Build.SafeReload {
		preserved, err = reload.resolve(generatedFiles)
		if err != nil {
			return err
		}
	}

	if err := cleanGeneratedAppLayers(outDir); err != nil {
		return err
	}
	if opts.Features.Build.Configured {
		if err := os.RemoveAll(filepath.Join(outDir, "curl")); err != nil {
			return err
		}
		for _, path := range generatedOptionalPaths(opts.Features) {
			if path == "" {
				continue
			}
			target := path
			if !filepath.IsAbs(target) {
				target = filepath.Join(outDir, target)
			}
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		if err := removeGeneratedEnv(filepath.Join(outDir, ".env")); err != nil {
			return err
		}
		gitignorePath := filepath.Join(outDir, defaultPath(opts.Features.Build.GitignorePath, ".gitignore"))
		if err := removeGeneratedSection(gitignorePath, "# rest:begin", "# rest:end"); err != nil {
			return err
		}
		if err := removeGeneratedMigration(migrationPath); err != nil {
			return err
		}
	}
	if opts.Features.Build.Configured && !opts.Features.OpenAPI.Enabled {
		output := opts.Features.OpenAPI.Output
		if output == "" {
			output = "docs/swagger.yaml"
		}
		if !filepath.IsAbs(output) {
			output = filepath.Join(outDir, output)
		}
		if err := os.Remove(output); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	if opts.Features.Build.Configured && opts.Features.Build.InitMigration {
		migration, err := buildInitialMigration(cfg.SchemaDirs, tables)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(migrationPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(migrationPath, []byte(migration), 0o644); err != nil {
			return err
		}
	}

	for path, tmpl := range files {
		if err := renderFile(filepath.Join(outDir, path), tmpl, data); err != nil {
			return err
		}
	}
	if opts.Features.Build.Gitignore {
		path := defaultPath(opts.Features.Build.GitignorePath, ".gitignore")
		if err := writeGeneratedText(filepath.Join(outDir, path), gitignoreTemplate, data, opts.Features.Build.GitignoreAppend); err != nil {
			return err
		}
	}

	for _, tbl := range tables {
		tableData := data
		tableData.Table = tbl
		tableData.Queries = tbl.Queries
		tableFiles := tableFilesByName[tbl.Name]
		for path, tmpl := range tableFiles {
			if err := renderFile(filepath.Join(outDir, path), tmpl, tableData); err != nil {
				return err
			}
		}
	}
	if opts.Features.OpenAPI.Enabled {
		if err := os.MkdirAll(filepath.Dir(openAPIOutput), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(openAPIOutput, []byte(data.OpenAPI), 0o644); err != nil {
			return err
		}
	}

	if opts.Features.Build.SafeReload {
		if err := reload.save(generatedFiles); err != nil {
			return err
		}
		if err := reload.restore(preserved); err != nil {
			return err
		}
	}

	return nil
}

func generatedRelPath(root, path string) (string, bool, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", false, err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", false, nil
	}
	return filepath.ToSlash(rel), true, nil
}

func completeAuthFeatures(auth *AuthFeatures, tables []table) {
	if auth == nil || !auth.Enabled {
		return
	}
	if auth.UserTable == "" {
		auth.UserTable = "users"
	}
	if auth.UserModel == "" {
		auth.UserModel = "User"
	}
	if auth.ClaimsModel == "" {
		auth.ClaimsModel = auth.UserModel
	}
	if auth.UserIDField == "" {
		auth.UserIDField = "id"
	}
	if auth.UsernameField == "" {
		auth.UsernameField = "username"
	}
	if auth.PasswordField == "" {
		auth.PasswordField = "password"
	}
	if auth.RolesField == "" {
		auth.RolesField = "roles"
	}
	for _, tbl := range tables {
		if !isAuthIdentityTable(tbl, *auth) {
			continue
		}
		for _, col := range tbl.Columns {
			switch col.Name {
			case auth.UserIDField:
				auth.UserIDGoName = col.GoName
			case auth.UsernameField:
				auth.UsernameGoName = col.GoName
			case auth.PasswordField:
				auth.PasswordGoName = col.GoName
			case auth.RolesField:
				auth.RolesGoName = col.GoName
			}
		}
	}
	if auth.UserIDGoName == "" {
		auth.UserIDGoName = exported(auth.UserIDField)
	}
	if auth.UsernameGoName == "" {
		auth.UsernameGoName = exported(auth.UsernameField)
	}
	if auth.PasswordGoName == "" {
		auth.PasswordGoName = exported(auth.PasswordField)
	}
	if auth.RolesGoName == "" {
		auth.RolesGoName = exported(auth.RolesField)
	}
	if auth.JWTAccessTokenTTL == "" {
		auth.JWTAccessTokenTTL = "15m"
	}
	if auth.JWTRefreshStorage == "" {
		auth.JWTRefreshStorage = "context"
	}
}

func isAuthIdentityTable(tbl table, auth AuthFeatures) bool {
	return strings.EqualFold(tbl.Name, auth.UserTable) || strings.EqualFold(tbl.GoName, auth.UserModel)
}

func validateAuthGeneration(auth AuthFeatures, tables []table) error {
	if !auth.Enabled || !strings.EqualFold(auth.Strategy, "jwt") {
		return nil
	}
	for _, tbl := range tables {
		if !isAuthIdentityTable(tbl, auth) {
			continue
		}
		if !tbl.Queries.Create {
			return fmt.Errorf("jwt auth requires Create%s SQLC query for signup", tbl.GoName)
		}
		if !tbl.Queries.GetAll {
			return fmt.Errorf("jwt auth requires Get%s SQLC query for signin lookup", tbl.GoPlural)
		}
		if !tableHasColumn(tbl, auth.UsernameField) {
			return fmt.Errorf("jwt auth identity username_field %q was not found in table %s", auth.UsernameField, tbl.Name)
		}
		if !tableHasColumn(tbl, auth.PasswordField) {
			return fmt.Errorf("jwt auth identity password_field %q was not found in table %s", auth.PasswordField, tbl.Name)
		}
		return nil
	}
	return fmt.Errorf("jwt auth requires identity table %q / model %q generated by SQLC", auth.UserTable, auth.UserModel)
}

func tableHasColumn(tbl table, name string) bool {
	for _, col := range tbl.Columns {
		if strings.EqualFold(col.Name, name) || strings.EqualFold(col.GoName, name) {
			return true
		}
	}
	return false
}

func detectCreateArg(tbl table, paramStructs map[string][]endpointParam) string {
	if !tbl.Queries.Create {
		return ""
	}
	if _, ok := paramStructs["Create"+tbl.GoName+"Params"]; ok {
		return "struct"
	}
	if len(tbl.CreateCols) == 1 {
		return "single"
	}
	return "struct"
}

func defaultPath(path, fallback string) string {
	if path == "" {
		return fallback
	}
	return path
}

func generatedOptionalPaths(features FeatureOptions) []string {
	return []string{
		defaultPath(features.Build.MakefilePath, "Makefile"),
		defaultPath(features.Build.InitDBPath, "init_db.sh"),
		defaultPath(features.Build.EnvPath, ".env.example"),
		defaultPath(features.Build.CIPath, ".github/workflows/ci.yaml"),
		defaultPath(features.Build.CDPath, ".github/workflows/cd.yaml"),
		defaultPath(features.Docker.Output, "Dockerfile"),
		defaultPath(features.Docker.DockerignoreOutput, ".dockerignore"),
		defaultPath(features.Docker.ComposeOutput, "docker-compose.yml"),
	}
}

func removeGeneratedEnv(path string) error {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if hasGeneratedPrefix(string(content), "#") {
		return os.Remove(path)
	}
	return nil
}

func removeGeneratedMigration(path string) error {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if hasGeneratedPrefix(string(content), "--") {
		return os.Remove(path)
	}
	return fmt.Errorf("refusing to overwrite non-generated migration %s", path)
}

func hasGeneratedPrefix(content, comment string) bool {
	return strings.HasPrefix(content, comment+" Code generated by rest.")
}

func removeGeneratedSection(path, begin, end string) error {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	text := string(content)
	start := strings.Index(text, begin)
	if start < 0 {
		return nil
	}
	finish := strings.Index(text[start:], end)
	if finish < 0 {
		return fmt.Errorf("generated section in %s has no closing marker", path)
	}
	finish = start + finish + len(end)
	for finish < len(text) && (text[finish] == '\n' || text[finish] == '\r') {
		finish++
	}
	updated := strings.TrimRight(text[:start]+text[finish:], "\r\n")
	if updated == "" {
		return os.Remove(path)
	}
	updated += "\n"
	return os.WriteFile(path, []byte(updated), 0o644)
}

func writeGeneratedText(path, tmpl string, data renderData, appendFile bool) error {
	var existing []byte
	if appendFile {
		existing, _ = os.ReadFile(path)
	}
	temp := path + ".generated"
	if err := renderFile(temp, tmpl, data); err != nil {
		return err
	}
	content, err := os.ReadFile(temp)
	if err != nil {
		return err
	}
	_ = os.Remove(temp)
	if appendFile && len(existing) > 0 {
		if strings.Contains(string(existing), string(content)) {
			return nil
		}
		existing = append(existing, '\n')
		content = append(existing, content...)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func cleanGeneratedAppLayers(outDir string) error {
	paths := []string{
		"internal/app/common",
		"internal/app/config",
		"internal/app/domain",
		"internal/app/repository",
		"internal/app/services",
		"internal/app/transport",
		"internal/app/logging",
		"internal/app/metrics",
	}
	for _, path := range paths {
		if err := os.RemoveAll(filepath.Join(outDir, path)); err != nil {
			return err
		}
	}
	return nil
}
