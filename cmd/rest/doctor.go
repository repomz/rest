package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/repomz/rest/internal/config"
	"gopkg.in/yaml.v3"
)

type doctorLevel string

const (
	doctorOK    doctorLevel = "OK"
	doctorWarn  doctorLevel = "WARN"
	doctorError doctorLevel = "ERROR"
)

type doctorCheck struct {
	Level doctorLevel
	Name  string
	Fix   string
}

type doctorReport struct {
	Checks []doctorCheck
}

type doctorFailedError struct {
	errors int
}

func (e doctorFailedError) Error() string {
	return fmt.Sprintf("rest doctor found %d error(s)", e.errors)
}

func runDoctor(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown argument %q", args[0])
	}
	report := runDoctorChecks("rest_config")
	report.Print(os.Stdout)
	if report.Errors() > 0 {
		return doctorFailedError{errors: report.Errors()}
	}
	return nil
}

func runDoctorChecks(configDir string) doctorReport {
	d := doctorRunner{configDir: configDir}
	d.checkProject()
	return d.report
}

type doctorRunner struct {
	configDir  string
	bundle     config.Bundle
	projectDir string
	report     doctorReport
}

func (d *doctorRunner) check(level doctorLevel, name, fix string) {
	d.report.Checks = append(d.report.Checks, doctorCheck{Level: level, Name: name, Fix: fix})
}

func (d *doctorRunner) ok(name string) {
	d.check(doctorOK, name, "")
}

func (d *doctorRunner) warn(name, fix string) {
	d.check(doctorWarn, name, fix)
}

func (d *doctorRunner) err(name, fix string) {
	d.check(doctorError, name, fix)
}

func (d *doctorRunner) checkProject() {
	if _, err := os.Stat(d.configDir); err != nil {
		if os.IsNotExist(err) {
			d.err("rest_config directory is missing", "Run `rest init` first.")
			return
		}
		d.err("rest_config directory cannot be read: "+err.Error(), "Check filesystem permissions.")
		return
	}
	d.ok("rest_config directory found")

	if err := config.ValidateYAMLTree(d.configDir); err != nil {
		d.err("YAML validation failed: "+err.Error(), "Fix YAML syntax/duplicate keys and run `rest doctor` again.")
		return
	}
	d.ok("YAML files are syntactically valid")

	bundle, err := config.Load(d.configDir)
	if err != nil {
		d.err("configuration loading failed: "+err.Error(), "Fix unknown fields or missing enabled config files.")
		return
	}
	d.bundle = bundle
	d.projectDir = bundle.Rest.ProjectPath
	if d.projectDir == "" {
		d.projectDir = "."
	}
	if !filepath.IsAbs(d.projectDir) {
		d.projectDir = filepath.Clean(d.projectDir)
	}
	d.ok("configuration loaded")

	d.checkBasicConfig()
	d.checkGoModule()
	d.checkSQL()
	d.checkMongo()
	d.checkAuth()
	d.checkOpenAPI()
	d.checkDocker()
	d.checkGeneratedProject()
	d.checkCI()
}

func (d *doctorRunner) checkBasicConfig() {
	rest := d.bundle.Rest
	if rest.Module == "" {
		d.err("rest.yaml module is empty", "Set module to the generated Go module path.")
	} else {
		d.ok("rest.yaml module is set: " + rest.Module)
	}
	if rest.HTTP.Port < 1 || rest.HTTP.Port > 65535 {
		d.err("http.port is invalid", "Set http.port to a value between 1 and 65535.")
	} else {
		d.ok(fmt.Sprintf("HTTP port is valid: %d", rest.HTTP.Port))
	}
	for name, path := range map[string]string{
		"http.base_path":             rest.HTTP.BasePath,
		"http.health.path":           rest.HTTP.Health.Path,
		"openapi.spec_path":          rest.OpenAPI.SpecPath,
		"openapi.ui_path":            rest.OpenAPI.UIPath,
		"observability.metrics.path": rest.Observability.Metrics.Path,
	} {
		if path != "" && !strings.HasPrefix(path, "/") {
			d.err(name+" must start with /", "Use an absolute HTTP path such as /api or /health.")
		}
	}
	if rest.SQL.Bool() && rest.Mongo.Bool() {
		d.warn("both sql and mongo are enabled; SQL generator takes precedence", "Disable one backend unless this is intentional.")
	}
	if !rest.SQL.Bool() && !rest.Mongo.Bool() {
		d.err("no backend is enabled", "Set sql: enable or mongo: enable in rest_config/rest.yaml.")
	}
	if rest.HTTP.Middleware.CORS.Enabled.Bool() && rest.HTTP.Middleware.CORS.AllowCredentials {
		for _, origin := range rest.HTTP.Middleware.CORS.AllowOrigins {
			if origin == "*" {
				d.err("CORS credentials cannot be used with wildcard origin", "Use explicit origins or set allow_credentials: false.")
			}
		}
	}
}

