package generator

const tokenServiceTemplate = `package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt"

	"{{ .Module }}/internal/app/domain"
)

type TokenConfig struct {
	TTL          time.Duration
	RefreshToken bool
	Secret       string
	Issuer       string
	Audience     string
}

type TokenService struct {
	config TokenConfig
}

func NewTokenService(config TokenConfig) TokenService {
	return TokenService{config: config}
}

type UserClaims struct {
	User domain.{{ .Features.Auth.ClaimsModel }} ` + "`json:\"user\"`" + `
	jwt.StandardClaims
}

func (s TokenService) GenerateToken(user domain.{{ .Features.Auth.UserModel }}) (string, string, error) {
	now := time.Now()
	payload := UserClaims{
		User: user,
		StandardClaims: jwt.StandardClaims{
			IssuedAt:  now.Unix(),
			ExpiresAt: now.Add(s.config.TTL).Unix(),
			Issuer:    s.config.Issuer,
			Audience:  s.config.Audience,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, payload)
	signed, err := token.SignedString([]byte(s.config.Secret))
	if err != nil {
		return "", "", fmt.Errorf("failed to sign token: %w", err)
	}
	if !s.config.RefreshToken {
		return signed, "", nil
	}
	refreshPayload := UserClaims{
		User: user,
		StandardClaims: jwt.StandardClaims{
			IssuedAt:  now.Unix(),
			ExpiresAt: now.Add(s.config.TTL * 24).Unix(),
			Issuer:    s.config.Issuer,
			Audience:  s.config.Audience,
		},
	}
	refresh, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshPayload).SignedString([]byte(s.config.Secret))
	if err != nil {
		return "", "", fmt.Errorf("failed to sign refresh token: %w", err)
	}
	return signed, refresh, nil
}

func (s TokenService) GetUser(tokenValue string) (domain.{{ .Features.Auth.UserModel }}, error) {
	var userClaims UserClaims
	parsed, err := jwt.ParseWithClaims(tokenValue, &userClaims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.Secret), nil
	})
	if err != nil {
		return domain.{{ .Features.Auth.UserModel }}{}, fmt.Errorf("failed to parse token: %w", err)
	}
	if !parsed.Valid {
		return domain.{{ .Features.Auth.UserModel }}{}, errors.New("invalid token")
	}
	return userClaims.User, nil
}
`

const authHandlersTemplate = `package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"{{ .Module }}/internal/app/common/server"
	"{{ .Module }}/internal/app/domain"
)

type AuthRequest struct {
	Username string ` + "`json:\"username\"`" + `
	Password string ` + "`json:\"password\"`" + `
}

func (r AuthRequest) Validate() error {
	if r.Username == "" {
		return errors.New("username is required")
	}
	if r.Password == "" {
		return errors.New("password is required")
	}
	return nil
}

func (h HttpServer) SignUp(w http.ResponseWriter, r *http.Request) {
	var authRequest AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&authRequest); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}
	if err := authRequest.Validate(); err != nil {
		server.BadRequest("invalid-request", err, w, r)
		return
	}
	hashedPassword, err := hashPassword(authRequest.Password)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	user, err := domain.New{{ .Features.Auth.UserModel }}DB(domain.{{ .Features.Auth.UserModel }}DBData{
		{{ .Features.Auth.UsernameGoName }}: authRequest.Username,
		{{ .Features.Auth.PasswordGoName }}: hashedPassword,
	})
	if err != nil {
		server.BadRequest("invalid-request", err, w, r)
		return
	}
	_, err = h.{{ .Table.Singular }}Service.Create{{ .Features.Auth.UserModel }}(r.Context(), user)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	server.RespondOK(map[string]bool{"ok": true}, w, r)
}

func (h HttpServer) SignIn(w http.ResponseWriter, r *http.Request) {
	var authRequest AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&authRequest); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}
	if err := authRequest.Validate(); err != nil {
		server.BadRequest("invalid-request", err, w, r)
		return
	}
{{- if .Table.Queries.GetAll }}
	users, err := h.{{ .Table.Singular }}Service.GetAll{{ .Table.GoPlural }}(r.Context())
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	var user domain.{{ .Features.Auth.UserModel }}
	found := false
	for _, candidate := range users {
		if stringField(candidate, {{ printf "%q" .Features.Auth.UsernameGoName }}) == authRequest.Username {
			user = candidate
			found = true
			break
		}
	}
	if !found {
		server.BadRequest("invalid-credentials", nil, w, r)
		return
	}
	if !checkPasswordHash(authRequest.Password, stringField(user, {{ printf "%q" .Features.Auth.PasswordGoName }})) {
		server.BadRequest("invalid-credentials", nil, w, r)
		return
	}
	token, refreshToken, err := h.tokenService.GenerateToken(user)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	response := map[string]string{"token": token}
	if refreshToken != "" {
		response["refresh_token"] = refreshToken
	}
	server.RespondOK(response, w, r)
{{- else }}
	server.InternalError("missing-user-lookup", errors.New("auth signin requires a generated GetAll{{ .Table.GoPlural }} query or a configured user lookup endpoint"), w, r)
{{- end }}
}
`

