package cli

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestRuntimeE2EPostgresJWTAuthRBACSwagger(t *testing.T) {
	requireRuntimeE2E(t)
	requireBinary(t, "sqlc")
	postgresDSN := runtimeEnv("POSTGRES_DSN", "postgres://app_user:app_password@localhost:5432/myapp_db?sslmode=disable")
	if err := waitPostgres(postgresDSN); err != nil {
		t.Skipf("Postgres is not available: %v", err)
	}

	projectDir := filepath.Join(t.TempDir(), "sql-runtime-app")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	withWorkingDir(t, projectDir)
	if err := run([]string{"init"}); err != nil {
		t.Fatal(err)
	}
	patchFileForE2E(t, filepath.Join(projectDir, "rest_config", "rest.yaml"), map[string]string{
		"module: github.com/repomz/myapp": "module: example.test/runtime-sql",
		"auth: disable":                   "auth: enable",
	})
	writeRuntimeSQLCProject(t, projectDir)
	if err := run([]string{"gen"}); err != nil {
		t.Fatal(err)
	}
	writeRuntimeSQLAuthConfig(t, filepath.Join(projectDir, "rest_config", "auth_rest.yaml"))
	if err := run([]string{"gen"}); err != nil {
		t.Fatal(err)
	}
	applyRuntimePostgresSchema(t, postgresDSN)

	addr := freeRuntimeAddr(t)
	cmd := startGeneratedApp(t, projectDir, []string{
		"HTTP_ADDR=" + addr,
		"DB_DSN=" + postgresDSN,
		"JWT_SIGNING_KEY=runtime-secret",
	})
	defer stopGeneratedApp(t, cmd)

	baseURL := "http://" + addr
	waitRuntimeHTTP(t, baseURL+"/swagger/openapi.yaml", http.StatusOK, "")
	assertRuntimeSecurityHeaders(t, httpRequest(t, http.MethodGet, baseURL+"/swagger/openapi.yaml", "", nil))

	assertRuntimeStatus(t, httpRequest(t, http.MethodGet, baseURL+"/items", "", nil), http.StatusUnauthorized)
	assertRuntimeStatus(t, httpRequest(t, http.MethodGet, baseURL+"/items", "Bearer malformed.token", nil), http.StatusUnauthorized)
	assertRuntimeStatus(t, httpRequest(t, http.MethodGet, baseURL+"/items", "Bearer "+runtimeJWT(t, "wrong-secret", time.Now().Add(15*time.Minute), []string{"admin"}), nil), http.StatusUnauthorized)
	assertRuntimeStatus(t, httpRequest(t, http.MethodGet, baseURL+"/items", "Bearer "+runtimeJWT(t, "runtime-secret", time.Now().Add(-time.Minute), []string{"admin"}), nil), http.StatusUnauthorized)
	assertRuntimeStatus(t, httpRequest(t, http.MethodPost, baseURL+"/signup", "", map[string]any{"username": "viewer", "password": "secret"}), http.StatusOK)
	viewerToken := runtimeSignIn(t, baseURL, "viewer", "secret")
	assertRuntimeStatus(t, httpRequest(t, http.MethodGet, baseURL+"/items", "Bearer "+viewerToken, nil), http.StatusUnauthorized)

	assertRuntimeStatus(t, httpRequest(t, http.MethodPost, baseURL+"/signup", "", map[string]any{"username": "admin", "password": "secret"}), http.StatusOK)
	setRuntimeUserRoles(t, postgresDSN, "admin", "admin")
	adminToken := runtimeSignIn(t, baseURL, "admin", "secret")
	assertRuntimeStatus(t, httpRequest(t, http.MethodPost, baseURL+"/items", "Bearer "+adminToken, map[string]any{"name": "runtime item"}), http.StatusOK)
	itemsResponse := httpRequest(t, http.MethodGet, baseURL+"/items", "Bearer "+adminToken, nil)
	assertRuntimeStatus(t, itemsResponse, http.StatusOK)
	if !strings.Contains(string(itemsResponse.Body), "runtime item") {
		t.Fatalf("created item was not returned by runtime list endpoint:\n%s", itemsResponse.Body)
	}

	swagger := string(httpRequest(t, http.MethodGet, baseURL+"/swagger/openapi.yaml", "", nil).Body)
	for _, expected := range []string{"/items:", "security:", "bearerAuth:"} {
		if !strings.Contains(swagger, expected) {
			t.Fatalf("runtime swagger missing %q:\n%s", expected, swagger)
		}
	}
}