func (d *doctorRunner) checkGoModule() {
	if _, err := exec.LookPath("go"); err != nil {
		d.err("go toolchain is not available", "Install Go and make sure `go` is in PATH.")
	} else {
		d.ok("go toolchain is available")
	}
	goMod := filepath.Join(d.projectDir, "go.mod")
	module, err := readDoctorModule(goMod)
	if err != nil {
		if d.generatedExists("cmd/main.go") {
			d.err("go.mod is missing or invalid", "Run `go mod init "+fallbackModule(d.bundle.Rest.Module)+"` or run `go mod tidy`.")
		} else {
			d.warn("go.mod is not present yet", "Run `rest gen`; it will create the generated Go module when needed.")
		}
		return
	}
	d.ok("go.mod found: " + module)
	if d.bundle.Rest.Module != "" && module != d.bundle.Rest.Module {
		d.warn("go.mod module differs from rest.yaml module", "Use the same module in go.mod and rest_config/rest.yaml unless this is intentional.")
	}
}

func (d *doctorRunner) checkSQL() {
	if !d.bundle.Rest.SQL.Bool() {
		return
	}
	if d.bundle.SQL == nil {
		d.err("sql is enabled but rest_sqlc.yaml was not loaded", "Create rest_config/rest_sqlc.yaml or disable sql.")
		return
	}
	d.ok("SQL config loaded")
	if !d.bundle.SQL.SQLC.Enabled.Bool() {
		d.warn("sql is enabled but sqlc.enable is disabled", "Set sqlc.enable: enable when SQL generation is expected.")
	}
	sqlcPath := d.bundle.SQL.SQLC.Path
	if sqlcPath == "" {
		d.err("sqlc_path is empty", "Set rest_config/rest_sqlc.yaml sqlc.sqlc_path.")
	} else {
		resolved := resolveDoctorPath(d.configDir, sqlcPath)
		if _, err := os.Stat(resolved); err != nil {
			d.err("sqlc config not found: "+resolved, "Fix sqlc.sqlc_path.")
		} else {
			d.ok("sqlc config found: " + filepath.ToSlash(resolved))
			d.checkSQLCInputs(resolved)
		}
	}
	if d.bundle.Rest.AutoSQLC.Bool() {
		if _, err := exec.LookPath("sqlc"); err != nil {
			d.err("auto_sqlc is enabled but sqlc is not installed", "Install it with: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest")
		} else {
			d.ok("sqlc binary is available")
		}
	}
	dbDir := filepath.Join(d.projectDir, "internal", "app", "db")
	if _, err := os.Stat(dbDir); err == nil {
		d.ok("SQLC generated db package exists")
		if _, err := os.Stat(filepath.Join(dbDir, "querier.go")); err != nil {
			d.warn("SQLC db package exists but querier.go was not found", "Run `sqlc generate` or `rest gen`.")
		}
	} else {
		d.warn("SQLC generated db package is not present yet", "Run `rest gen` after configuring SQLC.")
	}
}

func (d *doctorRunner) checkSQLCInputs(sqlcPath string) {
	inputs, err := readDoctorSQLCInputs(sqlcPath)
	if err != nil {
		d.err("sqlc config cannot be inspected: "+err.Error(), "Fix sqlc config YAML.")
		return
	}
	for _, dir := range inputs {
		if _, err := os.Stat(dir); err != nil {
			d.err("sqlc input path is missing: "+dir, "Create the schema/query path or fix sqlc config.")
		} else {
			d.ok("sqlc input path exists: " + filepath.ToSlash(dir))
		}
	}
}