const authMiddlewareTemplate = `package httpserver

import (
	"context"
{{- if eq .Features.Auth.Strategy "basic" }}
	"crypto/subtle"
{{- end }}
	"net/http"
	"reflect"
	"strings"

	"{{ .Module }}/internal/app/common/server"
)

const (
	AuthorizationHeader = "{{ .Features.Auth.JWTHeader }}"
	BearerPrefix        = "{{ .Features.Auth.JWTScheme }} "
)

type ContextKey string

const ContextUserKey ContextKey = "user"

type BasicAuthConfig struct {
	Username string
	Password string
	Realm    string
	Roles    []string
}

func (h HttpServer) CheckRoles(next http.HandlerFunc, allowedRoles ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, roles, ok := h.authenticateRequest(w, r)
		if !ok {
			return
		}
		if !hasAllowedRole(roles, allowedRoles) {
			server.Unauthorised("not-authorized", nil, w, r)
			return
		}
		ctx := context.WithValue(r.Context(), ContextUserKey, user)
		next(w, r.WithContext(ctx))
	}
}

func (h HttpServer) CheckAuthorizedUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := h.authenticateRequest(w, r)
		if !ok {
			return
		}
		ctx := context.WithValue(r.Context(), ContextUserKey, user)
		next(w, r.WithContext(ctx))
	}
}

func (h HttpServer) authenticateRequest(w http.ResponseWriter, r *http.Request) (any, []string, bool) {
{{- if eq .Features.Auth.Strategy "basic" }}
	username, password, ok := r.BasicAuth()
	if !ok || h.basicAuth.Username == "" || h.basicAuth.Password == "" {
		h.basicChallenge(w)
		server.Unauthorised("missing-basic-auth", nil, w, r)
		return nil, nil, false
	}
	usernameOK := subtle.ConstantTimeCompare([]byte(username), []byte(h.basicAuth.Username)) == 1
	passwordOK := subtle.ConstantTimeCompare([]byte(password), []byte(h.basicAuth.Password)) == 1
	if !usernameOK || !passwordOK {
		h.basicChallenge(w)
		server.Unauthorised("invalid-basic-auth", nil, w, r)
		return nil, nil, false
	}
	return username, append([]string(nil), h.basicAuth.Roles...), true
{{- else }}
	token := r.Header.Get(AuthorizationHeader)
	token = strings.TrimSpace(strings.TrimPrefix(token, BearerPrefix))
	user, err := h.tokenService.GetUser(token)
	if err != nil {
		server.InternalError("validate-token", err, w, r)
		return nil, nil, false
	}
	return user, rolesFromUser(user, "{{ .Features.Auth.RolesGoName }}"), true
{{- end }}
}

{{- if eq .Features.Auth.Strategy "basic" }}
func (h HttpServer) basicChallenge(w http.ResponseWriter) {
	if h.basicAuth.Realm == "" {
		h.basicAuth.Realm = "Restricted"
	}
	w.Header().Set("WWW-Authenticate", ` + "`Basic realm=\"`" + `+h.basicAuth.Realm+` + "`\"`" + `)
}
{{- end }}

func hasAllowedRole(actual, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, want := range allowed {
		for _, got := range actual {
			if want == got || want == "*" {
				return true
			}
		}
	}
	return false
}

func rolesFromUser(user any, fieldName string) []string {
	value := reflect.Indirect(reflect.ValueOf(user))
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return nil
	}
	field := value.FieldByName(fieldName)
	if !field.IsValid() {
		return nil
	}
	switch field.Kind() {
	case reflect.Bool:
		if field.Bool() {
			return []string{strings.ToLower(fieldName)}
		}
	case reflect.String:
		raw := field.String()
		if raw == "" {
			return nil
		}
		parts := strings.Split(raw, ",")
		roles := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				roles = append(roles, part)
			}
		}
		return roles
	case reflect.Slice:
		if field.Type().Elem().Kind() != reflect.String {
			return nil
		}
		roles := make([]string, 0, field.Len())
		for i := 0; i < field.Len(); i++ {
			roles = append(roles, field.Index(i).String())
		}
		return roles
	}
	return nil
}

func stringField(value any, fieldName string) string {
	field := reflect.Indirect(reflect.ValueOf(value)).FieldByName(fieldName)
	if !field.IsValid() || field.Kind() != reflect.String {
		return ""
	}
	return field.String()
}
`

const passwordHelpersTemplate = `package httpserver

import "golang.org/x/crypto/bcrypt"

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func checkPasswordHash(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
`
