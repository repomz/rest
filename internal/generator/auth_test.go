package generator

import (
	"strings"
	"testing"
)

func TestAppTemplateWrapsProtectedEndpoints(t *testing.T) {
	data := renderData{
		Module:   "example.test/app",
		DBImport: "example.test/app/internal/db",
		Features: FeatureOptions{
			HTTP: HTTPFeatures{BasePath: "/api"},
			Auth: AuthFeatures{
				Enabled: true, Strategy: "jwt", JWTAlgorithm: "HS256",
				JWTSecretEnv: "JWT_SECRET", JWTHeader: "Authorization", JWTScheme: "Bearer",
				RoleClaim:     "roles",
				DefaultPolicy: "deny",
				Policies: map[string]AuthPolicy{
					"GET /api/users": {Roles: []string{"admin"}},
					"GET /api/":      {Public: true},
				},
			},
		},
		Tables: []table{{
			Name: "users", Singular: "user", GoName: "User", GoPlural: "Users",
			RouteBase: "/users", Queries: querySet{GetAll: true},
		}},
	}
	rendered, err := renderTemplateForTest(t, "main.go", appMainTemplate, data)
	if err != nil {
		t.Fatal(err)
	}
	text := string(rendered)
	if !strings.Contains(text, `httpServer.CheckRoles(httpServer.GetAllUsers, "admin")`) {
		t.Fatalf("protected endpoint was not wrapped:\n%s", text)
	}
	if strings.Contains(text, `httpServer.CheckAuthorizedUser(func(w http.ResponseWriter, r *http.Request)`) {
		t.Fatalf("public root endpoint must not be wrapped:\n%s", text)
	}
}

func TestAppTemplateConfiguresBasicAuth(t *testing.T) {
	data := renderData{
		Module:   "example.test/app",
		DBImport: "example.test/app/internal/db",
		Features: FeatureOptions{
			HTTP: HTTPFeatures{BasePath: "/"},
			Auth: AuthFeatures{
				Enabled: true, Strategy: "basic",
				BasicUsernameEnv: "ADMIN_USER", BasicPasswordEnv: "ADMIN_PASSWORD",
				BasicRealm: "Admin API", BasicRoles: []string{"admin"},
				DefaultPolicy: "deny",
			},
		},
	}
	rendered, err := renderTemplateForTest(t, "main.go", appMainTemplate, data)
	if err != nil {
		t.Fatal(err)
	}
	text := string(rendered)
	for _, expected := range []string{
		`basicAuthConfig := httpserver.BasicAuthConfig{`,
		`Username: os.Getenv("ADMIN_USER")`,
		`Password: os.Getenv("ADMIN_PASSWORD")`,
		`Realm:    "Admin API"`,
		`Roles:    []string{"admin"}`,
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("basic auth configuration missing %q:\n%s", expected, text)
		}
	}
}