func (d *doctorRunner) checkMongo() {
	if !d.bundle.Rest.Mongo.Bool() {
		return
	}
	if d.bundle.Mongo == nil {
		d.err("mongo is enabled but mongo_rest.yaml was not loaded", "Create rest_config/mongo_rest.yaml or disable mongo.")
		return
	}
	d.ok("Mongo config loaded")
	if d.bundle.Mongo.Connection.Database == "" {
		d.err("mongo.connection.database is empty", "Set the target Mongo database name.")
	}
	modelsPath := d.bundle.Mongo.Mongo.ModelsPath
	if modelsPath == "" {
		modelsPath = "rest_mongo"
	}
	modelsPath = resolveDoctorPath(d.configDir, modelsPath)
	info, err := os.Stat(modelsPath)
	if err != nil {
		d.err("Mongo models path is missing: "+modelsPath, "Create rest_config/rest_mongo or fix mongo.models_path.")
		return
	}
	if !info.IsDir() {
		d.err("Mongo models path is not a directory: "+modelsPath, "Use a directory with one YAML file per entity.")
		return
	}
	files, err := filepath.Glob(filepath.Join(modelsPath, "*.yaml"))
	if err != nil {
		d.err("cannot list Mongo model files: "+err.Error(), "Check mongo.models_path.")
		return
	}
	active := 0
	for _, file := range files {
		if strings.HasPrefix(filepath.Base(file), "rest_") || strings.HasPrefix(filepath.Base(file), ".") {
			continue
		}
		active++
		d.checkMongoContract(file)
	}
	if active == 0 {
		d.warn("no active Mongo entity contracts found", "Create rest_config/rest_mongo/<entity>.yaml; files starting with rest_ are documentation/examples.")
	} else {
		d.ok(fmt.Sprintf("Mongo active contract files: %d", active))
	}
}

func (d *doctorRunner) checkMongoContract(path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		d.err("cannot read Mongo contract "+path+": "+err.Error(), "Check file permissions.")
		return
	}
	var contract struct {
		Models []struct {
			Name       string `yaml:"name"`
			Collection string `yaml:"collection"`
			Embedded   bool   `yaml:"embedded"`
			Fields     []struct {
				Name string `yaml:"name"`
				Type string `yaml:"type"`
			} `yaml:"fields"`
		} `yaml:"models"`
		Methods []struct {
			Model     string `yaml:"model"`
			Name      string `yaml:"name"`
			Operation string `yaml:"operation"`
			HTTP      struct {
				Method string `yaml:"method"`
				Path   string `yaml:"path"`
			} `yaml:"http"`
		} `yaml:"methods"`
	}
	if err := yaml.Unmarshal(content, &contract); err != nil {
		d.err("Mongo contract cannot be parsed: "+filepath.ToSlash(path), "Fix YAML structure.")
		return
	}
	models := map[string]bool{}
	for _, model := range contract.Models {
		if model.Name == "" {
			d.err("Mongo model without name in "+filepath.Base(path), "Set models[].name.")
			continue
		}
		models[model.Name] = true
		if !model.Embedded && model.Collection == "" {
			d.err("Mongo model "+model.Name+" has no collection", "Set collection for non-embedded models.")
		}
		if len(model.Fields) == 0 {
			d.warn("Mongo model "+model.Name+" has no fields", "Add fields for better OpenAPI/request schemas.")
		}
	}
	for _, method := range contract.Methods {
		if method.Name == "" || method.Model == "" {
			d.err("Mongo method without name/model in "+filepath.Base(path), "Set methods[].name and methods[].model.")
			continue
		}
		if !models[method.Model] {
			d.err("Mongo method "+method.Name+" references unknown model "+method.Model, "Use a model declared in the same active contract file.")
		}
		if !validMongoOperation(method.Operation) {
			d.err("Mongo method "+method.Name+" has unsupported operation "+method.Operation, "Use find_one, find_many, update_one, delete_one, or aggregate.")
		}
		if method.HTTP.Path == "" || !strings.HasPrefix(method.HTTP.Path, "/") {
			d.err("Mongo method "+method.Name+" has invalid HTTP path", "Set http.path starting with /.")
		}
	}
}

