package appgen

import (
	"strings"

	"github.com/repomz/rest/internal/config"
	"github.com/repomz/rest/internal/generator"
)

func authFeatures(bundle config.Bundle) generator.AuthFeatures {
	if !bundle.Rest.Auth.Bool() || bundle.Auth == nil {
		return generator.AuthFeatures{}
	}
	result := generator.AuthFeatures{
		Enabled:             true,
		Strategy:            strings.ToLower(bundle.Auth.Authentication.Strategy),
		UserModel:           bundle.Auth.Identity.Model,
		UserTable:           bundle.Auth.Identity.Table,
		UserIDField:         bundle.Auth.Identity.IDField,
		UserIDGoName:        exportedName(bundle.Auth.Identity.IDField),
		UsernameField:       bundle.Auth.Identity.UsernameField,
		UsernameGoName:      exportedName(bundle.Auth.Identity.UsernameField),
		PasswordField:       bundle.Auth.Identity.PasswordField,
		PasswordGoName:      exportedName(bundle.Auth.Identity.PasswordField),
		RolesField:          bundle.Auth.Identity.RolesField,
		RolesGoName:         exportedName(bundle.Auth.Identity.RolesField),
		ClaimsModel:         bundle.Auth.Identity.ClaimsModel,
		JWTAlgorithm:        strings.ToUpper(bundle.Auth.Authentication.JWT.Algorithm),
		JWTSecretEnv:        bundle.Auth.Authentication.JWT.SigningKeyEnv,
		JWTPublicKeyFileEnv: bundle.Auth.Authentication.JWT.VerificationKeyFileEnv,
		JWTIssuer:           bundle.Auth.Authentication.JWT.Issuer,
		JWTAudience:         bundle.Auth.Authentication.JWT.Audience,
		JWTAccessTokenTTL:   bundle.Auth.Authentication.JWT.AccessTokenTTL,
		JWTRefreshToken:     bundle.Auth.Authentication.JWT.RefreshToken,
		JWTRefreshStorage:   bundle.Auth.Authentication.JWT.RefreshTokenStorage,
		JWTLeeway:           bundle.Auth.Authentication.JWT.Leeway,
		JWTHeader:           bundle.Auth.Authentication.JWT.HeaderName,
		JWTScheme:           bundle.Auth.Authentication.JWT.TokenPrefix,
		BasicUsernameEnv:    bundle.Auth.Authentication.Basic.UsernameEnv,
		BasicPasswordEnv:    bundle.Auth.Authentication.Basic.PasswordEnv,
		BasicRealm:          bundle.Auth.Authentication.Basic.Realm,
		BasicRoles:          append([]string(nil), bundle.Auth.Authentication.Basic.Roles...),
		RoleClaim:           bundle.Auth.Authorization.RoleClaim,
		DefaultPolicy:       bundle.Auth.Authorization.DefaultPolicy,
		Policies:            map[string]generator.AuthPolicy{},
	}
	for _, endpoint := range bundle.Auth.Endpoints {
		public := endpoint.Public || !endpoint.RequireAuth
		result.Policies[strings.ToUpper(endpoint.Method)+" "+endpoint.Path] = generator.AuthPolicy{
			Public: public,
			Roles:  append([]string(nil), endpoint.Roles...),
		}
	}
	return result
}

func exportedName(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}
