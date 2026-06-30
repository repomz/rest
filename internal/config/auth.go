package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

type GeneratedEndpoint struct {
	Name   string
	Method string
	Path   string
	Public bool
}

func GenerateAuth(dir string, endpoints []GeneratedEndpoint) error {
	path := filepath.Join(dir, "auth_rest.yaml")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	_, err := SyncAuth(dir, nil, endpoints)
	return err
}

func SyncAuth(dir string, current *Auth, endpoints []GeneratedEndpoint) (bool, error) {
	path := filepath.Join(dir, "auth_rest.yaml")
	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Path == endpoints[j].Path {
			return endpoints[i].Method < endpoints[j].Method
		}
		return endpoints[i].Path < endpoints[j].Path
	})
	auth := defaultAuth()
	if current != nil {
		auth = *current
		auth.Endpoints = nil
	}
	existing := map[string]AuthEndpoint{}
	if current != nil {
		for _, endpoint := range current.Endpoints {
			existing[endpoint.Method+" "+endpoint.Path] = endpoint
		}
	}
	for _, endpoint := range endpoints {
		policy, ok := existing[endpoint.Method+" "+endpoint.Path]
		if !ok {
			policy = AuthEndpoint{
				Public: endpoint.Public, RequireAuth: !endpoint.Public, Roles: []string{},
			}
		}
		policy.Name = endpoint.Name
		policy.Method = endpoint.Method
		policy.Path = endpoint.Path
		auth.Endpoints = append(auth.Endpoints, policy)
	}
	if current != nil && reflect.DeepEqual(*current, auth) {
		return false, nil
	}
	if err := writeAuth(path, auth); err != nil {
		return false, err
	}
	return true, nil
}

func defaultAuth() Auth {
	return Auth{
		Version: "0.1.0",
		Identity: AuthIdentity{
			Model: "User", Table: "users", IDField: "id", UsernameField: "username",
			PasswordField: "password", RolesField: "roles", ClaimsModel: "User",
		},
		Authentication: AuthAuthentication{
			Strategy: "jwt",
			JWT: AuthJWT{
				Algorithm: "HS256", SigningKeyEnv: "JWT_SIGNING_KEY",
				VerificationKeyFileEnv: "JWT_VERIFICATION_KEY_FILE",
				Issuer:                 "myapp", Audience: "myapp-api", AccessTokenTTL: "15m",
				RefreshToken: false, RefreshTokenStorage: "context", Leeway: "30s",
				HeaderName: "Authorization", TokenPrefix: "Bearer",
			},
			Basic: AuthBasic{
				UsernameEnv: "BASIC_AUTH_USERNAME", PasswordEnv: "BASIC_AUTH_PASSWORD",
				Realm: "Restricted", Roles: []string{},
			},
		},
		Authorization: AuthAuthorization{
			DefaultPolicy: "deny", RoleClaim: "roles",
		},
	}
}