func TestRuntimeE2EMongoBasicAuthCRUDSwagger(t *testing.T) {
	requireRuntimeE2E(t)
	mongoURI := runtimeEnv("MONGO_URI", "mongodb://localhost:27017")
	if err := waitMongoTCP(mongoURI); err != nil {
		t.Skipf("MongoDB is not available: %v", err)
	}

	projectDir := filepath.Join(t.TempDir(), "mongo-runtime-app")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	withWorkingDir(t, projectDir)
	if err := run([]string{"init", "--example", "mongo"}); err != nil {
		t.Fatal(err)
	}
	patchFileForE2E(t, filepath.Join(projectDir, "rest_config", "rest.yaml"), map[string]string{
		"auth: disable": "auth: enable",
	})
	patchFileForE2E(t, filepath.Join(projectDir, "rest_config", "mongo_rest.yaml"), map[string]string{
		"  database: myapp_db": "  database: runtime_mongo_e2e",
	})
	if err := run([]string{"gen"}); err != nil {
		t.Fatal(err)
	}
	writeRuntimeMongoBasicAuthConfig(t, filepath.Join(projectDir, "rest_config", "auth_rest.yaml"))
	if err := run([]string{"gen"}); err != nil {
		t.Fatal(err)
	}

	addr := freeRuntimeAddr(t)
	cmd := startGeneratedApp(t, projectDir, []string{
		"HTTP_ADDR=" + addr,
		"MONGO_URI=" + mongoURI,
		"BASIC_AUTH_USERNAME=admin",
		"BASIC_AUTH_PASSWORD=secret",
	})
	defer stopGeneratedApp(t, cmd)

	baseURL := "http://" + addr
	waitRuntimeHTTP(t, baseURL+"/swagger/openapi.yaml", http.StatusOK, "")
	assertRuntimeSecurityHeaders(t, httpRequest(t, http.MethodGet, baseURL+"/swagger/openapi.yaml", "", nil))

	assertRuntimeStatus(t, httpRequest(t, http.MethodGet, baseURL+"/items", "", nil), http.StatusUnauthorized)
	assertRuntimeStatus(t, httpRequest(t, http.MethodGet, baseURL+"/items", "Basic "+basicAuth("admin", "wrong"), nil), http.StatusUnauthorized)
	assertRuntimeStatus(t, httpRequest(t, http.MethodGet, baseURL+"/items", "Basic not-base64", nil), http.StatusUnauthorized)
	assertRuntimeStatus(t, httpRequest(t, http.MethodPost, baseURL+"/items", "Basic "+basicAuth("admin", "secret"), map[string]any{"title": "runtime mongo", "status": "draft"}), http.StatusOK)
	itemsResponse := httpRequest(t, http.MethodGet, baseURL+"/items", "Basic "+basicAuth("admin", "secret"), nil)
	assertRuntimeStatus(t, itemsResponse, http.StatusOK)
	if !strings.Contains(string(itemsResponse.Body), "runtime mongo") {
		t.Fatalf("created Mongo item was not returned by runtime list endpoint:\n%s", itemsResponse.Body)
	}

	swagger := string(httpRequest(t, http.MethodGet, baseURL+"/swagger/openapi.yaml", "", nil).Body)
	for _, expected := range []string{"/items:", "security:", "basicAuth:"} {
		if !strings.Contains(swagger, expected) {
			t.Fatalf("runtime Mongo swagger missing %q:\n%s", expected, swagger)
		}
	}
}

type runtimeResponse struct {
	Status int
	Header http.Header
	Body   []byte
}

func requireRuntimeE2E(t *testing.T) {
	t.Helper()
	if os.Getenv("REST_RUNTIME_E2E") != "1" {
		t.Skip("set REST_RUNTIME_E2E=1 to run live runtime e2e tests")
	}
}

func requireBinary(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s is required for runtime e2e: %v", name, err)
	}
}

func runtimeEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func freeRuntimeAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