func (d *doctorRunner) checkAuth() {
	if !d.bundle.Rest.Auth.Bool() {
		return
	}
	authPath := filepath.Join(d.configDir, "auth_rest.yaml")
	if d.bundle.Auth == nil {
		d.warn("auth is enabled but auth_rest.yaml does not exist yet", "Run `rest gen`, configure generated endpoint policies, then run `rest gen` again.")
		return
	}
	d.ok("auth_rest.yaml loaded")
	auth := d.bundle.Auth
	strategy := strings.ToLower(auth.Authentication.Strategy)
	if strategy != "jwt" && strategy != "basic" {
		d.err("unsupported auth strategy: "+auth.Authentication.Strategy, "Use jwt or basic.")
	}
	if len(auth.Endpoints) == 0 {
		d.warn("auth_rest.yaml has no endpoint policies", "Run `rest gen` to discover endpoints or add policies manually.")
	} else {
		d.ok(fmt.Sprintf("auth endpoint policies: %d", len(auth.Endpoints)))
	}
	for _, endpoint := range auth.Endpoints {
		if endpoint.Method == "" || endpoint.Path == "" {
			d.err("auth endpoint policy has empty method/path", "Every endpoint policy needs method and path.")
		}
		if endpoint.Public && endpoint.RequireAuth {
			d.err("auth endpoint "+endpoint.Method+" "+endpoint.Path+" is both public and require_auth", "Set either public: true or require_auth: true, not both.")
		}
	}
	if strategy == "jwt" {
		if auth.Authentication.JWT.SigningKeyEnv == "" {
			d.err("JWT signing_key_env is empty", "Set authentication.jwt.signing_key_env.")
		}
		if d.bundle.Rest.SQL.Bool() && d.bundle.Auth.Identity.Table == "" {
			d.err("JWT identity table is empty", "Set identity.table in auth_rest.yaml.")
		}
	}
	if strategy == "basic" {
		if auth.Authentication.Basic.UsernameEnv == "" || auth.Authentication.Basic.PasswordEnv == "" {
			d.err("Basic Auth env names are incomplete", "Set authentication.basic.username_env and password_env.")
		}
	}
	if _, err := os.Stat(authPath); err == nil {
		d.ok("auth_rest.yaml exists on disk")
	}
}

func (d *doctorRunner) checkOpenAPI() {
	if !d.bundle.Rest.OpenAPI.Enabled.Bool() {
		return
	}
	output := d.bundle.Rest.OpenAPI.Output
	if output == "" {
		output = "docs/swagger.yaml"
	}
	if !filepath.IsAbs(output) {
		output = filepath.Join(d.projectDir, output)
	}
	if _, err := os.Stat(output); err == nil {
		d.ok("OpenAPI output exists: " + filepath.ToSlash(output))
	} else {
		d.warn("OpenAPI output is not generated yet", "Run `rest gen`.")
	}
}

func (d *doctorRunner) checkDocker() {
	if !d.bundle.Rest.Docker.Enabled.Bool() && !d.bundle.Rest.Docker.Compose.Enabled.Bool() {
		return
	}
	if d.bundle.Rest.Docker.Port != 0 && d.bundle.Rest.HTTP.Port != 0 && d.bundle.Rest.Docker.Port != d.bundle.Rest.HTTP.Port {
		d.err("docker.port does not match http.port", "Set docker.port equal to http.port.")
	}
	if d.bundle.Rest.Docker.Healthcheck.Enabled.Bool() && !d.bundle.Rest.HTTP.Health.Enabled.Bool() {
		d.err("Docker healthcheck requires http.health.enabled", "Enable http.health or disable docker.healthcheck.")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		d.warn("Docker is enabled but docker CLI is not available", "Install Docker or disable docker output.")
	} else {
		d.ok("docker CLI is available")
		if err := runDoctorCommand(2*time.Second, "docker", "info"); err != nil {
			d.warn("Docker daemon is not reachable", "Start Docker before building/running generated containers.")
		} else {
			d.ok("docker daemon is reachable")
		}
	}
	if d.generatedExists("Dockerfile") {
		d.ok("Dockerfile exists")
	} else if d.bundle.Rest.Docker.Enabled.Bool() {
		d.warn("Dockerfile is not generated yet", "Run `rest gen`.")
	}
	compose := d.bundle.Rest.Docker.Compose.Output
	if compose == "" {
		compose = "docker-compose.yml"
	}
	if d.bundle.Rest.Docker.Compose.Enabled.Bool() {
		if d.generatedExists(compose) {
			d.ok("docker compose file exists")
		} else {
			d.warn("docker compose file is not generated yet", "Run `rest gen`.")
		}
	}
}