func writeAuth(path string, auth Auth) error {
	var body bytes.Buffer
	body.WriteString("# ==============================================================================\n")
	body.WriteString("# REST: AUTHENTICATION AND AUTHORIZATION\n")
	body.WriteString("# ==============================================================================\n")
	body.WriteString("# Generated after the first `rest gen` when rest.yaml contains auth: enable.\n")
	body.WriteString("# Configure every endpoint, then run `rest gen` again.\n")
	body.WriteString("# authentication.strategy: jwt or basic.\n")
	body.WriteString("# JWT token issuing currently uses HS256 with signing_key_env.\n")
	body.WriteString("# identity.claims_model defaults to the generated User model when users table exists.\n")
	body.WriteString("# Basic Auth reads one service credential pair from username_env/password_env.\n")
	body.WriteString("# public: true bypasses authentication. roles is an allow-list; [] allows any authenticated user.\n\n")

	fmt.Fprintf(&body, "version: %s # Version of this auth configuration format.\n\n", yamlString(auth.Version))
	body.WriteString("identity: # Maps authentication data to the generated user model.\n")
	fmt.Fprintf(&body, "  model: %s # Generated Go model used as the authenticated identity.\n", yamlString(auth.Identity.Model))
	fmt.Fprintf(&body, "  table: %s # Database table that stores users.\n", yamlString(auth.Identity.Table))
	fmt.Fprintf(&body, "  id_field: %s # Field used as the unique user identifier.\n", yamlString(auth.Identity.IDField))
	fmt.Fprintf(&body, "  username_field: %s # Field used as login name in signup/signin requests.\n", yamlString(auth.Identity.UsernameField))
	fmt.Fprintf(&body, "  password_field: %s # Field storing the password hash.\n", yamlString(auth.Identity.PasswordField))
	fmt.Fprintf(&body, "  roles_field: %s # Field containing user roles for authorization.\n", yamlString(auth.Identity.RolesField))
	fmt.Fprintf(&body, "  claims_model: %s # Generated model used to fill UserClaims; keep User unless you need a custom claims source.\n\n", yamlString(auth.Identity.ClaimsModel))

	body.WriteString("authentication: # Selects and configures the request authentication mechanism.\n")
	fmt.Fprintf(&body, "  strategy: %s # Active strategy: jwt or basic.\n", yamlString(auth.Authentication.Strategy))
	body.WriteString("  jwt: # JWT settings; ignored when strategy is basic.\n")
	fmt.Fprintf(&body, "    algorithm: %s # Signature algorithm for generated tokens; currently HS256.\n", yamlString(auth.Authentication.JWT.Algorithm))
	fmt.Fprintf(&body, "    signing_key_env: %s # Environment variable containing the HS256 signing key.\n", yamlString(auth.Authentication.JWT.SigningKeyEnv))
	fmt.Fprintf(&body, "    verification_key_file_env: %s # Reserved for asymmetric JWT verification; leave as-is for HS256.\n", yamlString(auth.Authentication.JWT.VerificationKeyFileEnv))
	fmt.Fprintf(&body, "    issuer: %s # Expected standard JWT \"iss\" claim; empty disables this check.\n", yamlString(auth.Authentication.JWT.Issuer))
	fmt.Fprintf(&body, "    audience: %s # Expected standard JWT \"aud\" claim; empty disables this check.\n", yamlString(auth.Authentication.JWT.Audience))
	fmt.Fprintf(&body, "    access_token_ttl: %s # Access token lifetime used for the JWT ExpiresAt claim.\n", yamlString(auth.Authentication.JWT.AccessTokenTTL))
	fmt.Fprintf(&body, "    refresh_token: %t # true also returns a refresh_token from signin.\n", auth.Authentication.JWT.RefreshToken)
	fmt.Fprintf(&body, "    refresh_token_storage: %s # Where refresh tokens are expected to be stored: context or client.\n", yamlString(auth.Authentication.JWT.RefreshTokenStorage))
	fmt.Fprintf(&body, "    leeway: %s # Allowed clock difference when checking \"exp\" and \"nbf\".\n", yamlString(auth.Authentication.JWT.Leeway))
	fmt.Fprintf(&body, "    header_name: %s # HTTP header containing the token.\n", yamlString(auth.Authentication.JWT.HeaderName))
	fmt.Fprintf(&body, "    token_prefix: %s # Prefix before the token value in the header.\n", yamlString(auth.Authentication.JWT.TokenPrefix))
	body.WriteString("  basic: # Basic Auth settings; ignored when strategy is jwt.\n")
	fmt.Fprintf(&body, "    username_env: %s # Environment variable containing the expected username.\n", yamlString(auth.Authentication.Basic.UsernameEnv))
	fmt.Fprintf(&body, "    password_env: %s # Environment variable containing the expected password.\n", yamlString(auth.Authentication.Basic.PasswordEnv))
	fmt.Fprintf(&body, "    realm: %s # Realm returned in the WWW-Authenticate challenge.\n", yamlString(auth.Authentication.Basic.Realm))
	fmt.Fprintf(&body, "    roles: %s # Roles assigned to a successfully authenticated user.\n\n", yamlStringList(auth.Authentication.Basic.Roles))

	body.WriteString("authorization: # Controls default access and role-based authorization.\n")
	fmt.Fprintf(&body, "  default_policy: %s # Policy for endpoints absent from this file: deny or allow.\n", yamlString(auth.Authorization.DefaultPolicy))
	fmt.Fprintf(&body, "  role_claim: %s # JWT claim containing an array of role names.\n\n", yamlString(auth.Authorization.RoleClaim))

	if len(auth.Endpoints) == 0 {
		body.WriteString("endpoints: [] # Generated endpoint access rules; existing rules are preserved.\n")
		return os.WriteFile(path, body.Bytes(), 0o644)
	}
	body.WriteString("endpoints: # Generated endpoint access rules; existing rules are preserved.\n")
	for _, endpoint := range auth.Endpoints {
		fmt.Fprintf(&body, "  - name: %s # Stable generated handler name; informational only.\n", yamlString(endpoint.Name))
		fmt.Fprintf(&body, "    method: %s # HTTP method used together with path to identify the endpoint.\n", yamlString(endpoint.Method))
		fmt.Fprintf(&body, "    path: %s # Full application route, including the configured base path.\n", yamlString(endpoint.Path))
		fmt.Fprintf(&body, "    public: %t # true allows requests without authentication.\n", endpoint.Public)
		fmt.Fprintf(&body, "    require_auth: %t # true requires the selected authentication strategy.\n", endpoint.RequireAuth)
		fmt.Fprintf(&body, "    roles: %s # Allowed roles; [] permits any authenticated user.\n", yamlStringList(endpoint.Roles))
	}
	return os.WriteFile(path, body.Bytes(), 0o644)
}

func yamlString(value string) string {
	return strconv.Quote(value)
}

func yamlStringList(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, yamlString(value))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