func startGeneratedApp(t *testing.T, projectDir string, env []string) *exec.Cmd {
	t.Helper()
	binaryPath := filepath.Join(t.TempDir(), "generated-app")
	buildCtx, buildCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer buildCancel()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", binaryPath, "./cmd")
	build.Dir = projectDir
	build.Env = runtimeGeneratedEnv(env)
	var buildOutput bytes.Buffer
	build.Stdout = &buildOutput
	build.Stderr = &buildOutput
	if err := build.Run(); err != nil {
		if buildCtx.Err() == context.DeadlineExceeded {
			t.Fatalf("generated app build timed out after 3m\n%s", buildOutput.String())
		}
		t.Fatalf("generated app build failed: %v\n%s", err, buildOutput.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, binaryPath)
	cmd.Dir = projectDir
	cmd.Env = runtimeGeneratedEnv(env)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("generated app output:\n%s", output.String())
		}
	})
	return cmd
}

func runtimeGeneratedEnv(extra []string) []string {
	env := append([]string{}, os.Environ()...)
	env = append(env, "GOWORK=off")
	env = append(env, extra...)
	return env
}

func stopGeneratedApp(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}

func waitRuntimeHTTP(t *testing.T, url string, status int, auth string) {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		response := httpRequest(t, http.MethodGet, url, auth, nil)
		if response.Status == status {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("runtime endpoint %s did not become ready with status %d", url, status)
}

func httpRequest(t *testing.T, method, url, auth string, body any) runtimeResponse {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return runtimeResponse{Status: 0, Body: []byte(err.Error())}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return runtimeResponse{Status: resp.StatusCode, Header: resp.Header.Clone(), Body: data}
}

func assertRuntimeStatus(t *testing.T, response runtimeResponse, status int) {
	t.Helper()
	if response.Status != status {
		t.Fatalf("status = %d, want %d; body:\n%s", response.Status, status, response.Body)
	}
}

func assertRuntimeSecurityHeaders(t *testing.T, response runtimeResponse) {
	t.Helper()
	assertRuntimeStatus(t, response, http.StatusOK)
	for name, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	} {
		if got := response.Header.Get(name); got != want {
			t.Fatalf("security header %s = %q, want %q", name, got, want)
		}
	}
}

func runtimeSignIn(t *testing.T, baseURL, username, password string) string {
	t.Helper()
	response := httpRequest(t, http.MethodPost, baseURL+"/signin", "", map[string]any{"username": username, "password": password})
	assertRuntimeStatus(t, response, http.StatusOK)
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(response.Body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Token == "" {
		t.Fatalf("signin response did not contain token:\n%s", response.Body)
	}
	return payload.Token
}

func waitPostgres(dsn string) error {
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		db, err := sql.Open("postgres", dsn)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			lastErr = db.PingContext(ctx)
			cancel()
			_ = db.Close()
			if lastErr == nil {
				return nil
			}
		} else {
			lastErr = err
		}
		time.Sleep(250 * time.Millisecond)
	}
	return lastErr
}

func waitMongoTCP(uri string) error {
	parsed, err := url.Parse(uri)
	if err != nil {
		return err
	}
	host := parsed.Host
	if host == "" {
		host = "localhost:27017"
	}
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", host, time.Second)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = err
		time.Sleep(250 * time.Millisecond)
	}
	return lastErr
}

func applyRuntimePostgresSchema(t *testing.T, dsn string) {
	t.Helper()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	statements := []string{
		`CREATE EXTENSION IF NOT EXISTS pgcrypto`,
		`DROP TABLE IF EXISTS items`,
		`DROP TABLE IF EXISTS users`,
		runtimeSQLSchema,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("apply runtime postgres schema failed: %v\nSQL:\n%s", err, statement)
		}
	}
}