func (d *doctorRunner) checkGeneratedProject() {
	if !d.generatedExists("cmd/main.go") {
		d.warn("generated application is not present yet", "Run `rest gen`.")
		return
	}
	d.ok("generated cmd/main.go exists")
	for _, path := range []string{
		"internal/app/domain",
		"internal/app/repository",
		"internal/app/services",
		"internal/app/transport",
	} {
		if d.generatedExists(path) {
			d.ok("generated layer exists: " + path)
		} else {
			d.err("generated layer is missing: "+path, "Run `rest gen` again.")
		}
	}
	if d.bundle.Rest.Auth.Bool() && d.bundle.Auth != nil {
		if d.generatedExists("internal/app/transport/httpserver/auth_middleware.go") {
			d.ok("auth middleware generated")
		} else {
			d.warn("auth middleware is not generated yet", "Configure auth_rest.yaml and run `rest gen` again.")
		}
	}
	if d.generatedExists(filepath.Join(".rest", "generation.json")) {
		d.ok("generation fingerprint state exists")
	} else {
		d.warn("generation fingerprint state is missing", "Run `rest gen` to create generation state.")
	}
	if d.generatedExists("go.sum") {
		d.ok("go.sum exists")
	} else {
		d.warn("go.sum is missing", "Run `go mod tidy` or `rest gen`.")
	}
}

func (d *doctorRunner) checkCI() {
	if d.generatedExists("Makefile") {
		d.ok("Makefile exists")
	} else {
		d.warn("Makefile is missing", "Enable features.makefile or keep your own build commands documented.")
	}
	ciPath := filepath.Join(".github", "workflows", "ci.yml")
	if d.generatedExists(ciPath) || fileExists(ciPath) {
		d.ok("GitHub CI workflow exists")
		readPath := ciPath
		if d.generatedExists(ciPath) {
			readPath = filepath.Join(d.projectDir, ciPath)
		}
		content, _ := os.ReadFile(readPath)
		text := string(content)
		if strings.Contains(text, "runtime-e2e") {
			d.ok("CI contains runtime e2e job")
		} else {
			d.warn("CI does not mention runtime-e2e", "Add live runtime e2e before production releases.")
		}
	} else {
		d.warn("GitHub CI workflow is missing", "Add .github/workflows/ci.yml or enable features.ci.")
	}
}

func (d *doctorRunner) generatedExists(path string) bool {
	return fileExists(filepath.Join(d.projectDir, path))
}

func (r doctorReport) Errors() int {
	count := 0
	for _, check := range r.Checks {
		if check.Level == doctorError {
			count++
		}
	}
	return count
}

func (r doctorReport) Warnings() int {
	count := 0
	for _, check := range r.Checks {
		if check.Level == doctorWarn {
			count++
		}
	}
	return count
}

func (r doctorReport) OKs() int {
	count := 0
	for _, check := range r.Checks {
		if check.Level == doctorOK {
			count++
		}
	}
	return count
}

func (r doctorReport) Print(w io.Writer) {
	fmt.Fprintln(w, "REST Doctor")
	fmt.Fprintln(w)
	for _, check := range r.Checks {
		fmt.Fprintf(w, "%-5s %s\n", check.Level, check.Name)
		if check.Fix != "" {
			fmt.Fprintf(w, "      Fix: %s\n", check.Fix)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Summary: %d OK, %d WARN, %d ERROR\n", r.OKs(), r.Warnings(), r.Errors())
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func resolveDoctorPath(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(base, path))
}

func readDoctorModule(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	return "", errors.New("module declaration not found")
}

func fallbackModule(module string) string {
	if module != "" {
		return module
	}
	return "github.com/you/app"
}

func readDoctorSQLCInputs(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var document struct {
		SQL []struct {
			Queries any `yaml:"queries"`
			Schema  any `yaml:"schema"`
		} `yaml:"sql"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	if len(document.SQL) == 0 {
		return nil, nil
	}
	base := filepath.Dir(path)
	var result []string
	for _, value := range append(doctorYAMLPaths(document.SQL[0].Queries), doctorYAMLPaths(document.SQL[0].Schema)...) {
		if !filepath.IsAbs(value) {
			value = filepath.Join(base, value)
		}
		result = append(result, filepath.Clean(value))
	}
	sort.Strings(result)
	return result, nil
}

func doctorYAMLPaths(value any) []string {
	switch value := value.(type) {
	case string:
		return []string{value}
	case []any:
		result := make([]string, 0, len(value))
		for _, item := range value {
			if path, ok := item.(string); ok {
				result = append(result, path)
			}
		}
		return result
	default:
		return nil
	}
}

func validMongoOperation(value string) bool {
	switch strings.ToLower(value) {
	case "find_one", "find_many", "update_one", "delete_one", "aggregate":
		return true
	default:
		return false
	}
}

func runDoctorCommand(timeout time.Duration, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("%s timed out", name)
	}
	return err
}