func setRuntimeUserRoles(t *testing.T, dsn, username, roles string) {
	t.Helper()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE users SET roles = $1 WHERE username = $2`, roles, username); err != nil {
		t.Fatal(err)
	}
}

func basicAuth(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

func runtimeJWT(t *testing.T, secret string, expiresAt time.Time, roles []string) string {
	t.Helper()
	header := map[string]any{"alg": "HS256", "typ": "JWT"}
	claims := map[string]any{
		"exp":   expiresAt.Unix(),
		"iat":   time.Now().Unix(),
		"roles": roles,
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func writeRuntimeSQLCProject(t *testing.T, projectDir string) {
	t.Helper()
	writeE2EFile(t, filepath.Join(projectDir, "rest_sqlc", "schema", "runtime.sql"), runtimeSQLSchema)
	writeE2EFile(t, filepath.Join(projectDir, "rest_sqlc", "queries", "runtime.sql"), runtimeSQLQueries)
}

func writeRuntimeSQLAuthConfig(t *testing.T, path string) {
	t.Helper()
	writeE2EFile(t, path, `version: "0.1.0"
identity:
  model: User
  table: users
  id_field: id
  username_field: username
  password_field: password
  roles_field: roles
  claims_model: User
authentication:
  strategy: jwt
  jwt:
    algorithm: HS256
    signing_key_env: JWT_SIGNING_KEY
    verification_key_file_env: JWT_VERIFICATION_KEY_FILE
    issuer: runtime
    audience: runtime-api
    access_token_ttl: 15m
    refresh_token: false
    refresh_token_storage: context
    leeway: 30s
    header_name: Authorization
    token_prefix: Bearer
  basic:
    username_env: BASIC_AUTH_USERNAME
    password_env: BASIC_AUTH_PASSWORD
    realm: Restricted
    roles: []
authorization:
  default_policy: deny
  role_claim: roles
endpoints:
  - name: SignUp
    method: POST
    path: /signup
    public: true
    require_auth: false
    roles: []
  - name: SignIn
    method: POST
    path: /signin
    public: true
    require_auth: false
    roles: []
  - name: GetOpenAPISpec
    method: GET
    path: /swagger/openapi.yaml
    public: true
    require_auth: false
    roles: []
  - name: CreateItem
    method: POST
    path: /items
    public: false
    require_auth: true
    roles: ["admin"]
  - name: GetAllItems
    method: GET
    path: /items
    public: false
    require_auth: true
    roles: ["admin"]
`)
}

func writeRuntimeMongoBasicAuthConfig(t *testing.T, path string) {
	t.Helper()
	writeE2EFile(t, path, `version: "0.1.0"
identity:
  model: User
  table: users
  id_field: id
  username_field: username
  password_field: password
  roles_field: roles
  claims_model: User
authentication:
  strategy: basic
  jwt:
    algorithm: HS256
    signing_key_env: JWT_SIGNING_KEY
    verification_key_file_env: JWT_VERIFICATION_KEY_FILE
    issuer: runtime
    audience: runtime-api
    access_token_ttl: 15m
    refresh_token: false
    refresh_token_storage: context
    leeway: 30s
    header_name: Authorization
    token_prefix: Bearer
  basic:
    username_env: BASIC_AUTH_USERNAME
    password_env: BASIC_AUTH_PASSWORD
    realm: Restricted
    roles: ["admin"]
authorization:
  default_policy: deny
  role_claim: roles
endpoints:
  - name: GetOpenAPISpec
    method: GET
    path: /swagger/openapi.yaml
    public: true
    require_auth: false
    roles: []
  - name: CreateItem
    method: POST
    path: /items
    public: false
    require_auth: true
    roles: ["admin"]
  - name: GetAllItems
    method: GET
    path: /items
    public: false
    require_auth: true
    roles: ["admin"]
`)
}

const runtimeSQLSchema = `
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    roles TEXT NOT NULL DEFAULT '',
    deleted BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    deleted BOOLEAN NOT NULL DEFAULT FALSE
);
`

const runtimeSQLQueries = `
-- name: CreateUser :one
INSERT INTO users (username, password)
VALUES ($1, $2)
RETURNING *;

-- name: GetUsers :many
SELECT * FROM users
WHERE deleted = false
ORDER BY username;

-- name: CreateItem :one
INSERT INTO items (name)
VALUES ($1)
RETURNING *;

-- name: GetItems :many
SELECT * FROM items
WHERE deleted = false
ORDER BY name;

-- name: GetItemByID :one
SELECT * FROM items
WHERE id = $1 AND deleted = false;

-- name: SoftDeleteItem :exec
UPDATE items SET deleted = true
WHERE id = $1;
`
